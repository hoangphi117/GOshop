package mail

import "context"

type EmailProviderService interface {
	SendEmail(context context.Context, email *Email) error
}
