package model

import "time"

type BookingStatus string

const (
	BookingStatusHeld      BookingStatus = "held"
	BookingStatusConfirmed BookingStatus = "confirmed"
	BookingStatusCheckedIn BookingStatus = "checked_in"
	BookingStatusCancelled BookingStatus = "cancelled"
	BookingStatusExpired   BookingStatus = "expired"
)

type BookingConflictReason string

const (
	BookingConflictRoom       BookingConflictReason = "room"
	BookingConflictInstructor BookingConflictReason = "instructor"
	BookingConflictCapacity   BookingConflictReason = "capacity"
)

type Room struct {
	ID        string
	TenantID  string
	Name      string
	Capacity  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Instructor struct {
	ID        string
	TenantID  string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Booking struct {
	ID              string
	TenantID        string
	UserID          string
	RoomID          string
	InstructorID    string
	Title           string
	StartAt         time.Time
	EndAt           time.Time
	Capacity        int
	Attendees       int
	Status          BookingStatus
	HoldExpiresAt   *time.Time
	RescheduleCount int
	Version         int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CancelledAt     *time.Time
	CheckedInAt     *time.Time
}

type BookingConflict struct {
	Reason BookingConflictReason
	Detail string
}

type AlternativeSlot struct {
	RoomID       string
	InstructorID string
	StartAt      time.Time
	EndAt        time.Time
	Reason       string
}
