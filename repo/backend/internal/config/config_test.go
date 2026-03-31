package config

import (
	"os"
	"testing"
)

func TestLoadPanicsWhenJWTSecretTooShortOutsideTestMode(t *testing.T) {
	originalGOEnv := os.Getenv("GO_ENV")
	originalAppEnv := os.Getenv("APP_ENV")
	originalJWT := os.Getenv("JWT_SECRET")
	originalFallback := os.Getenv("JWT_SIGNING_KEY")
	defer func() {
		_ = os.Setenv("GO_ENV", originalGOEnv)
		_ = os.Setenv("APP_ENV", originalAppEnv)
		_ = os.Setenv("JWT_SECRET", originalJWT)
		_ = os.Setenv("JWT_SIGNING_KEY", originalFallback)
	}()

	_ = os.Setenv("GO_ENV", "")
	_ = os.Setenv("APP_ENV", "development")
	_ = os.Setenv("JWT_SECRET", "short")
	_ = os.Unsetenv("JWT_SIGNING_KEY")

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when JWT secret is too short")
		}
	}()
	_ = Load()
}

func TestLoadAllowsShortSecretInTestMode(t *testing.T) {
	originalGOEnv := os.Getenv("GO_ENV")
	originalJWT := os.Getenv("JWT_SECRET")
	defer func() {
		_ = os.Setenv("GO_ENV", originalGOEnv)
		_ = os.Setenv("JWT_SECRET", originalJWT)
	}()

	_ = os.Setenv("GO_ENV", "test")
	_ = os.Setenv("JWT_SECRET", "short")
	cfg := Load()
	if cfg.JWTSigningKey != "short" {
		t.Fatalf("expected test secret to load, got %q", cfg.JWTSigningKey)
	}
}
