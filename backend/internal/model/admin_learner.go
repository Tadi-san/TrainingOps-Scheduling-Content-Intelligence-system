package model

import "time"

type LearnerReservation struct {
	ID            string
	TenantID      string
	BookingID     string
	LearnerUserID string
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type TenantPolicy struct {
	TenantID string
	Policies map[string]any
}
