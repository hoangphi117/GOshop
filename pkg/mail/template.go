package mail

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"time"
)

const resetPasswordHTMLTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<meta name="x-apple-disable-message-reformatting">
	<title>Reset Your Password</title>
	<style>
		body, table, td, a { -webkit-text-size-adjust: 100%; -ms-text-size-adjust: 100%; }
		table, td { mso-table-lspace: 0pt; mso-table-rspace: 0pt; }
		img { -ms-interpolation-mode: bicubic; border: 0; height: auto; line-height: 100%; outline: none; text-decoration: none; }
		body { margin: 0; padding: 0; background-color: #f4f7f6; font-family: 'Helvetica Neue', Helvetica, Arial, sans-serif; }
		.container { max-width: 600px; margin: 40px auto; background-color: #ffffff; border-radius: 8px; overflow: hidden; box-shadow: 0 4px 12px rgba(0,0,0,0.05); }
		.header { background-color: #00ADD8; padding: 30px 20px; text-align: center; }
		.header h1 { margin: 0; font-size: 24px; font-weight: 600; letter-spacing: 1px; color: #ffffff; }
		.content { padding: 40px 30px; color: #333333; line-height: 1.6; font-size: 16px; }
		.content p { margin: 0 0 15px 0; }
		.btn-container { text-align: center; margin: 35px 0; }
		.btn { display: inline-block; background-color: #00ADD8; color: #ffffff !important; text-decoration: none; padding: 14px 32px; border-radius: 6px; font-weight: bold; font-size: 16px; border: 1px solid #00ADD8; }
		.expiry-note { font-size: 14px; color: #888888; margin-top: 10px; }
		.link-fallback { margin-top: 30px; font-size: 14px; color: #555555; word-break: break-all; background: #f4f7f6; padding: 15px; border-radius: 4px; border-left: 4px solid #00ADD8; }
		.footer { background-color: #f9fbfb; padding: 20px; text-align: center; font-size: 13px; color: #888888; border-top: 1px solid #eeeeee; }
	</style>
</head>
<body style="background-color: #f4f7f6; margin: 0; padding: 20px 0;">
	<table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="background-color: #f4f7f6;">
		<tr>
			<td align="center">
				<table role="presentation" class="container" border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 600px; background-color: #ffffff; border-radius: 8px; overflow: hidden; box-shadow: 0 4px 12px rgba(0,0,0,0.05);">
					<tr>
						<td class="header" style="background-color: #00ADD8; padding: 30px 20px; text-align: center;">
							<h1 style="margin: 0; font-size: 24px; font-weight: 600; color: #ffffff;">Password Reset</h1>
						</td>
					</tr>
					<tr>
						<td class="content" style="padding: 40px 30px; color: #333333; font-size: 16px; line-height: 1.6;">
							<p>Hello <strong>{{.UserName}}</strong>,</p>
							<p>We received a request to reset the password for the account associated with this email address. If this was you, please click the button below to set a new password.</p>

							<table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%">
								<tr>
									<td align="center" style="padding: 30px 0;">
										<a href="{{.ResetLink}}" class="btn" style="display: inline-block; background-color: #00ADD8; color: #ffffff; text-decoration: none; padding: 14px 32px; border-radius: 6px; font-weight: bold; font-size: 16px;">Reset Password</a>
										<p class="expiry-note" style="font-size: 14px; color: #888888; margin-top: 10px;">This link will expire in {{.ExpiresIn}} for your security.</p>
									</td>
								</tr>
							</table>

							<p>If you did not make this request, you can safely ignore this email. Your account remains secure and your current password has not been changed. If you believe someone else may have access to your account, please reset your password immediately or contact our support team.</p>

							<div class="link-fallback" style="margin-top: 30px; font-size: 14px; background: #f4f7f6; padding: 15px; border-radius: 4px; border-left: 4px solid #00ADD8;">
								<p style="margin: 0 0 5px 0; color: #555555;">Or copy and paste this link into your browser:</p>
								<a href="{{.ResetLink}}" style="color: #00ADD8; word-break: break-all;">{{.ResetLink}}</a>
							</div>
						</td>
					</tr>
					<tr>
						<td class="footer" style="background-color: #f9fbfb; padding: 20px; text-align: center; font-size: 13px; color: #888888; border-top: 1px solid #eeeeee;">
							<p style="margin: 0 0 5px 0;">This is an automated message, please do not reply.</p>
							<p style="margin: 0;">&copy; {{.Year}} GOshop. All rights reserved.</p>
						</td>
					</tr>
				</table>
			</td>
		</tr>
	</table>
</body>
</html>`

const resetPasswordTextTmpl = `Hello {{.UserName}},

We received a request to reset the password for the account associated with this email address. If this was you, click the link below to set a new password:

{{.ResetLink}}

This link will expire in {{.ExpiresIn}} for your security.

If you did not make this request, you can safely ignore this email. Your account remains secure and your current password has not been changed.

This is an automated message, please do not reply.
© {{.Year}} GOshop. All rights reserved.`

type resetPasswordData struct {
	UserName  string
	ResetLink string
	ExpiresIn string
	Year      int
}

var (
	resetPasswordHTMLParsed = template.Must(template.New("resetPasswordHTML").Parse(resetPasswordHTMLTmpl))
	resetPasswordTextParsed = template.Must(template.New("resetPasswordText").Parse(resetPasswordTextTmpl))
)

func ResetPasswordTemplate(userName, resetLink string, expiresIn time.Duration, year int) (htmlBody, textBody string, err error) {
	data := resetPasswordData{
		UserName:  userName,
		ResetLink: resetLink,
		ExpiresIn: formatDuration(expiresIn),
		Year:      year,
	}

	var htmlBuf bytes.Buffer
	if err := resetPasswordHTMLParsed.Execute(&htmlBuf, data); err != nil {
		return "", "", fmt.Errorf("failed to render reset password html template: %w", err)
	}

	var textBuf bytes.Buffer
	if err := resetPasswordTextParsed.Execute(&textBuf, data); err != nil {
		return "", "", fmt.Errorf("failed to render reset password text template: %w", err)
	}

	return htmlBuf.String(), textBuf.String(), nil
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)

	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	var parts []string
	if h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
	}
	if m > 0 {
		parts = append(parts, fmt.Sprintf("%dm", m))
	}
	if s > 0 && h == 0 && m == 0 {
		parts = append(parts, fmt.Sprintf("%ds", s))
	}

	if len(parts) == 0 {
		return "0s"
	}

	return strings.Join(parts, " ")
}
