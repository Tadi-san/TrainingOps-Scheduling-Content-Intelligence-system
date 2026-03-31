package repository

import (
	"context"
	"time"

	"trainingops/internal/model"
)

type UserStore interface {
	GetByEmail(ctx context.Context, tenantID, email string) (*model.User, error)
	IncrementFailedAttempts(ctx context.Context, tenantID, userID string, now time.Time) error
	ResetFailedAttempts(ctx context.Context, tenantID, userID string) error
}

type UserCreator interface {
	CreateUser(ctx context.Context, user model.User) error
}

type SessionStore interface {
	CreateSession(ctx context.Context, session model.Session) error
	Revoke(ctx context.Context, tenantID, sessionID string, revokedAt time.Time) error
	IsActive(ctx context.Context, tenantID, sessionID string, now time.Time) (bool, error)
	Rotate(ctx context.Context, tenantID, oldSessionID string, next model.Session, revokedAt time.Time) error
}

type BookingStore interface {
	GetBooking(ctx context.Context, tenantID, bookingID string) (*model.Booking, error)
	CreateHold(ctx context.Context, booking model.Booking, holdTTL time.Duration, now time.Time) (model.Booking, []model.BookingConflict, error)
	Confirm(ctx context.Context, tenantID, bookingID string, now time.Time) error
	Cancel(ctx context.Context, tenantID, bookingID string, now time.Time) error
	CheckIn(ctx context.Context, tenantID, bookingID string, now time.Time) error
	Reschedule(ctx context.Context, tenantID, bookingID string, newStart, newEnd time.Time, now time.Time) (model.Booking, error)
	ReleaseExpiredHolds(ctx context.Context, now time.Time) (int, error)
	ListBookings(ctx context.Context, tenantID string) ([]model.Booking, error)
	ListRooms(ctx context.Context, tenantID string) ([]model.Room, error)
	ListInstructors(ctx context.Context, tenantID string) ([]model.Instructor, error)
}

type ContentStore interface {
	UpsertContentItem(ctx context.Context, item model.ContentItem) error
	DeleteContentItem(ctx context.Context, tenantID, itemID string) error
	ListContentItems(ctx context.Context, tenantID string) ([]model.ContentItem, error)
	UpsertTag(ctx context.Context, tag model.ContentTag) error
	DeleteTag(ctx context.Context, tenantID, tagID string) error
	ListTags(ctx context.Context, tenantID string) ([]model.ContentTag, error)
	SaveDocument(ctx context.Context, document model.Document) error
	ListDocuments(ctx context.Context, tenantID string) ([]model.Document, error)
	AddDocumentVersion(ctx context.Context, version model.DocumentVersion) (model.DocumentVersion, error)
	ListDocumentVersions(ctx context.Context, tenantID, documentID string) ([]model.DocumentVersion, error)
	SaveUploadSession(ctx context.Context, session model.UploadSession) error
	GetUploadSession(ctx context.Context, tenantID, sessionID string) (*model.UploadSession, error)
	DeleteUploadSession(ctx context.Context, tenantID, sessionID string) error
}

type TaskStore interface {
	UpsertTask(ctx context.Context, task model.Task) error
	GetTask(ctx context.Context, tenantID, taskID string) (*model.Task, error)
	ListTasks(ctx context.Context, tenantID string) ([]model.Task, error)
	DeleteTask(ctx context.Context, tenantID, taskID string) error
}

type TenantStore interface {
	CountTenants(ctx context.Context) (int, error)
	CreateTenant(ctx context.Context, name, slug string, now time.Time) (*model.Tenant, error)
}
