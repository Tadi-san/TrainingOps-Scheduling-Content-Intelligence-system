package bcrypt

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

const DefaultCost = 10

func GenerateFromPassword(password []byte, cost int) ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(append(salt, password...))
	encoded := base64.RawStdEncoding.EncodeToString(salt) + ":" + base64.RawStdEncoding.EncodeToString(sum[:])
	return []byte(encoded), nil
}

func CompareHashAndPassword(hashedPassword, password []byte) error {
	parts := bytesSplitN(hashedPassword, ':', 2)
	if len(parts) != 2 {
		return errors.New("invalid hash")
	}
	salt, err := base64.RawStdEncoding.DecodeString(string(parts[0]))
	if err != nil {
		return err
	}
	expected, err := base64.RawStdEncoding.DecodeString(string(parts[1]))
	if err != nil {
		return err
	}
	sum := sha256.Sum256(append(salt, password...))
	if !equal(sum[:], expected) {
		return errors.New("mismatch")
	}
	return nil
}

func bytesSplitN(b []byte, sep byte, n int) [][]byte {
	var out [][]byte
	start := 0
	for i := 0; i < len(b) && len(out) < n-1; i++ {
		if b[i] == sep {
			out = append(out, b[start:i])
			start = i + 1
		}
	}
	out = append(out, b[start:])
	return out
}

func equal(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
