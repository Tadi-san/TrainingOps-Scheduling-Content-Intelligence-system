package model

import "time"

type IngestionSession struct {
	ID           string
	TenantID     string
	ActorUserID  string
	Proxy        string
	UserAgent    string
	RequestCount int
	LastSeenAt   time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ManualReviewItem struct {
	ID        string
	TenantID  string
	JobID     string
	Reason    string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}
