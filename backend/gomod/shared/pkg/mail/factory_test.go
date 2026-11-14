// Assert-based check for the provider factory: MAIL_PROVIDER_TYPE must select a
// real provider, an empty value must fall back to smtp, and an unknown value
// must fail loudly instead of silently disabling mail. Run with:
//
//	go test ./pkg/mail/ -run TestNewProviderFactory -v
package mail

import "testing"

func TestNewProviderFactory(t *testing.T) {
	cases := []struct {
		raw     string
		wantErr bool
		check   func(t *testing.T, p EmailProviderService)
	}{
		{
			raw: "smtp",
			check: func(t *testing.T, p EmailProviderService) {
				t.Helper()
				if _, ok := p.(*smtpProvider); !ok {
					t.Fatalf("provider is %T, want *smtpProvider", p)
				}
			},
		},
		{
			raw: "",
			check: func(t *testing.T, p EmailProviderService) {
				t.Helper()
				if _, ok := p.(*smtpProvider); !ok {
					t.Fatalf("empty value resolved to %T, want *smtpProvider", p)
				}
			},
		},
		{raw: "mailtrap", wantErr: true},
		{raw: "sendgrid", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			factory, err := NewProviderFactory(ProviderTypeFromConfig(tc.raw))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("value %q was accepted", tc.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewProviderFactory(%q): %v", tc.raw, err)
			}

			provider, err := factory.CreateProvider(&MailConfig{ProviderConfig: testProviderConfig()})
			if err != nil {
				t.Fatalf("CreateProvider(%q): %v", tc.raw, err)
			}
			tc.check(t, provider)
		})
	}
}

// testProviderConfig returns a map that satisfies the smtp provider.
func testProviderConfig() map[string]any {
	return map[string]any{
		"smtp": map[string]any{
			"host":             "smtp.gmail.com",
			"port":             "587",
			"username":         "shop@example.com",
			"password":         "app-password",
			"mail_sender":      "shop@example.com",
			"mail_sender_name": "GOshop Admin",
		},
	}
}
