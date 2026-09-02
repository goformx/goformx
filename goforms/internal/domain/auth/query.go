package auth

import (
	"errors"
	"time"
)

const (
	DefaultTokenPageLimit = 25
	MaxTokenPageLimit     = 100
)

// TokenListOptions is an organization-independent keyset page boundary.
type TokenListOptions struct {
	Limit    int
	Before   time.Time
	BeforeID string
}

func (o TokenListOptions) Validate() error {
	if o.Limit < 1 || o.Limit > MaxTokenPageLimit {
		return errors.New("token page limit is invalid")
	}
	if o.Before.IsZero() != (o.BeforeID == "") {
		return errors.New("token cursor requires both time and identifier")
	}
	return nil
}
