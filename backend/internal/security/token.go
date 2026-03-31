package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

func GenerateUUID() string {
	return uuid.New().String()
}

type SessionClaims struct {
	UserID    string `json:"user_id"`
	TenantID  string `json:"tenant_id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	SessionID string `json:"session_id"`
	ExpiresAt int64  `json:"expires_at"`
}

func SignSessionToken(secret string, claims SessionClaims) (string, error) {
	header, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	headerEncoded := base64.RawURLEncoding.EncodeToString(header)
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payload)
	signingInput := headerEncoded + "." + payloadEncoded
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signingInput + "." + signature, nil
}

func VerifySessionToken(secret, token string, now time.Time) (SessionClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return SessionClaims{}, errors.New("invalid token format")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[2]), []byte(expected)) {
		return SessionClaims{}, errors.New("invalid token signature")
	}

	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return SessionClaims{}, errors.New("invalid token payload")
	}

	var claims SessionClaims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return SessionClaims{}, errors.New("invalid token claims")
	}
	if claims.UserID == "" || claims.TenantID == "" || claims.ExpiresAt == 0 {
		return SessionClaims{}, errors.New("token missing required claims")
	}
	if now.Unix() >= claims.ExpiresAt {
		return SessionClaims{}, errors.New("token expired")
	}
	return claims, nil
}
