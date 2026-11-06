package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"

	"goshop/shared/pkg/utils"
)

// Phase 6: broker transport security.
//
// Transport security and authentication are configured entirely through
// environment variables, so the same binary runs against a plaintext dev broker
// and a SASL_SSL prod one:
//
//	KAFKA_SASL_USER / _PASSWORD / _MECHANISM   authentication (SCRAM-SHA-512)
//	KAFKA_TLS_ENABLED                          "true" turns TLS on
//	KAFKA_TLS_CA_FILE                          path to the CA bundle that signs the broker cert
//	KAFKA_TLS_SERVER_NAME                      override the SNI name (default: the dialled host)
//	KAFKA_TLS_SKIP_VERIFY                      "true" disables verification (dev only)
//
// Nothing set -> PLAINTEXT, the pre-phase-6 behaviour, so `make dev-host`
// keeps working without a broker change.
//
// ponytail: certificates are read once at startup. Rotation needs a restart;
// add a reloader when certs stop being long-lived.

const (
	// SASL mechanisms the dialer accepts. Values match Kafka's own names so an
	// operator can copy the broker config verbatim.
	saslSCRAMSHA256 = "SCRAM-SHA-256"
	saslSCRAMSHA512 = "SCRAM-SHA-512"
	saslPlain       = "PLAIN"

	defaultSASLMechanism = saslSCRAMSHA512
)

// authConfig is what the environment says about connecting to the broker.
// Zero value means "plaintext, no auth".
type authConfig struct {
	user      string
	password  string
	mechanism string

	tlsEnabled    bool
	tlsCACert     string // path to a PEM bundle, empty = system roots
	tlsServerName string
	tlsSkipVerify bool
}

// loadAuthConfig reads the security environment. Missing values are not an
// error: a plaintext broker is a supported configuration.
func loadAuthConfig() authConfig {
	return authConfig{
		user:      strings.TrimSpace(utils.GetEnv("KAFKA_SASL_USER", "")),
		password:  utils.GetEnv("KAFKA_SASL_PASSWORD", ""),
		mechanism: strings.ToUpper(strings.TrimSpace(utils.GetEnv("KAFKA_SASL_MECHANISM", defaultSASLMechanism))),

		tlsEnabled:    isTrue(utils.GetEnv("KAFKA_TLS_ENABLED", "")),
		tlsCACert:     strings.TrimSpace(utils.GetEnv("KAFKA_TLS_CA_FILE", "")),
		tlsServerName: strings.TrimSpace(utils.GetEnv("KAFKA_TLS_SERVER_NAME", "")),
		tlsSkipVerify: isTrue(utils.GetEnv("KAFKA_TLS_SKIP_VERIFY", "")),
	}
}

// dialer returns the kafka.Dialer every connection must use, or nil when the
// broker needs neither TLS nor SASL. A non-nil dialer carries both settings.
func (c authConfig) dialer() (*kafka.Dialer, error) {
	// Validate before deciding whether to build anything: a half-configured
	// deployment must fail at startup, not on the first message.
	if c.user == "" && c.password != "" {
		return nil, errors.New("kafka: KAFKA_SASL_PASSWORD is set but KAFKA_SASL_USER is empty")
	}

	var mechanism sasl.Mechanism
	if c.user != "" {
		m, err := c.mechanismFor()
		if err != nil {
			return nil, err
		}
		mechanism = m
	}

	tlsCfg, err := c.tlsConfig()
	if err != nil {
		return nil, err
	}

	if mechanism == nil && tlsCfg == nil {
		return nil, nil // plaintext
	}

	return &kafka.Dialer{Timeout: dialTimeout, DualStack: true, SASLMechanism: mechanism, TLS: tlsCfg}, nil
}

// tlsConfig builds the client TLS config, or nil when TLS is off.
func (c authConfig) tlsConfig() (*tls.Config, error) {
	if !c.tlsEnabled {
		// A CA file without KAFKA_TLS_ENABLED is a half-configured deployment:
		// the operator meant to encrypt and would silently send plaintext.
		if c.tlsCACert != "" {
			return nil, errors.New("kafka: KAFKA_TLS_CA_FILE is set but KAFKA_TLS_ENABLED is not; set it to true")
		}
		if c.tlsSkipVerify {
			return nil, errors.New("kafka: KAFKA_TLS_SKIP_VERIFY is set but KAFKA_TLS_ENABLED is not; set it to true")
		}
		return nil, nil
	}

	cfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         c.tlsServerName,
		InsecureSkipVerify: c.tlsSkipVerify, //nolint:gosec // explicit dev-only switch
	}
	if c.tlsCACert != "" {
		pool, err := rootPool(c.tlsCACert)
		if err != nil {
			return nil, fmt.Errorf("kafka: load TLS CA: %w", err)
		}
		cfg.RootCAs = pool
	}
	return cfg, nil
}

// mechanismFor maps the mechanism name to a kafka-go SASL implementation.
func (c authConfig) mechanismFor() (sasl.Mechanism, error) {
	switch c.mechanism {
	case saslSCRAMSHA256:
		return scram.Mechanism(scram.SHA256, c.user, c.password)
	case saslSCRAMSHA512, "":
		return scram.Mechanism(scram.SHA512, c.user, c.password)
	case saslPlain:
		// PLAIN sends the password in clear text; it exists for brokers that
		// cannot do SCRAM, and is refused without a verified TLS channel.
		if !c.tlsEnabled || c.tlsSkipVerify {
			return nil, errors.New("kafka: SASL PLAIN requires KAFKA_TLS_ENABLED with verification, refusing to send credentials in clear text")
		}
		return plain.Mechanism{Username: c.user, Password: c.password}, nil
	default:
		return nil, fmt.Errorf("kafka: unsupported KAFKA_SASL_MECHANISM %q (want %s, %s or %s)",
			c.mechanism, saslSCRAMSHA512, saslSCRAMSHA256, saslPlain)
	}
}

// rootPool loads a PEM bundle into a certificate pool. The variable name
// carries _FILE for a reason: a multi-line PEM cannot live in an env var on
// every platform, so the value is a path readable by the process.
func rootPool(pemPath string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(pemPath)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("no certificates found in %s", pemPath)
	}
	return pool, nil
}

// isTrue accepts the spellings env files and Helm charts actually use.
func isTrue(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
