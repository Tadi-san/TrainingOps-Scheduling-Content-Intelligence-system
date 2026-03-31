package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"trainingops/internal/model"
	"trainingops/internal/repository"
)

var (
	ErrSetupAlreadyCompleted = errors.New("tenant setup has already been completed")
)

type SetupService struct {
	Tenants repository.TenantStore
	Auth    *AuthService
}

func NewSetupService(tenants repository.TenantStore, auth *AuthService) *SetupService {
	return &SetupService{Tenants: tenants, Auth: auth}
}

func (s *SetupService) NeedsSetup(ctx context.Context) (bool, error) {
	count, err := s.Tenants.CountTenants(ctx)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

func (s *SetupService) BootstrapTenant(ctx context.Context, tenantName, tenantSlug, adminEmail, adminDisplayName, adminPassword string, now time.Time) (*model.Tenant, error) {
	needsSetup, err := s.NeedsSetup(ctx)
	if err != nil {
		return nil, err
	}
	if !needsSetup {
		return nil, ErrSetupAlreadyCompleted
	}

	tenantName = strings.TrimSpace(tenantName)
	tenantSlug = strings.TrimSpace(tenantSlug)
	adminEmail = strings.ToLower(strings.TrimSpace(adminEmail))
	adminDisplayName = strings.TrimSpace(adminDisplayName)
	if tenantName == "" || adminEmail == "" || adminDisplayName == "" || strings.TrimSpace(adminPassword) == "" {
		return nil, errors.New("tenant_name, admin_email, admin_username, and admin_password are required")
	}

	tenant, err := s.Tenants.CreateTenant(ctx, tenantName, tenantSlug, now)
	if err != nil {
		return nil, err
	}

	user := model.User{
		ID:          "",
		TenantID:    tenant.ID,
		Email:       adminEmail,
		DisplayName: adminDisplayName,
		Role:        "admin",
	}
	if err := s.Auth.Register(ctx, user, adminPassword, now); err != nil {
		return nil, err
	}
	return tenant, nil
}
