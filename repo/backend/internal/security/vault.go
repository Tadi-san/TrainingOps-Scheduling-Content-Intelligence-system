package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"strings"
)

var ErrInvalidVaultKey = errors.New("vault master key must be 32 hex characters for AES-256")

type Vault struct {
	key []byte
}

func NewVault(masterKey string) (*Vault, error) {
	key, err := hex.DecodeString(masterKey)
	if err != nil || len(key) != 32 {
		return nil, ErrInvalidVaultKey
	}
	return &Vault{key: key}, nil
}

func (v *Vault) Encrypt(plainText []byte) ([]byte, error) {
	block, err := aes.NewCipher(v.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plainText, nil), nil
}

func (v *Vault) Decrypt(cipherText []byte) ([]byte, error) {
	block, err := aes.NewCipher(v.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(cipherText) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, payload := cipherText[:nonceSize], cipherText[nonceSize:]
	return gcm.Open(nil, nonce, payload, nil)
}

func Mask(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 4 {
		return strings.Repeat("*", len(value))
	}
	return value[:2] + strings.Repeat("*", len(value)-4) + value[len(value)-2:]
}
