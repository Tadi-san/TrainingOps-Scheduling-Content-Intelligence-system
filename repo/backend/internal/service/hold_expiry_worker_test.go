package service

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"trainingops/internal/model"
)

type fakeHoldExpiryStore struct {
	items []model.Booking
}

func (f *fakeHoldExpiryStore) ExpireHeldBookings(ctx context.Context, now time.Time) ([]model.Booking, error) {
	out := make([]model.Booking, len(f.items))
	copy(out, f.items)
	return out, nil
}

func TestHoldExpiryWorkerWritesAuditEntries(t *testing.T) {
	recorder := NewAuditRecorder(nil, slog.Default())
	store := &fakeHoldExpiryStore{
		items: []model.Booking{
			{ID: "b1", TenantID: "tenant-1", Status: model.BookingStatusExpired},
			{ID: "b2", TenantID: "tenant-1", Status: model.BookingStatusExpired},
		},
	}
	worker := NewHoldExpiryWorker(store, recorder, slog.Default(), 10*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go worker.Start(ctx)
	time.Sleep(30 * time.Millisecond)
	cancel()
	time.Sleep(10 * time.Millisecond)

	entries := recorder.Entries()
	if len(entries) < 2 {
		t.Fatalf("expected at least 2 audit entries, got %d", len(entries))
	}
	found := 0
	for _, entry := range entries {
		if entry.Action == "booking_expired" {
			found++
		}
	}
	if found < 2 {
		t.Fatalf("expected booking_expired audit entries, got %d", found)
	}
}
