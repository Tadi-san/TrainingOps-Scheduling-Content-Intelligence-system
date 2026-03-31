package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"trainingops/internal/model"
	"trainingops/internal/repository"
	"trainingops/internal/security"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountLocked      = errors.New("account locked")
	ErrUserExists         = errors.New("user already exists")
)

type AuthService struct {
	Users    repository.UserStore
	Creator  repository.UserCreator
	Sessions repository.SessionStore
}

func NewAuthService(users repository.UserStore, creator repository.UserCreator, sessions repository.SessionStore) *AuthService {
	return &AuthService{Users: users, Creator: creator, Sessions: sessions}
}

func (s *AuthService) Register(ctx context.Context, user model.User, password string, now time.Time) error {
	if err := security.ValidatePassword(password); err != nil {
		return err
	}
	if s.Users != nil {
		if existing, err := s.Users.GetByEmail(ctx, user.TenantID, user.Email); err == nil && existing != nil {
			return ErrUserExists
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.PasswordHash = string(hash)
	user.PasswordChangedAt = now
	if s.Creator == nil {
		return errors.New("registration storage is not configured")
	}
	return s.Creator.CreateUser(ctx, user)
}

func (s *AuthService) Authenticate(ctx context.Context, tenantID, email, password string, now time.Time) (*model.User, error) {
	user, err := s.Users.GetByEmail(ctx, tenantID, email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if user.LockedUntil != nil && now.Before(*user.LockedUntil) {
		return nil, ErrAccountLocked
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		_ = s.Users.IncrementFailedAttempts(ctx, tenantID, user.ID, now)
		return nil, ErrInvalidCredentials
	}

	_ = s.Users.ResetFailedAttempts(ctx, tenantID, user.ID)
	return user, nil
}

func (s *AuthService) CreateSession(ctx context.Context, user model.User, now time.Time) (model.Session, error) {
	if s.Sessions == nil {
		return model.Session{}, errors.New("session storage is not configured")
	}

	session := model.Session{
		ID:               security.GenerateUUID(),
		TenantID:         user.TenantID,
		UserID:           user.ID,
		RefreshTokenHash: fmt.Sprintf("rt_%d", now.UnixNano()),
		ExpiresAt:        now.Add(24 * time.Hour),
		LastUsedAt:       now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.Sessions.CreateSession(ctx, session); err != nil {
		return model.Session{}, err
	}
	return session, nil
}
