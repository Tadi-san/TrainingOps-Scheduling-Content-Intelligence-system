package model

import "time"

type Tenant struct {
	ID        string
	Name      string
	Slug      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type User struct {
	ID                string
	TenantID          string
	Email             string
	DisplayName       string
	Role              string
	PasswordHash      string
	FailedAttempts    int
	LockedUntil       *time.Time
	PIIEncrypted      []byte
	PasswordChangedAt time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Role struct {
	ID        string
	TenantID  string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Permission struct {
	ID        string
	Key       string
	CreatedAt time.Time
}

type ContentItem struct {
	ID              string
	TenantID        string
	CreatedByUserID string
	Title           string
	CategoryID      string
	Difficulty      int
	DurationM       int
	DurationMinutes int
	Version         int64
	Checksum        string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Session struct {
	ID               string
	TenantID         string
	UserID           string
	RefreshTokenHash string
	ExpiresAt        time.Time
	LastUsedAt       time.Time
	RevokedAt        *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
