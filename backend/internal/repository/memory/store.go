package memory

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"trainingops/internal/model"
)

var ErrNotFound = errors.New("not found")
var ErrAlreadyExists = errors.New("already exists")

type Store struct {
	mu       sync.RWMutex
	users    map[string]model.User
	sessions map[string]model.Session
}

func NewStore() *Store {
	return &Store{
		users:    map[string]model.User{},
		sessions: map[string]model.Session{},
	}
}

func userKey(tenantID, email string) string {
	return tenantID + ":" + strings.ToLower(strings.TrimSpace(email))
}

func sessionKey(tenantID, sessionID string) string {
	return tenantID + ":" + sessionID
}

func (s *Store) GetByEmail(ctx context.Context, tenantID, email string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[userKey(tenantID, email)]
	if !ok {
		return nil, ErrNotFound
	}
	copy := user
	return &copy, nil
}

func (s *Store) SaveUser(user model.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[userKey(user.TenantID, user.Email)] = user
}

func (s *Store) CreateUser(ctx context.Context, user model.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := userKey(user.TenantID, user.Email)
	if _, ok := s.users[key]; ok {
		return ErrAlreadyExists
	}

	s.users[key] = user
	return nil
}

func (s *Store) IncrementFailedAttempts(ctx context.Context, tenantID, userID string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, user := range s.users {
		if user.TenantID == tenantID && user.ID == userID {
			user.FailedAttempts++
			if user.FailedAttempts >= 5 {
				lockUntil := now.Add(15 * time.Minute)
				user.LockedUntil = &lockUntil
			}
			s.users[key] = user
			return nil
		}
	}
	return ErrNotFound
}

func (s *Store) ResetFailedAttempts(ctx context.Context, tenantID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, user := range s.users {
		if user.TenantID == tenantID && user.ID == userID {
			user.FailedAttempts = 0
			user.LockedUntil = nil
			s.users[key] = user
			return nil
		}
	}
	return ErrNotFound
}

func (s *Store) CreateSession(ctx context.Context, session model.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionKey(session.TenantID, session.ID)] = session
	return nil
}

func (s *Store) Revoke(ctx context.Context, tenantID, sessionID string, revokedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := sessionKey(tenantID, sessionID)
	session, ok := s.sessions[key]
	if !ok {
		return ErrNotFound
	}
	session.RevokedAt = &revokedAt
	s.sessions[key] = session
	return nil
}

func (s *Store) IsActive(ctx context.Context, tenantID, sessionID string, now time.Time) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[sessionKey(tenantID, sessionID)]
	if !ok {
		return false, ErrNotFound
	}
	if session.RevokedAt != nil {
		return false, nil
	}
	if now.After(session.ExpiresAt) {
		return false, nil
	}
	return true, nil
}

func (s *Store) Rotate(ctx context.Context, tenantID, oldSessionID string, next model.Session, revokedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := sessionKey(tenantID, oldSessionID)
	current, ok := s.sessions[key]
	if !ok {
		return ErrNotFound
	}
	current.RevokedAt = &revokedAt
	current.UpdatedAt = revokedAt
	s.sessions[key] = current
	s.sessions[sessionKey(next.TenantID, next.ID)] = next
	return nil
}
