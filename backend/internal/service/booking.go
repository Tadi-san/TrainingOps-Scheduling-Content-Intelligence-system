package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"trainingops/internal/model"
	"trainingops/internal/repository"
)

var (
	ErrRescheduleLimitReached = errors.New("max 2 reschedules reached")
	ErrCancellationClosed     = errors.New("cancellation cutoff is 24 hours before start")
)

type BookingService struct {
	Store   repository.BookingStore
	HoldTTL time.Duration
}

func NewBookingService(store repository.BookingStore) *BookingService {
	return &BookingService{Store: store, HoldTTL: 5 * time.Minute}
}

func (s *BookingService) Hold(ctx context.Context, booking model.Booking, now time.Time) (model.Booking, []model.BookingConflict, []model.AlternativeSlot, error) {
	held, conflicts, err := s.Store.CreateHold(ctx, booking, s.HoldTTL, now)
	if err == nil {
		return held, nil, nil, nil
	}

	if len(conflicts) == 0 {
		return model.Booking{}, nil, nil, err
	}

	alternatives, altErr := s.SuggestAlternatives(ctx, booking, now)
	if altErr != nil {
		return model.Booking{}, conflicts, nil, altErr
	}
	return model.Booking{}, conflicts, alternatives, err
}

func (s *BookingService) Confirm(ctx context.Context, tenantID, bookingID string, now time.Time) error {
	return s.Store.Confirm(ctx, tenantID, bookingID, now)
}

func (s *BookingService) Cancel(ctx context.Context, tenantID, bookingID string, now time.Time) error {
	booking, err := s.Store.GetBooking(ctx, tenantID, bookingID)
	if err != nil {
		return err
	}
	if booking.StartAt.Sub(now) < 24*time.Hour {
		return ErrCancellationClosed
	}
	return s.Store.Cancel(ctx, tenantID, bookingID, now)
}

func (s *BookingService) CheckIn(ctx context.Context, tenantID, bookingID string, now time.Time) error {
	return s.Store.CheckIn(ctx, tenantID, bookingID, now)
}

func (s *BookingService) Reschedule(ctx context.Context, tenantID, bookingID string, newStart, newEnd time.Time, now time.Time) (model.Booking, error) {
	booking, err := s.Store.GetBooking(ctx, tenantID, bookingID)
	if err != nil {
		return model.Booking{}, err
	}
	if booking.RescheduleCount >= 2 {
		return model.Booking{}, ErrRescheduleLimitReached
	}
	return s.Store.Reschedule(ctx, tenantID, bookingID, newStart, newEnd, now)
}

func (s *BookingService) ReleaseExpiredHolds(ctx context.Context, now time.Time) (int, error) {
	return s.Store.ReleaseExpiredHolds(ctx, now)
}

func (s *BookingService) SuggestAlternatives(ctx context.Context, booking model.Booking, now time.Time) ([]model.AlternativeSlot, error) {
	bookings, err := s.Store.ListBookings(ctx, booking.TenantID)
	if err != nil {
		return nil, err
	}
	rooms, err := s.Store.ListRooms(ctx, booking.TenantID)
	if err != nil {
		return nil, err
	}
	instructors, err := s.Store.ListInstructors(ctx, booking.TenantID)
	if err != nil {
		return nil, err
	}

	sort.Slice(rooms, func(i, j int) bool { return rooms[i].ID < rooms[j].ID })
	sort.Slice(instructors, func(i, j int) bool { return instructors[i].ID < instructors[j].ID })

	var alternatives []model.AlternativeSlot
	for offset := 1; offset <= 6 && len(alternatives) < 3; offset++ {
		start := booking.StartAt.Add(time.Duration(offset) * 30 * time.Minute)
		end := booking.EndAt.Add(time.Duration(offset) * 30 * time.Minute)
		for _, room := range rooms {
			if booking.Attendees > room.Capacity {
				continue
			}
			for _, instructor := range instructors {
				candidate := model.Booking{
					TenantID:     booking.TenantID,
					RoomID:       room.ID,
					InstructorID: instructor.ID,
					StartAt:      start,
					EndAt:        end,
					Attendees:    booking.Attendees,
				}
				if hasConflict(bookings, candidate, now) {
					continue
				}
				reason := "room and instructor availability"
				if room.ID != booking.RoomID && instructor.ID != booking.InstructorID {
					reason = "alternate room and instructor"
				} else if room.ID != booking.RoomID {
					reason = "alternate room"
				} else if instructor.ID != booking.InstructorID {
					reason = "alternate instructor"
				}
				alternatives = append(alternatives, model.AlternativeSlot{
					RoomID:       room.ID,
					InstructorID: instructor.ID,
					StartAt:      start,
					EndAt:        end,
					Reason:       reason,
				})
				if len(alternatives) == 3 {
					return alternatives, nil
				}
			}
		}
	}

	if len(alternatives) < 3 {
		for offset := 1; offset <= 3 && len(alternatives) < 3; offset++ {
			start := booking.StartAt.Add(time.Duration(offset) * time.Hour)
			end := booking.EndAt.Add(time.Duration(offset) * time.Hour)
			var roomCapacity int
			for _, room := range rooms {
				if room.ID == booking.RoomID {
					roomCapacity = room.Capacity
					break
				}
			}
			if roomCapacity > 0 && booking.Attendees > roomCapacity {
				continue
			}
			if hasConflict(bookings, model.Booking{
				TenantID:     booking.TenantID,
				RoomID:       booking.RoomID,
				InstructorID: booking.InstructorID,
				StartAt:      start,
				EndAt:        end,
				Attendees:    booking.Attendees,
			}, now) {
				continue
			}
			alternatives = append(alternatives, model.AlternativeSlot{
				RoomID:       booking.RoomID,
				InstructorID: booking.InstructorID,
				StartAt:      start,
				EndAt:        end,
				Reason:       fmt.Sprintf("shift by %d hour(s)", offset),
			})
		}
	}
	return alternatives, nil
}

func hasConflict(bookings []model.Booking, candidate model.Booking, now time.Time) bool {
	for _, booking := range bookings {
		if booking.TenantID != candidate.TenantID {
			continue
		}
		if booking.Status == model.BookingStatusCancelled || booking.Status == model.BookingStatusExpired {
			continue
		}
		if booking.HoldExpiresAt != nil && now.After(*booking.HoldExpiresAt) {
			continue
		}
		if candidate.StartAt.Before(booking.EndAt) && booking.StartAt.Before(candidate.EndAt) {
			if candidate.RoomID == booking.RoomID || candidate.InstructorID == booking.InstructorID {
				return true
			}
		}
	}
	return false
}
