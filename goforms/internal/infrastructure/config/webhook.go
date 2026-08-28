package config

import (
	"errors"
	"time"

	domainwebhook "github.com/goformx/goforms/internal/domain/webhook"
)

type WebhookConfig struct {
	Enabled        bool          `json:"enabled"`
	EncryptionKey  string        `json:"-"`
	PollInterval   time.Duration `json:"poll_interval"`
	RequestTimeout time.Duration `json:"request_timeout"`
	LockTimeout    time.Duration `json:"lock_timeout"`
	MaxAttempts    int           `json:"max_attempts"`
	BackoffBase    time.Duration `json:"backoff_base"`
	BackoffMax     time.Duration `json:"backoff_max"`
}

func (c WebhookConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if _, err := domainwebhook.NewCipher(c.EncryptionKey); err != nil {
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
