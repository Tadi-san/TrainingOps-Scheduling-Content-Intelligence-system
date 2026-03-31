package postgres

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"trainingops/internal/model"
)

var slugCleaner = regexp.MustCompile(`[^a-z0-9-]+`)

func (s *Store) CountTenants(ctx context.Context) (int, error) {
	var count int
	if err := s.Pool.QueryRow(ctx, `SELECT COUNT(1) FROM tenants`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) CreateTenant(ctx context.Context, name, slug string, now time.Time) (*model.Tenant, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if strings.TrimSpace(slug) == "" {
		slug = toSlug(name)
	}
	if slug == "" {
		slug = fmt.Sprintf("tenant-%d", now.Unix())
	}

	var tenant model.Tenant
	if err := tx.QueryRow(ctx, `
		INSERT INTO tenants (name, slug, created_at, updated_at)
		VALUES ($1, $2, $3, $3)
		RETURNING id::text, name, slug, created_at, updated_at`,
		strings.TrimSpace(name), slug, now.UTC(),
	).Scan(&tenant.ID, &tenant.Name, &tenant.Slug, &tenant.CreatedAt, &tenant.UpdatedAt); err != nil {
		return nil, err
	}

	roleNames := []string{"admin", "coordinator", "instructor", "learner"}
	for _, roleName := range roleNames {
		if _, err := tx.Exec(ctx, `
			INSERT INTO roles (tenant_id, name, created_at, updated_at)
			VALUES ($1::uuid, $2, $3, $3)
			ON CONFLICT (tenant_id, name) DO NOTHING`,
			tenant.ID, roleName, now.UTC(),
		); err != nil {
			return nil, err
		}
	}

	permissionKeys := []string{
		"bookings.manage",
		"bookings.view",
		"content.manage",
		"content.view",
		"tasks.manage",
		"tasks.view",
		"reports.view",
		"tenants.manage",
		"users.manage",
	}
	for _, key := range permissionKeys {
		if _, err := tx.Exec(ctx, `
			INSERT INTO permissions (permission_key, created_at)
			VALUES ($1, $2)
			ON CONFLICT (permission_key) DO NOTHING`,
			key, now.UTC(),
		); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &tenant, nil
}

func toSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, " ", "-")
	value = slugCleaner.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	value = strings.ReplaceAll(value, "--", "-")
	return value
}
