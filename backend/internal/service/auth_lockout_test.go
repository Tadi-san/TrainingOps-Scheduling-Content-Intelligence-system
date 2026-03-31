package service

import (
	"context"
	"testing"
	"time"

	"trainingops/internal/model"
	"trainingops/internal/repository/memory"
)

func TestAuthLockoutAfterFiveFailures(t *testing.T) {
	store := memory.NewStore()
	svc := NewAuthService(store, store, store)
	now := time.Now().UTC()
	user := model.User{
		ID:          "u1",
		TenantID:    "tenant-1",
		Email:       "lockout@example.com",
		DisplayName: "Lockout User",
		Role:        "learner",
	}
	if err := svc.Register(context.Background(), user, "Password123!A", now); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		_, _ = svc.Authenticate(context.Background(), "tenant-1", "lockout@example.com", "wrong-pass", now.Add(time.Duration(i)*time.Second))
	}

	_, err := svc.Authenticate(context.Background(), "tenant-1", "lockout@example.com", "Password123!A", now.Add(6*time.Second))
	if err != ErrAccountLocked {
		t.Fatalf("expected ErrAccountLocked, got %v", err)
	}
}

func TestAuthUnlockAfterFifteenMinutes(t *testing.T) {
	store := memory.NewStore()
	svc := NewAuthService(store, store, store)
	now := time.Now().UTC()
	user := model.User{
		ID:          "u2",
		TenantID:    "tenant-1",
		Email:       "unlock@example.com",
		DisplayName: "Unlock User",
		Role:        "learner",
	}
	if err := svc.Register(context.Background(), user, "Password123!A", now); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	for i := 0; i < 5; i++ {
		_, _ = svc.Authenticate(context.Background(), "tenant-1", "unlock@example.com", "wrong-pass", now.Add(time.Duration(i)*time.Second))
	}

	_, err := svc.Authenticate(context.Background(), "tenant-1", "unlock@example.com", "Password123!A", now.Add(16*time.Minute))
	if err != nil {
		t.Fatalf("expected unlock success, got %v", err)
	}
}
