package config

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	domainwebhook "github.com/goformx/goforms/internal/domain/webhook"
)

const maxWebhookKeyringJSONBytes = 8192

type WebhookConfig struct {
	Enabled               bool          `json:"enabled"`
	EncryptionKey         string        `json:"-"`
	EncryptionKeyring     string        `json:"-"`
	ActiveEncryptionKeyID string        `json:"active_encryption_key_id"`
	PollInterval          time.Duration `json:"poll_interval"`
	RequestTimeout        time.Duration `json:"request_timeout"`
	LockTimeout           time.Duration `json:"lock_timeout"`
	MaxAttempts           int           `json:"max_attempts"`
	BackoffBase           time.Duration `json:"backoff_base"`
	BackoffMax            time.Duration `json:"backoff_max"`
}

func (c WebhookConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if _, err := c.Cipher(); err != nil {
		return err
	}
	if c.PollInterval <= 0 || c.RequestTimeout <= 0 || c.LockTimeout <= 0 ||
		c.BackoffBase <= 0 || c.BackoffMax <= 0 {
		return errors.New("webhook durations must be positive")
	}
	if c.LockTimeout <= c.RequestTimeout {
		return errors.New("webhook lock timeout must exceed request timeout")
	}
	if c.MaxAttempts < 1 {
		return errors.New("webhook max attempts must be positive")
	}
	if c.BackoffMax < c.BackoffBase {
		return errors.New("webhook maximum backoff must not be shorter than base backoff")
	}
	return nil
}

// Cipher is shared by the API and maintenance command. No parser error includes
// key material; legacy-only configuration remains supported until migrated.
func (c WebhookConfig) Cipher() (*domainwebhook.Cipher, error) {
	if c.EncryptionKeyring == "" && c.ActiveEncryptionKeyID == "" {
		return domainwebhook.NewCipher(c.EncryptionKey)
	}
	invalid := errors.New("webhook encryption keyring must be a bounded JSON object with unique key IDs and string keys")
	if len(c.EncryptionKeyring) > maxWebhookKeyringJSONBytes {
		return nil, invalid
	}
	decoder := json.NewDecoder(strings.NewReader(c.EncryptionKeyring))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, invalid
	}
	keys := make(map[string]string)
	for decoder.More() {
		token, tokenErr := decoder.Token()
		id, ok := token.(string)
		if tokenErr != nil || !ok || len(keys) >= domainwebhook.MaxEncryptionKeys {
			return nil, invalid
		}
		if _, duplicate := keys[id]; duplicate {
			return nil, invalid
		}
		var key string
		if err := decoder.Decode(&key); err != nil || key == "" {
			return nil, invalid
		}
		keys[id] = key
	}
	if end, err := decoder.Token(); err != nil || end != json.Delim('}') {
		return nil, invalid
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, invalid
	}
	return domainwebhook.NewKeyring(c.ActiveEncryptionKeyID, keys, c.EncryptionKey)
}
