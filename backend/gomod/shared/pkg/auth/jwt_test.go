package auth

import "testing"

func TestLoadOrGenerateRSAKeys_DisablesSigning(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("JWT_SIGNING_ENABLED", "false")
	t.Setenv("JWT_PRIVATE_KEY", "")
	t.Setenv("JWT_PUBLIC_KEY", "")

	privateKey, publicKey, _ := loadOrGenerateRSAKeys()
	if privateKey != nil {
		t.Fatal("expected private key to remain unloaded when signing is disabled")
	}
	if publicKey != nil {
		t.Fatal("expected no public key when no verifier key is configured")
	}
}

func TestLoadOrGenerateRSAKeys_GeneratesDevelopmentSignerKey(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("JWT_SIGNING_ENABLED", "true")
	t.Setenv("JWT_PRIVATE_KEY", "")
	t.Setenv("JWT_PUBLIC_KEY", "")

	privateKey, publicKey, _ := loadOrGenerateRSAKeys()
	if privateKey == nil {
		t.Fatal("expected development signer to generate a private key")
	}
	if publicKey == nil {
		t.Fatal("expected development signer to generate a public key")
	}
}
