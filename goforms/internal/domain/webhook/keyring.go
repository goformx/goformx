package webhook

import (
	"crypto/cipher"
	"errors"
)

const (
	// The versioned header and key ID are authenticated, not secret. Once an
	// envelope is recognized, authentication failure never tries another key.
	envelopePrefix          = "goformx.webhook.v1:"
	MaxEncryptionKeys       = 8
	MaxEncryptionKeyIDBytes = 64
)

var errCipherAuthentication = errors.New("encrypted webhook configuration failed authentication")

// ActiveKeyID returns an empty ID for legacy-only or disabled construction.
func (c *Cipher) ActiveKeyID() string {
	if c == nil {
		return ""
	}
	return c.activeID
}

// NewKeyring writes only with activeID and reads any explicitly configured key.
// legacyKey permits reading the original untagged format during migration;
// it never becomes an implicit writer or a fallback for a tagged ciphertext.
func NewKeyring(activeID string, encodedKeys map[string]string, legacyKey string) (*Cipher, error) {
	if !validEncryptionKeyID(activeID) || len(encodedKeys) == 0 || len(encodedKeys) > MaxEncryptionKeys {
		return nil, errors.New("webhook keyring requires a valid active key ID and 1 to 8 keys")
	}
	result := &Cipher{activeID: activeID, keys: make(map[string]cipher.AEAD, len(encodedKeys))}
	for id, key := range encodedKeys {
		if !validEncryptionKeyID(id) {
			return nil, errors.New("webhook encryption key IDs require 1 to 64 ASCII letters, digits, underscores or hyphens")
		}
		instance, err := NewCipher(key)
		if err != nil {
			return nil, errors.New("webhook keyring contains an invalid encryption key")
		}
		result.keys[id] = instance.aead
	}
	result.aead = result.keys[activeID]
	if result.aead == nil {
		return nil, errors.New("webhook active encryption key is absent from keyring")
	}
	if legacyKey != "" {
		legacy, err := NewCipher(legacyKey)
		if err != nil {
			return nil, errors.New("webhook legacy encryption key is invalid")
		}
		result.legacy = legacy.aead
	}
	return result, nil
}

func validEncryptionKeyID(id string) bool {
	if len(id) == 0 || len(id) > MaxEncryptionKeyIDBytes {
		return false
	}
	for _, char := range id {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') && char != '_' && char != '-' {
			return false
		}
	}
	return true
}

func envelopeAAD(header []byte, formID string) []byte {
	aad := append([]byte(nil), header...)
	aad = append(aad, 0)
	return append(aad, formID...)
}

// Reencrypt authenticates even already-current rows. It preserves the exact
// plaintext bytes, including fields a future SecretConfig may add.
func (c *Cipher) Reencrypt(ciphertext []byte, formID string) ([]byte, bool, error) {
	if c == nil || c.activeID == "" {
		return nil, false, errors.New("webhook rotation requires an explicit active keyring")
	}
	plaintext, id, err := c.decryptBytes(ciphertext, formID)
	if err != nil {
		return nil, false, err
	}
	if id == c.activeID {
		return ciphertext, false, nil
	}
	updated, err := c.encryptBytes(plaintext, formID)
	return updated, err == nil, err
}
