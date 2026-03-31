package memory

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"trainingops/internal/model"
)

var ErrBookingNotFound = errors.New("booking not found")
var ErrBookingConflict = errors.New("booking conflict")

type BookingRepository struct {
	mu          sync.Mutex
	bookings    map[string]model.Booking
	rooms       map[string]model.Room
	instructors map[string]model.Instructor
}

func NewBookingRepository() *BookingRepository {
	return &BookingRepository{
		bookings:    map[string]model.Booking{},
		rooms:       map[string]model.Room{},
		instructors: map[string]model.Instructor{},
	}
}

func bookingKey(tenantID, bookingID string) string {
	return tenantID + ":" + bookingID
}

func roomKey(tenantID, roomID string) string {
	return tenantID + ":" + roomID
}

func instructorKey(tenantID, instructorID string) string {
	return tenantID + ":" + instructorID
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func overlaps(aStart, aEnd, bStart, bEnd time.Time) bool {
	return aStart.Before(bEnd) && bStart.Before(aEnd)
}

func (r *BookingRepository) SaveRoom(room model.Room) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rooms[roomKey(room.TenantID, room.ID)] = room
}

func (r *BookingRepository) SaveInstructor(instructor model.Instructor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.instructors[instructorKey(instructor.TenantID, instructor.ID)] = instructor
}

func (r *BookingRepository) CreateHold(ctx context.Context, booking model.Booking, holdTTL time.Duration, now time.Time) (model.Booking, []model.BookingConflict, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	conflicts := r.detectConflictsLocked(booking, now)
	if len(conflicts) > 0 {
		return model.Booking{}, conflicts, ErrBookingConflict
	}

	booking.Status = model.BookingStatusHeld
	booking.HoldExpiresAt = ptrTime(now.Add(holdTTL))
	booking.CreatedAt = now
	booking.UpdatedAt = now
	booking.Version = 1

	if booking.ID == "" {
		booking.ID = fmt.Sprintf("bk_%d", now.UnixNano())
	}

	r.bookings[bookingKey(booking.TenantID, booking.ID)] = booking
	return booking, nil, nil
}

func (r *BookingRepository) GetBooking(ctx context.Context, tenantID, bookingID string) (*model.Booking, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	booking, ok := r.bookings[bookingKey(tenantID, bookingID)]
	if !ok {
		return nil, ErrBookingNotFound
	}
	copy := booking
	return &copy, nil
}

func (r *BookingRepository) Confirm(ctx context.Context, tenantID, bookingID string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := bookingKey(tenantID, bookingID)
	booking, ok := r.bookings[key]
	if !ok {
		return ErrBookingNotFound
	}
	if booking.HoldExpiresAt != nil && now.After(*booking.HoldExpiresAt) {
		booking.Status = model.BookingStatusExpired
		r.bookings[key] = booking
		return ErrBookingConflict
	}
	booking.Status = model.BookingStatusConfirmed
	booking.HoldExpiresAt = nil
	booking.UpdatedAt = now
	booking.Version++
	r.bookings[key] = booking
	return nil
}

func (r *BookingRepository) Cancel(ctx context.Context, tenantID, bookingID string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := bookingKey(tenantID, bookingID)
	booking, ok := r.bookings[key]
	if !ok {
		return ErrBookingNotFound
	}
	booking.Status = model.BookingStatusCancelled
	booking.CancelledAt = ptrTime(now)
	booking.UpdatedAt = now
	booking.Version++
	r.bookings[key] = booking
	return nil
}

func (r *BookingRepository) CheckIn(ctx context.Context, tenantID, bookingID string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := bookingKey(tenantID, bookingID)
	booking, ok := r.bookings[key]
	if !ok {
		return ErrBookingNotFound
	}
	if booking.Status != model.BookingStatusConfirmed {
		return ErrBookingConflict
	}
	booking.Status = model.BookingStatusCheckedIn
	booking.CheckedInAt = ptrTime(now)
	booking.UpdatedAt = now
	booking.Version++
	r.bookings[key] = booking
	return nil
}

