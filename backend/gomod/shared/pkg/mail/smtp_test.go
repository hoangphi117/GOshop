// Assert-based check for the SMTP MIME builder, which is the only part of the
// adapter that can be verified without a mail server. Run with:
//
//	go test ./pkg/mail/ -run TestBuildMIME -v
package mail

import (
	"bufio"
	"context"
	"encoding/base64"
	"io"
	"mime"
	"mime/quotedprintable"
	"net/textproto"
	"strings"
	"testing"
)

func testProvider(t *testing.T) *smtpProvider {
	t.Helper()
	svc, err := NewSMTPProvider(&MailConfig{ProviderConfig: map[string]any{
		"smtp": map[string]any{
			"host":             "smtp.gmail.com",
			"port":             "587",
			"username":         "shop@example.com",
			"password":         "app-password",
			"mail_sender":      "shop@example.com",
			"mail_sender_name": "GOshop Admin",
		},
	}})
	if err != nil {
		t.Fatalf("NewSMTPProvider: %v", err)
	}
	p, ok := svc.(*smtpProvider)
	if !ok {
		t.Fatalf("provider is %T, want *smtpProvider", svc)
	}
	return p
}

func splitMessage(t *testing.T, msg []byte) (textproto.MIMEHeader, string) {
	t.Helper()
	full := string(msg)
	sep := strings.Index(full, "\r\n\r\n")
	if sep < 0 {
		t.Fatalf("message has no header/body separator:\n%q", full)
	}
	h, err := textproto.NewReader(bufio.NewReader(strings.NewReader(full[:sep] + "\r\n\r\n"))).ReadMIMEHeader()
	if err != nil {
		t.Fatalf("parse headers: %v", err)
	}
	// The body keeps its real CRLFs: the base64 decoder trims those itself.
	return h, full[sep+4:]
}

