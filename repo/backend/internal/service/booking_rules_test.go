package service

import (
	"context"
	"testing"
	"time"

	"trainingops/internal/model"
	"trainingops/internal/repository/memory"
)

func TestCancelWithin24HoursIsBlocked(t *testing.T) {
	repo := memory.NewBookingRepository()
	repo.SaveRoom(model.Room{ID: "r1", TenantID: "tenant-1", Capacity: 20})
	repo.SaveInstructor(model.Instructor{ID: "i1", TenantID: "tenant-1"})
	svc := NewBookingService(repo)
	now := time.Now().UTC()
	held, _, _, err := svc.Hold(context.Background(), model.Booking{
		TenantID:     "tenant-1",
		RoomID:       "r1",
		InstructorID: "i1",
		Title:        "Session",
		StartAt:      now.Add(2 * time.Hour),
		EndAt:        now.Add(3 * time.Hour),
		Attendees:    10,
	}, now)
	if err != nil {
		t.Fatalf("hold failed: %v", err)
	}
	if err := svc.Cancel(context.Background(), "tenant-1", held.ID, now); err != ErrCancellationClosed {
		t.Fatalf("expected ErrCancellationClosed, got %v", err)
	}
}

func TestRescheduleThirdAttemptBlocked(t *testing.T) {
	repo := memory.NewBookingRepository()
	repo.SaveRoom(model.Room{ID: "r1", TenantID: "tenant-1", Capacity: 20})
	repo.SaveInstructor(model.Instructor{ID: "i1", TenantID: "tenant-1"})
	svc := NewBookingService(repo)
	now := time.Now().UTC()
	held, _, _, err := svc.Hold(context.Background(), model.Booking{
		TenantID:     "tenant-1",
		RoomID:       "r1",
		InstructorID: "i1",
		Title:        "Session",
		StartAt:      now.Add(48 * time.Hour),
		EndAt:        now.Add(49 * time.Hour),
		Attendees:    10,
	}, now)
	if err != nil {
		t.Fatalf("hold failed: %v", err)
	}

	start := held.StartAt
	end := held.EndAt
	for i := 0; i < 2; i++ {
		start = start.Add(2 * time.Hour)
		end = end.Add(2 * time.Hour)
		if _, err := svc.Reschedule(context.Background(), "tenant-1", held.ID, start, end, now.Add(time.Duration(i+1)*time.Hour)); err != nil {
			t.Fatalf("unexpected reschedule error on attempt %d: %v", i+1, err)
		}
	}

	_, err = svc.Reschedule(context.Background(), "tenant-1", held.ID, start.Add(2*time.Hour), end.Add(2*time.Hour), now.Add(4*time.Hour))
	if err != ErrRescheduleLimitReached {
		t.Fatalf("expected ErrRescheduleLimitReached, got %v", err)
	}
}
