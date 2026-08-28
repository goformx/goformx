package webhook

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const EncryptionKeyBytes = 32

type Cipher struct {
	aead cipher.AEAD
}

func NewCipher(encodedKey string) (*Cipher, error) {
	if encodedKey == "" {
		return nil, ErrDisabled
	}
	key, err := base64.RawStdEncoding.DecodeString(encodedKey)
	if err != nil {
		key, err = base64.StdEncoding.DecodeString(encodedKey)
	}
	if err != nil || len(key) != EncryptionKeyBytes {
		return nil, fmt.Errorf("webhook encryption key must be base64-encoded %d bytes", EncryptionKeyBytes)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create webhook cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create webhook AEAD: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

func (c *Cipher) Encrypt(config SecretConfig, formID string) ([]byte, error) {
	if c == nil {
		return nil, ErrDisabled
	}
	plaintext, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("encode webhook secrets: %w", err)
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate webhook nonce: %w", err)
	}
	return c.aead.Seal(nonce, nonce, plaintext, []byte(formID)), nil
}

func (c *Cipher) Decrypt(ciphertext []byte, formID string) (SecretConfig, error) {
	if c == nil {
		return SecretConfig{}, ErrDisabled
	}
	if len(ciphertext) < c.aead.NonceSize() {
		return SecretConfig{}, errors.New("encrypted webhook configuration is truncated")
	}
	nonce, sealed := ciphertext[:c.aead.NonceSize()], ciphertext[c.aead.NonceSize():]
	plaintext, err := c.aead.Open(nil, nonce, sealed, []byte(formID))
	if err != nil {
		return SecretConfig{}, errors.New("encrypted webhook configuration failed authentication")
	}
	var config SecretConfig
	if err := json.Unmarshal(plaintext, &config); err != nil {
		return SecretConfig{}, errors.New("encrypted webhook configuration is invalid")
	}
	return config, nil
}
