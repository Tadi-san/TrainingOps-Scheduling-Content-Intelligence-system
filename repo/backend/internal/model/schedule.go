package model

import "time"

type ClassPeriod struct {
	ID        string
	TenantID  string
	Title     string
	StartTime string
	EndTime   string
	Weekday   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type BlackoutDate struct {
	ID           string
	TenantID     string
	BlackoutDate time.Time
	Reason       string
	CreatedAt    time.Time
}

type ScheduleConflict struct {
	HasConflict bool
	Reasons     []string
}
