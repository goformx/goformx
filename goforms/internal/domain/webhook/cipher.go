package webhook

import (
	"bytes"
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
	aead     cipher.AEAD
	activeID string
	keys     map[string]cipher.AEAD
	legacy   cipher.AEAD
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
	return c.encryptBytes(plaintext, formID)
}

func (c *Cipher) encryptBytes(plaintext []byte, formID string) ([]byte, error) {
	var header []byte
	aad := []byte(formID)
	if c.activeID != "" {
		header = []byte(envelopePrefix + c.activeID + ":")
		aad = envelopeAAD(header, formID)
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate webhook nonce: %w", err)
	}
	return c.aead.Seal(append(header, nonce...), nonce, plaintext, aad), nil
}

func (c *Cipher) Decrypt(ciphertext []byte, formID string) (SecretConfig, error) {
	plaintext, _, err := c.decryptBytes(ciphertext, formID)
	if err != nil {
		return SecretConfig{}, err
	}
	var config SecretConfig
	if err := json.Unmarshal(plaintext, &config); err != nil {
		return SecretConfig{}, errors.New("encrypted webhook configuration is invalid")
	}
	return config, nil
}

func (c *Cipher) decryptBytes(ciphertext []byte, formID string) ([]byte, string, error) {
	if c == nil {
		return nil, "", ErrDisabled
	}
	aead, aad, keyID := c.aead, []byte(formID), ""
	if bytes.HasPrefix(ciphertext, []byte(envelopePrefix)) {
		prefixLength := len(envelopePrefix)
		if len(ciphertext) <= prefixLength {
			return nil, "", errCipherAuthentication
		}
		idLength := bytes.IndexByte(ciphertext[prefixLength:], ':')
		if idLength < 1 || idLength > MaxEncryptionKeyIDBytes {
			return nil, "", errCipherAuthentication
		}
		headerLength := prefixLength + idLength + 1
		keyID = string(ciphertext[prefixLength : headerLength-1])
		aead = c.keys[keyID]
		aad = envelopeAAD(ciphertext[:headerLength], formID)
		ciphertext = ciphertext[headerLength:]
	} else if c.activeID != "" {
		aead = c.legacy
	}
	if aead == nil || len(ciphertext) < aead.NonceSize()+aead.Overhead() {
		return nil, "", errCipherAuthentication
	}
	nonce, sealed := ciphertext[:aead.NonceSize()], ciphertext[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, sealed, aad)
	if err != nil {
		return nil, "", errCipherAuthentication
	}
	return plaintext, keyID, nil
}
