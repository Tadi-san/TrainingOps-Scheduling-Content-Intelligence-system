package config

import (
	"os"
	"strings"
)

type Config struct {
	AppEnv         string
	HTTPAddr       string
	DatabaseURL    string
	JWTSigningKey  string
	VaultMasterKey string
	CORSOrigins    []string
	StoragePath    string
	ReportsPath    string
}

func Load() Config {
	appEnv := getEnv("APP_ENV", "development")
	jwtKey := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if jwtKey == "" {
		jwtKey = strings.TrimSpace(os.Getenv("JWT_SIGNING_KEY"))
	}
	if !IsTestMode() && len(jwtKey) < 32 {
		panic("JWT_SECRET must be set to a strong secret (min 32 chars)")
	}
	return Config{
		AppEnv:         appEnv,
		HTTPAddr:       getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSigningKey:  jwtKey,
		VaultMasterKey: getEnv("VAULT_MASTER_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"),
		CORSOrigins:    parseCSV(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000,http://localhost:5173")),
		StoragePath:    getEnv("STORAGE_PATH", "./uploads"),
		ReportsPath:    getEnv("REPORTS_PATH", "./reports"),
	}
}

func IsTestMode() bool {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("GO_ENV")), "test") {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "test") {
		return true
	}
	return false
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func parseCSV(value string) []string {
	parts := strings.Split(value, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
