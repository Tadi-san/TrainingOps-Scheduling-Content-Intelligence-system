package model

import "time"

type ContentTag struct {
	ID        string
	TenantID  string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Document struct {
	ID             string
	TenantID       string
	Title          string
	Description    string
	CurrentVersion int
	Checksum       string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type DocumentVersion struct {
	ID         string
	TenantID   string
	DocumentID string
	Version    int
	FileName   string
	Checksum   string
	SizeBytes  int64
	CreatedAt  time.Time
}

type UploadSession struct {
	ID               string
	TenantID         string
	DocumentID       string
	FileName         string
	ExpectedChunks   int
	ReceivedChunks   map[int][]byte
	ExpectedChecksum string
	CreatedAt        time.Time
	ExpiresAt        time.Time
	UpdatedAt        time.Time
}

type ShareLink struct {
	URL        string
	Token      string
	DocumentID string
	TenantID   string
	ExpiresAt  time.Time
}
