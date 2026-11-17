package mail

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"mime/multipart"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"goshop/shared/pkg/utils"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// smtpProvider sends mail over SMTP with STARTTLS (Gmail on :587). It keeps the
// same contract as the other providers: it returns the message unchanged for
// MailService to retry, and turns transport failures into ErrCodeInternal.
//
// ponytail: the stdlib MIME builder covers what the templates emit — one text
// part and one html part. No attachments, no CC/BCC, no DKIM signing. Reach for
// github.com/wneessen/go-mail the day any of those is actually needed.
type SMTPConfig struct {
	Host           string
	Port           string
	Username       string
	Password       string
	MailSender     string
	MailSenderName string
}

type smtpProvider struct {
	config *SMTPConfig
	logger *zerolog.Logger
}

func NewSMTPProvider(config *MailConfig) (EmailProviderService, error) {
	raw, ok := config.ProviderConfig["smtp"].(map[string]any)
	if !ok {
		return nil, utils.NewError(utils.ErrCodeInternal, "Invalid smtp configuration")
	}

	str := func(key string) string {
		v, _ := raw[key].(string)
		return strings.TrimSpace(v)
	}

	cfg := &SMTPConfig{
		Host:           str("host"),
		Port:           str("port"),
		Username:       str("username"),
		Password:       str("password"),
		MailSender:     str("mail_sender"),
		MailSenderName: str("mail_sender_name"),
	}
	if cfg.Host == "" || cfg.Port == "" || cfg.Username == "" || cfg.Password == "" || cfg.MailSender == "" {
		return nil, utils.NewError(utils.ErrCodeInternal, "Missing required smtp configuration")
	}

	// Gmail rejects a From that is not the authenticated mailbox with a 550,
	// which is far harder to read than this warning at boot.
	if config.Logger != nil && !strings.EqualFold(cfg.MailSender, cfg.Username) {
		config.Logger.Warn().
			Str("mail_sender", cfg.MailSender).
			Str("smtp_username", cfg.Username).
			Msg("MAIL_SENDER differs from SMTP_USERNAME; Gmail will reject the message")
	}

	return &smtpProvider{config: cfg, logger: config.Logger}, nil
}

func (p *smtpProvider) SendEmail(ctx context.Context, email *Email) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(email.To) == 0 {
		return utils.NewError(utils.ErrCodeBadRequest, "email recipient is required")
	}

	toAddrs := make([]string, 0, len(email.To))
	for _, addr := range email.To {
		if addr.Email == "" {
			return utils.NewError(utils.ErrCodeBadRequest, "email recipient address is required")
		}
		toAddrs = append(toAddrs, addr.Email)
	}

	msg, err := p.buildMIME(email)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to build email", err)
	}

	// PlainAuth refuses to hand over credentials unless the host matches the
	// one it talks to, and net/smtp only upgrades to TLS when the server offers
	// STARTTLS — which is exactly Gmail on :587.
	auth := smtp.PlainAuth("", p.config.Username, p.config.Password, p.config.Host)
	addr := net.JoinHostPort(p.config.Host, p.config.Port)

	if err := smtp.SendMail(addr, auth, p.config.MailSender, toAddrs, msg); err != nil {
		if p.logger != nil {
			p.logger.Error().Err(err).Str("to", toAddrs[0]).Msg("failed to send email via smtp")
		}
		return utils.WrapError(utils.ErrCodeInternal, "Failed to send email", err)
	}

	return nil
}

// buildMIME renders the RFC 5322 message. Kept free of transport concerns so
// it can be asserted on without a mail server.
func (p *smtpProvider) buildMIME(email *Email) ([]byte, error) {
	var buf bytes.Buffer

	fromName := p.config.MailSenderName
	if fromName == "" {
		fromName = p.config.MailSender
	}
	// Q-encode so a Vietnamese subject or display name survives every client.
	buf.WriteString("From: " + mime.QEncoding.Encode("utf-8", fromName) + " <" + p.config.MailSender + ">\r\n")

	to := make([]string, 0, len(email.To))
	for _, addr := range email.To {
		if addr.Name == "" {
			to = append(to, addr.Email)
			continue
		}
		to = append(to, mime.QEncoding.Encode("utf-8", addr.Name)+" <"+addr.Email+">")
	}
	buf.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	buf.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", email.Subject) + "\r\n")
	buf.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	buf.WriteString("MIME-Version: 1.0\r\n")

	if email.Category != "" {
		// Mailtrap had a first-class Category field; a header keeps the same
		// filtering in the log pipeline without a vendor API.
		buf.WriteString("X-Goshop-Category: " + email.Category + "\r\n")
	}

	host := p.config.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	buf.WriteString("Message-ID: <" + uuid.NewString() + "@" + host + ">\r\n")

	parts := make([]struct{ contentType, body string }, 0, 2)
	if email.Text != "" {
		parts = append(parts, struct{ contentType, body string }{"text/plain; charset=\"utf-8\"", email.Text})
	}
	if email.HTML != "" {
		parts = append(parts, struct{ contentType, body string }{"text/html; charset=\"utf-8\"", email.HTML})
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("email has neither a text nor an html body")
	}

	if len(parts) == 1 {
		buf.WriteString("Content-Type: " + parts[0].contentType + "\r\n")
		buf.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
		writeBase64(&buf, parts[0].body)
		return buf.Bytes(), nil
	}

	mw := multipart.NewWriter(&buf)
	buf.WriteString("Content-Type: multipart/alternative; boundary=\"" + mw.Boundary() + "\"\r\n\r\n")
	for _, part := range parts {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Type", part.contentType)
		h.Set("Content-Transfer-Encoding", "base64")
		w, err := mw.CreatePart(h)
		if err != nil {
			return nil, fmt.Errorf("create %s part: %w", part.contentType, err)
		}
		// CreatePart writes its own CRLF; the encoded body must not add another
		// or the decoder sees a blank line before the payload.
		writeBase64NoTrailing(w, part.body)
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("close multipart body: %w", err)
	}

	return buf.Bytes(), nil
}

// writeBase64 emits MIME line-wrapped (76 char) base64 and a trailing CRLF.
func writeBase64(w interface{ Write([]byte) (int, error) }, body string) {
	encoded := base64.StdEncoding.EncodeToString([]byte(body))
	for len(encoded) > 76 {
		_, _ = w.Write([]byte(encoded[:76] + "\r\n"))
		encoded = encoded[76:]
	}
	if encoded != "" {
		_, _ = w.Write([]byte(encoded + "\r\n"))
	}
}

// writeBase64NoTrailing is the same without the final CRLF, for multipart
// parts where the writer already terminates the line.
func writeBase64NoTrailing(w interface{ Write([]byte) (int, error) }, body string) {
	encoded := base64.StdEncoding.EncodeToString([]byte(body))
	for len(encoded) > 76 {
		_, _ = w.Write([]byte(encoded[:76] + "\r\n"))
		encoded = encoded[76:]
	}
	if encoded != "" {
		_, _ = w.Write([]byte(encoded))
	}
}
