package postgres

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"trainingops/internal/model"
	"trainingops/internal/security"
)

type Store struct {
	Pool  *pgxpool.Pool
	Vault *security.Vault
}

func NewStore(pool *pgxpool.Pool, vault *security.Vault) *Store {
	return &Store{Pool: pool, Vault: vault}
}

func (s *Store) GetByEmail(ctx context.Context, tenantID, email string) (*model.User, error) {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	row := s.Pool.QueryRow(ctx, `
		SELECT u.id::text, u.tenant_id::text, u.email, u.display_name, u.password_hash,
		       u.failed_attempts, u.locked_until, u.pii_encrypted, u.password_changed_at, u.created_at, u.updated_at,
		       COALESCE(r.name, 'learner') AS role
		FROM users u
		LEFT JOIN user_roles ur ON ur.tenant_id = u.tenant_id AND ur.user_id = u.id
		LEFT JOIN roles r ON r.id = ur.role_id
		WHERE u.tenant_id = $1::uuid AND lower(u.email) = lower($2)
		LIMIT 1`, tenantUUID, email)

	var user model.User
	var lockedUntil *time.Time
	var role string
	var piiEncrypted []byte
	if err := row.Scan(
		&user.ID, &user.TenantID, &user.Email, &user.DisplayName, &user.PasswordHash,
		&user.FailedAttempts, &lockedUntil, &piiEncrypted, &user.PasswordChangedAt, &user.CreatedAt, &user.UpdatedAt,
		&role,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("not found")
		}
		return nil, err
	}
	user.LockedUntil = lockedUntil
	user.PIIEncrypted = piiEncrypted
	user.Role = role
	return &user, nil
}

func (s *Store) CreateUser(ctx context.Context, user model.User) error {
	tenantUUID, err := s.resolveTenantUUID(ctx, user.TenantID)
	if err != nil {
		return err
	}
	var encryptedPII []byte
	if s.Vault != nil {
		encryptedPII, _ = s.Vault.Encrypt([]byte(user.Email))
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO users (tenant_id, email, display_name, password_hash, pii_encrypted, password_changed_at, created_at, updated_at)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, now(), now())
		ON CONFLICT (tenant_id, email) DO NOTHING`,
		tenantUUID, user.Email, user.DisplayName, user.PasswordHash, encryptedPII, user.PasswordChangedAt,
	)
	if err != nil {
		return err
	}
	if user.Role != "" {
		var roleID string
		roleRow := s.Pool.QueryRow(ctx, `SELECT id::text FROM roles WHERE tenant_id = $1::uuid AND name = $2`, tenantUUID, user.Role)
		if scanErr := roleRow.Scan(&roleID); scanErr != nil {
			return nil
		}
		var userID string
		if err := s.Pool.QueryRow(ctx, `SELECT id::text FROM users WHERE tenant_id = $1::uuid AND lower(email)=lower($2)`, tenantUUID, user.Email).Scan(&userID); err != nil {
			return nil
		}
		_, _ = s.Pool.Exec(ctx, `
			INSERT INTO user_roles (tenant_id, user_id, role_id, created_at)
			VALUES ($1::uuid, $2::uuid, $3::uuid, now())
			ON CONFLICT (tenant_id, user_id, role_id) DO NOTHING`,
			tenantUUID, userID, roleID,
		)
	}
	return nil
}

func (s *Store) IncrementFailedAttempts(ctx context.Context, tenantID, userID string, now time.Time) error {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `
		UPDATE users
		SET failed_attempts = failed_attempts + 1,
		    locked_until = CASE WHEN failed_attempts + 1 >= 5 THEN $4 ELSE locked_until END,
		    updated_at = now()
		WHERE tenant_id = $1::uuid AND id = $2::uuid`,
		tenantUUID, userID, now, now.Add(15*time.Minute),
	)
	return err
}

func (s *Store) ResetFailedAttempts(ctx context.Context, tenantID, userID string) error {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `
		UPDATE users SET failed_attempts = 0, locked_until = NULL, updated_at = now()
		WHERE tenant_id = $1::uuid AND id = $2::uuid`,
		tenantUUID, userID,
	)
	return err
}

func (s *Store) CreateSession(ctx context.Context, session model.Session) error {
	tenantUUID, err := s.resolveTenantUUID(ctx, session.TenantID)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO sessions (
			id, tenant_id, user_id, refresh_token_hash, expires_at, revoked_at, created_at, updated_at, last_used_at
		) VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8, $9)`,
		session.ID, tenantUUID, session.UserID, session.RefreshTokenHash, session.ExpiresAt, session.RevokedAt, session.CreatedAt, session.UpdatedAt, session.LastUsedAt,
	)
	return err
}

func (s *Store) Revoke(ctx context.Context, tenantID, sessionID string, revokedAt time.Time) error {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `
		UPDATE sessions
		SET revoked_at = $3, updated_at = now()
		WHERE tenant_id = $1::uuid AND id = $2::uuid`,
		tenantUUID, sessionID, revokedAt,
	)
	return err
}

func (s *Store) IsActive(ctx context.Context, tenantID, sessionID string, now time.Time) (bool, error) {
	logger := slog.Default()
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		logger.Error("IsActive: tenant resolve failed", "tenant", tenantID, "error", err)
		return false, err
	}
	row := s.Pool.QueryRow(ctx, `
		SELECT CASE
			WHEN revoked_at IS NOT NULL THEN false
			WHEN expires_at <= $3 THEN false
			ELSE true
		END
		FROM sessions
		WHERE tenant_id = $1::uuid AND id = $2::uuid`,
		tenantUUID, sessionID, now,
	)
	var active bool
	if err := row.Scan(&active); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warn("IsActive: session not found", "tenantUUID", tenantUUID, "sessionID", sessionID)
			return false, nil
		}
		logger.Error("IsActive: scan failed", "error", err)
		return false, err
	}
	if !active {
		logger.Warn("IsActive: session expired or revoked in DB", "tenantUUID", tenantUUID, "sessionID", sessionID)
	}
	return active, nil
}

func (s *Store) Rotate(ctx context.Context, tenantID, oldSessionID string, next model.Session, revokedAt time.Time) error {
	tenantUUID, err := s.resolveTenantUUID(ctx, tenantID)
	if err != nil {
		return err
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE sessions
		SET revoked_at = $3, updated_at = now(), replaced_by_session_id = $4::uuid
		WHERE tenant_id = $1::uuid AND id = $2::uuid`,
		tenantUUID, oldSessionID, revokedAt, next.ID,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO sessions (
			id, tenant_id, user_id, refresh_token_hash, expires_at, revoked_at, created_at, updated_at, last_used_at
		) VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, NULL, $6, $7, $8)`,
		next.ID, tenantUUID, next.UserID, next.RefreshTokenHash, next.ExpiresAt, next.CreatedAt, next.UpdatedAt, next.LastUsedAt,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) resolveTenantUUID(ctx context.Context, tenant string) (string, error) {
	if s.Pool == nil {
		return "", errors.New("postgres pool is not configured")
	}
	row := s.Pool.QueryRow(ctx, `SELECT id::text FROM tenants WHERE id::text = $1 OR slug = $1 LIMIT 1`, tenant)
	var tenantID string
	if err := row.Scan(&tenantID); err != nil {
		return "", err
	}
	return tenantID, nil
}
