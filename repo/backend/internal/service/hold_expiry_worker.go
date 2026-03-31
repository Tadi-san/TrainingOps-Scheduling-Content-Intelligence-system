package service

import (
	"context"
	"log/slog"
	"time"

	"trainingops/internal/audit"
	"trainingops/internal/model"
)

type HoldExpiryStore interface {
	ExpireHeldBookings(ctx context.Context, now time.Time) ([]model.Booking, error)
}

type HoldExpiryWorker struct {
	store    HoldExpiryStore
	audit    *AuditRecorder
	logger   *slog.Logger
	interval time.Duration
}

func NewHoldExpiryWorker(store HoldExpiryStore, auditRecorder *AuditRecorder, logger *slog.Logger, interval time.Duration) *HoldExpiryWorker {
	if logger == nil {
		logger = slog.Default()
	}
	if interval <= 0 {
		interval = time.Minute
	}
	return &HoldExpiryWorker{
		store:    store,
		audit:    auditRecorder,
		logger:   logger,
		interval: interval,
	}
}

func (w *HoldExpiryWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.runOnce(ctx, time.Now().UTC())

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("hold expiry worker stopped")
			return
		case <-ticker.C:
			w.runOnce(ctx, time.Now().UTC())
		}
	}
}

func (w *HoldExpiryWorker) runOnce(ctx context.Context, now time.Time) {
	expired, err := w.store.ExpireHeldBookings(ctx, now)
	if err != nil {
		w.logger.Error("hold expiry worker failed", "error", err)
		return
	}
	for _, booking := range expired {
		w.audit.RecordTransition(ctx, "booking_expired", "booking", booking.ID, booking.TenantID, string(model.BookingStatusHeld), string(model.BookingStatusExpired), audit.Metadata{
			Who:    "system",
			When:   now.Format(time.RFC3339),
			Reason: "hold timeout",
		}, now)
	}
	if len(expired) > 0 {
		w.logger.Info("expired held bookings", "count", len(expired))
	}
}
