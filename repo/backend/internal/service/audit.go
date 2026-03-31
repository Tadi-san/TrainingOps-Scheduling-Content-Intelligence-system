package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"trainingops/internal/audit"
)

type AuditEntry struct {
	ActorUserID string
	OldState    string
	NewState    string
	Reason      string
	Who         string
	When        string
	Action      string
	EntityType  string
	EntityID    string
	TenantID    string
	CreatedAt   time.Time
}

type AuditStore interface {
	WriteAuditTransition(ctx context.Context, entry AuditEntry) error
}

type AuditRecorder struct {
	store   AuditStore
	logger  *slog.Logger
	mu      sync.Mutex
	entries []AuditEntry
}

func NewAuditRecorder(store AuditStore, logger *slog.Logger) *AuditRecorder {
	if logger == nil {
		logger = slog.Default()
	}
	return &AuditRecorder{
		store:   store,
		logger:  logger,
		entries: []AuditEntry{},
	}
}

func (r *AuditRecorder) RecordTransition(ctx context.Context, action, entityType, entityID, tenantID, oldState, newState string, md audit.Metadata, now time.Time) {
	entry := AuditEntry{
		ActorUserID: md.ActorUserID,
		OldState:    oldState,
		NewState:    newState,
		Reason:      md.Reason,
		Who:         md.Who,
		When:        md.When,
		Action:      action,
		EntityType:  entityType,
		EntityID:    entityID,
		TenantID:    tenantID,
		CreatedAt:   now,
	}

	if r.store != nil {
		if err := r.store.WriteAuditTransition(ctx, entry); err != nil {
			r.logger.Error("failed to persist audit transition", "error", err, "action", action, "entity_id", entityID, "tenant_id", tenantID)
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, entry)
}

func (r *AuditRecorder) Entries() []AuditEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]AuditEntry, len(r.entries))
	copy(out, r.entries)
	return out
}
