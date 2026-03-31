package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"trainingops/internal/model"
	"trainingops/internal/repository/memory"
)

func TestBookingHold_IsAtomicUnderRace(t *testing.T) {
	repo := memory.NewBookingRepository()
	repo.SaveRoom(model.Room{ID: "room-1", TenantID: "tenant-1", Name: "Room 1", Capacity: 20})
	repo.SaveInstructor(model.Instructor{ID: "inst-1", TenantID: "tenant-1", Name: "Instructor 1"})

	svc := NewBookingService(repo)
	now := time.Date(2026, 3, 28, 9, 0, 0, 0, time.UTC)
	candidate := model.Booking{
		TenantID:     "tenant-1",
		RoomID:       "room-1",
		InstructorID: "inst-1",
		StartAt:      now.Add(2 * time.Hour),
		EndAt:        now.Add(3 * time.Hour),
		Attendees:    10,
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, conflicts, _, err := svc.Hold(context.Background(), candidate, now)
			if err == nil {
				results <- nil
				return
			}
			if len(conflicts) == 0 {
				results <- err
				return
			}
			results <- memory.ErrBookingConflict
		}()
	}

	close(start)
	wg.Wait()
	close(results)

	successes := 0
	conflicts := 0
	for result := range results {
		switch {
		case result == nil:
			successes++
		case errors.Is(result, memory.ErrBookingConflict):
			conflicts++
		default:
			t.Fatalf("unexpected result: %v", result)
		}
	}

	if successes != 1 || conflicts != 1 {
		t.Fatalf("expected one success and one conflict, got successes=%d conflicts=%d", successes, conflicts)
	}
}

func TestBookingHold_ReturnsConflictReasonsAndAlternatives(t *testing.T) {
	repo := memory.NewBookingRepository()
	repo.SaveRoom(model.Room{ID: "room-1", TenantID: "tenant-1", Name: "Room 1", Capacity: 5})
	repo.SaveRoom(model.Room{ID: "room-2", TenantID: "tenant-1", Name: "Room 2", Capacity: 10})
	repo.SaveInstructor(model.Instructor{ID: "inst-1", TenantID: "tenant-1", Name: "Instructor 1"})
	repo.SaveInstructor(model.Instructor{ID: "inst-2", TenantID: "tenant-1", Name: "Instructor 2"})

	now := time.Date(2026, 3, 28, 9, 0, 0, 0, time.UTC)
	if _, conflicts, err := repo.CreateHold(context.Background(), model.Booking{
		TenantID:     "tenant-1",
		RoomID:       "room-1",
		InstructorID: "inst-1",
		StartAt:      now.Add(2 * time.Hour),
		EndAt:        now.Add(3 * time.Hour),
		Attendees:    5,
	}, 5*time.Minute, now); err != nil || len(conflicts) != 0 {
		t.Fatalf("setup booking failed: err=%v conflicts=%v", err, conflicts)
	}

	svc := NewBookingService(repo)
	_, conflicts, alternatives, err := svc.Hold(context.Background(), model.Booking{
		TenantID:     "tenant-1",
		RoomID:       "room-1",
		InstructorID: "inst-1",
		StartAt:      now.Add(2 * time.Hour).Add(30 * time.Minute),
		EndAt:        now.Add(3 * time.Hour).Add(30 * time.Minute),
		Attendees:    8,
	}, now)
	if err == nil {
		t.Fatal("expected conflict")
	}
	if len(conflicts) == 0 {
		t.Fatal("expected conflict reasons")
	}
	if len(alternatives) != 3 {
		t.Fatalf("expected 3 alternatives, got %d", len(alternatives))
	}

	reasonSet := map[model.BookingConflictReason]bool{}
	for _, conflict := range conflicts {
		reasonSet[conflict.Reason] = true
	}
	if !reasonSet[model.BookingConflictRoom] || !reasonSet[model.BookingConflictInstructor] {
		t.Fatalf("expected room and instructor conflicts, got %#v", conflicts)
	}
}