func TestBuildMIME(t *testing.T) {
	t.Run("vietnamese subject is q-encoded, not raw", func(t *testing.T) {
		p := testProvider(t)
		msg, err := p.buildMIME(&Email{
			To:      []Address{{Email: "user@example.com", Name: "Nguyễn Văn A"}},
			Subject: "Đặt lại mật khẩu",
			Text:    "body",
		})
		if err != nil {
			t.Fatalf("buildMIME: %v", err)
		}
		h, _ := splitMessage(t, msg)

		subject, err := new(mime.WordDecoder).DecodeHeader(h.Get("Subject"))
		if err != nil {
			t.Fatalf("decode subject: %v", err)
		}
		if subject != "Đặt lại mật khẩu" {
			t.Errorf("decoded subject = %q, want %q", subject, "Đặt lại mật khẩu")
		}
		if strings.Contains(h.Get("Subject"), "Đ") {
			t.Errorf("subject header still contains a raw non-ASCII rune: %q", h.Get("Subject"))
		}

		if !strings.Contains(h.Get("To"), "=?utf-8?q?") {
			t.Errorf("recipient name is not encoded: %q", h.Get("To"))
		}
	})

	t.Run("multipart/alternative puts text before html", func(t *testing.T) {
		p := testProvider(t)
		msg, err := p.buildMIME(&Email{
			To:      []Address{{Email: "user@example.com"}},
			Subject: "Order confirmed",
			Text:    "plain version",
			HTML:    "<p>html version</p>",
		})
		if err != nil {
			t.Fatalf("buildMIME: %v", err)
		}
		h, body := splitMessage(t, msg)
		if ct := h.Get("Content-Type"); !strings.HasPrefix(ct, "multipart/alternative;") {
			t.Fatalf("Content-Type = %q, want multipart/alternative", ct)
		}

		_, params, err := mime.ParseMediaType(h.Get("Content-Type"))
		if err != nil {
			t.Fatalf("parse media type: %v", err)
		}
		boundary := params["boundary"]
		if boundary == "" {
			t.Fatal("no boundary in Content-Type")
		}

		parts := strings.Split(body, "--"+boundary)
		if len(parts) < 4 {
			t.Fatalf("expected 2 parts + closing boundary, got %d sections", len(parts))
		}

		textIdx, htmlIdx := -1, -1
		for i, part := range parts {
			switch {
			case strings.Contains(part, "text/plain"):
				textIdx = i
			case strings.Contains(part, "text/html"):
				htmlIdx = i
			}
		}
		if textIdx < 0 || htmlIdx < 0 {
			t.Fatalf("missing a part: text=%d html=%d", textIdx, htmlIdx)
		}
		if textIdx > htmlIdx {
			t.Errorf("text part at %d comes after html at %d", textIdx, htmlIdx)
		}

		var plain strings.Builder
		plain.WriteString(parts[textIdx])
		if !strings.Contains(decodePart(t, plain.String()), "plain version") {
			t.Error("text part does not round-trip its body")
		}
	})

	t.Run("text only produces a single part", func(t *testing.T) {
		p := testProvider(t)
		msg, err := p.buildMIME(&Email{
			To:      []Address{{Email: "user@example.com"}},
			Subject: "hi",
			Text:    "only text",
		})
		if err != nil {
			t.Fatalf("buildMIME: %v", err)
		}
		h, body := splitMessage(t, msg)
		if ct := h.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain;") {
			t.Fatalf("Content-Type = %q, want text/plain", ct)
		}
		if strings.Contains(body, "multipart") {
			t.Error("single-part message mentions multipart")
		}
		if decodeBody(body) != "only text" {
			t.Errorf("body = %q, want %q", decodeBody(body), "only text")
		}
	})

	t.Run("category becomes a header", func(t *testing.T) {
		p := testProvider(t)
		msg, err := p.buildMIME(&Email{
			To:       []Address{{Email: "user@example.com"}},
			Subject:  "otp",
			Text:     "123456",
			Category: "register-otp",
		})
		if err != nil {
			t.Fatalf("buildMIME: %v", err)
		}
		h, _ := splitMessage(t, msg)
		if got := h.Get("X-Goshop-Category"); got != "register-otp" {
			t.Errorf("X-Goshop-Category = %q, want register-otp", got)
		}
		if got := h.Get("Message-ID"); !strings.HasPrefix(got, "<") || !strings.Contains(got, "@smtp.gmail.com") {
			t.Errorf("Message-ID = %q, want <uuid@smtp.gmail.com>", got)
		}
	})

	t.Run("no body at all is an error", func(t *testing.T) {
		p := testProvider(t)
		if _, err := p.buildMIME(&Email{To: []Address{{Email: "user@example.com"}}, Subject: "empty"}); err == nil {
			t.Fatal("buildMIME accepted an email with neither text nor html")
		}
	})
}

func TestSendEmailValidation(t *testing.T) {
	p := testProvider(t)

	if err := p.SendEmail(context.Background(), &Email{Subject: "x", Text: "y"}); err == nil {
		t.Fatal("SendEmail accepted an email with no recipients")
	}
	if err := p.SendEmail(context.Background(), &Email{
		To:      []Address{{Email: ""}},
		Subject: "x",
		Text:    "y",
	}); err == nil {
		t.Fatal("SendEmail accepted a blank recipient address")
	}
}

func TestNewSMTPProviderValidation(t *testing.T) {
	if _, err := NewSMTPProvider(&MailConfig{ProviderConfig: map[string]any{}}); err == nil {
		t.Fatal("NewSMTPProvider accepted a config without the smtp key")
	}
	if _, err := NewSMTPProvider(&MailConfig{ProviderConfig: map[string]any{
		"smtp": map[string]any{"host": "smtp.gmail.com", "port": "587", "username": "u"},
	}}); err == nil {
		t.Fatal("NewSMTPProvider accepted a config without a password or sender")
	}
}

// decodePart pulls the body out of a MIME part and base64-decodes it.
func decodePart(t *testing.T, part string) string {
	t.Helper()
	part = strings.TrimLeft(part, "\r\n")
	if i := strings.Index(part, "\r\n\r\n"); i >= 0 {
		return decodeBody(part[i+4:])
	}
	return ""
}

func decodeBody(body string) string {
	body = strings.TrimSpace(strings.ReplaceAll(body, "\r\n", ""))
	if decoded, err := base64.StdEncoding.DecodeString(body); err == nil {
		return string(decoded)
	}
	if decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(body))); err == nil {
		return string(decoded)
	}
	return body
}