func (r *BookingRepository) Reschedule(ctx context.Context, tenantID, bookingID string, newStart, newEnd time.Time, now time.Time) (model.Booking, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := bookingKey(tenantID, bookingID)
	booking, ok := r.bookings[key]
	if !ok {
		return model.Booking{}, ErrBookingNotFound
	}
	booking.StartAt = newStart
	booking.EndAt = newEnd
	booking.RescheduleCount++
	booking.UpdatedAt = now
	booking.Version++
	r.bookings[key] = booking
	return booking, nil
}

func (r *BookingRepository) ReleaseExpiredHolds(ctx context.Context, now time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	released := 0
	for key, booking := range r.bookings {
		if booking.Status == model.BookingStatusHeld && booking.HoldExpiresAt != nil && !now.Before(*booking.HoldExpiresAt) {
			booking.Status = model.BookingStatusExpired
			booking.UpdatedAt = now
			r.bookings[key] = booking
			released++
		}
	}
	return released, nil
}

func (r *BookingRepository) ListBookings(ctx context.Context, tenantID string) ([]model.Booking, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var out []model.Booking
	for _, booking := range r.bookings {
		if booking.TenantID == tenantID {
			out = append(out, booking)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartAt.Before(out[j].StartAt) })
	return out, nil
}

func (r *BookingRepository) ListRooms(ctx context.Context, tenantID string) ([]model.Room, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var out []model.Room
	for _, room := range r.rooms {
		if room.TenantID == tenantID {
			out = append(out, room)
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.Compare(out[i].ID, out[j].ID) < 0 })
	return out, nil
}

func (r *BookingRepository) ListInstructors(ctx context.Context, tenantID string) ([]model.Instructor, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var out []model.Instructor
	for _, instructor := range r.instructors {
		if instructor.TenantID == tenantID {
			out = append(out, instructor)
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.Compare(out[i].ID, out[j].ID) < 0 })
	return out, nil
}

func (r *BookingRepository) detectConflictsLocked(candidate model.Booking, now time.Time) []model.BookingConflict {
	var conflicts []model.BookingConflict

	room, hasRoom := r.rooms[roomKey(candidate.TenantID, candidate.RoomID)]
	if hasRoom && candidate.Attendees > room.Capacity {
		conflicts = append(conflicts, model.BookingConflict{
			Reason: model.BookingConflictCapacity,
			Detail: fmt.Sprintf("room capacity %d is lower than attendees %d", room.Capacity, candidate.Attendees),
		})
	}

	for _, booking := range r.bookings {
		if booking.TenantID != candidate.TenantID {
			continue
		}
		if booking.Status == model.BookingStatusCancelled || booking.Status == model.BookingStatusExpired {
			continue
		}
		if booking.HoldExpiresAt != nil && now.After(*booking.HoldExpiresAt) {
			continue
		}
		if !overlaps(candidate.StartAt, candidate.EndAt, booking.StartAt, booking.EndAt) {
			continue
		}
		if booking.RoomID == candidate.RoomID {
			conflicts = append(conflicts, model.BookingConflict{
				Reason: model.BookingConflictRoom,
				Detail: "room is already booked in the requested window",
			})
		}
		if booking.InstructorID == candidate.InstructorID {
			conflicts = append(conflicts, model.BookingConflict{
				Reason: model.BookingConflictInstructor,
				Detail: "instructor is already booked in the requested window",
			})
		}
	}

	return dedupeBookingConflicts(conflicts)
}

func dedupeBookingConflicts(conflicts []model.BookingConflict) []model.BookingConflict {
	seen := map[model.BookingConflictReason]bool{}
	var out []model.BookingConflict
	for _, conflict := range conflicts {
		if seen[conflict.Reason] {
			continue
		}
		seen[conflict.Reason] = true
		out = append(out, conflict)
	}
	return out
}
