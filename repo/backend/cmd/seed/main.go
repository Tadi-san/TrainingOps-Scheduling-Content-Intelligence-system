package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"trainingops/internal/repository/postgres"
	"trainingops/internal/security"
	"trainingops/internal/service"
)

func main() {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("failed to connect postgres: %v", err)
	}
	defer pool.Close()

	vault, err := security.NewVault(getEnv("VAULT_MASTER_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"))
	if err != nil {
		log.Fatalf("failed to init vault: %v", err)
	}
	store := postgres.NewStore(pool, vault)
	auth := service.NewAuthService(store, store, store)
	setup := service.NewSetupService(store, auth)

	password := strings.TrimSpace(os.Getenv("ADMIN_SEED_PASSWORD"))
	if password == "" {
		password = generatedSeedPassword()
	}
	email := strings.TrimSpace(getEnv("ADMIN_SEED_EMAIL", "admin@example.com"))

	tenant, err := setup.BootstrapTenant(context.Background(), "tenant-1", "tenant-1", email, "Default Admin", password, time.Now().UTC())
	if err != nil {
		if err == service.ErrSetupAlreadyCompleted {
			log.Println("seed skipped: tenant already exists")
			return
		}
		log.Fatalf("seed failed: %v", err)
	}

	fmt.Printf("Seed complete. tenant_id=%s tenant_slug=%s admin_email=%s\n", tenant.ID, tenant.Slug, email)
	if os.Getenv("ADMIN_SEED_PASSWORD") == "" {
		fmt.Printf("Generated admin password: %s\n", password)
	}
}

func generatedSeedPassword() string {
	buf := make([]byte, 18)
	_, _ = rand.Read(buf)
	raw := base64.RawURLEncoding.EncodeToString(buf)
	return "A1!" + raw + "zZ"
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value != "" {
		return value
	}
	return fallback
}
