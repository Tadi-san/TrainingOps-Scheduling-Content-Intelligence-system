package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"trainingops/internal/audit"
	"trainingops/internal/model"
	"trainingops/internal/service"
	"trainingops/internal/tenant"
)

type BookingHandler struct {
	Bookings *service.BookingService
	Audit    *service.AuditRecorder
}

type bookingRequest struct {
	BookingID    string `json:"booking_id"`
	RoomID       string `json:"room_id"`
	InstructorID string `json:"instructor_id"`
	Title        string `json:"title"`
	StartAt      string `json:"start_at"`
	EndAt        string `json:"end_at"`
	Capacity     int    `json:"capacity"`
	Attendees    int    `json:"attendees"`
	Why          string `json:"why"`
}

type rescheduleRequest struct {
	BookingID string `json:"booking_id"`
	NewStart  string `json:"new_start"`
	NewEnd    string `json:"new_end"`
	Why       string `json:"why"`
}

func (h *BookingHandler) Hold(c echo.Context) error {
	var req bookingRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	startAt, err := parseFlexTime(req.StartAt)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid start_at format. use YYYY-MM-DDTHH:MM or RFC3339"})
	}
	endAt, err := parseFlexTime(req.EndAt)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid end_at format. use YYYY-MM-DDTHH:MM or RFC3339"})
	}

	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok || req.RoomID == "" || req.InstructorID == "" || req.Title == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "room_id, instructor_id, and title are required"})
	}
	actorUserID, _ := actorIdentity(c)
	booking, conflicts, alternatives, err := h.Bookings.Hold(c.Request().Context(), model.Booking{
		TenantID:     tenantID,
		UserID:       actorUserID,
		RoomID:       req.RoomID,
		InstructorID: req.InstructorID,
		Title:        req.Title,
		StartAt:      startAt,
		EndAt:        endAt,
		Capacity:     req.Capacity,
		Attendees:    req.Attendees,
	}, time.Now().UTC())
	md, _ := audit.FromContext(c.Request().Context())
	if strings.TrimSpace(md.Reason) == "" {
		md.Reason = strings.TrimSpace(req.Why)
	}
	if err != nil {
		h.Audit.RecordTransition(c.Request().Context(), "booking_hold_failed", "booking", req.BookingID, tenantID, "", "", md, time.Now().UTC())
		return c.JSON(http.StatusConflict, map[string]any{
			"error":        "Booking could not be placed due to conflicts",
			"conflicts":    conflicts,
			"alternatives": alternatives,
		})
	}
	h.Audit.RecordTransition(c.Request().Context(), "booking_held", "booking", booking.ID, tenantID, "", string(booking.Status), md, time.Now().UTC())
	return c.JSON(http.StatusCreated, map[string]any{
		"booking": booking,
	})
}

func (h *BookingHandler) Confirm(c echo.Context) error {
	var req bookingRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "tenant context required"})
	}
	if strings.TrimSpace(req.BookingID) == "" {
		return jsonError(c, http.StatusBadRequest, "booking_id is required")
	}
	resource, err := h.Bookings.Store.GetBooking(c.Request().Context(), tenantID, req.BookingID)
	if err != nil {
		return jsonError(c, http.StatusNotFound, "Booking not found")
	}
	if err := authorizeBookingAccess(c, resource.TenantID, resource.UserID); err != nil {
		return jsonError(c, http.StatusForbidden, "Access denied")
	}
	if err := h.Bookings.Confirm(c.Request().Context(), tenantID, req.BookingID, time.Now().UTC()); err != nil {
		return jsonError(c, http.StatusConflict, "Unable to confirm booking")
	}
	md, _ := audit.FromContext(c.Request().Context())
	if strings.TrimSpace(md.Reason) == "" {
		md.Reason = strings.TrimSpace(req.Why)
	}
	h.Audit.RecordTransition(c.Request().Context(), "booking_confirmed", "booking", req.BookingID, tenantID, string(resource.Status), string(model.BookingStatusConfirmed), md, time.Now().UTC())
	return c.JSON(http.StatusOK, map[string]string{"status": "confirmed"})
}

func (h *BookingHandler) Cancel(c echo.Context) error {
	var req bookingRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "tenant context required"})
	}
	if strings.TrimSpace(req.BookingID) == "" {
		return jsonError(c, http.StatusBadRequest, "booking_id is required")
	}
	resource, err := h.Bookings.Store.GetBooking(c.Request().Context(), tenantID, req.BookingID)
	if err != nil {
		return jsonError(c, http.StatusNotFound, "Booking not found")
	}
	if err := authorizeBookingAccess(c, resource.TenantID, resource.UserID); err != nil {
		return jsonError(c, http.StatusForbidden, "Access denied")
	}
	if err := h.Bookings.Cancel(c.Request().Context(), tenantID, req.BookingID, time.Now().UTC()); err != nil {
		return jsonError(c, http.StatusConflict, "Unable to cancel booking")
	}
	md, _ := audit.FromContext(c.Request().Context())
	if strings.TrimSpace(md.Reason) == "" {
		md.Reason = strings.TrimSpace(req.Why)
	}
	h.Audit.RecordTransition(c.Request().Context(), "booking_cancelled", "booking", req.BookingID, tenantID, string(resource.Status), string(model.BookingStatusCancelled), md, time.Now().UTC())
	return c.JSON(http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *BookingHandler) Reschedule(c echo.Context) error {
	var req rescheduleRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	newStart, err := parseFlexTime(req.NewStart)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid new_start format"})
	}
	newEnd, err := parseFlexTime(req.NewEnd)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid new_end format"})
	}

	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "tenant context required"})
	}
	if strings.TrimSpace(req.BookingID) == "" || newStart.IsZero() || newEnd.IsZero() || !newEnd.After(newStart) {
		return jsonError(c, http.StatusBadRequest, "booking_id and valid new_start/new_end are required")
	}
	resource, err := h.Bookings.Store.GetBooking(c.Request().Context(), tenantID, req.BookingID)
	if err != nil {
		return jsonError(c, http.StatusNotFound, "Booking not found")
	}
	if err := authorizeBookingAccess(c, resource.TenantID, resource.UserID); err != nil {
		return jsonError(c, http.StatusForbidden, "Access denied")
	}
	booking, err := h.Bookings.Reschedule(c.Request().Context(), tenantID, req.BookingID, newStart, newEnd, time.Now().UTC())
	if err != nil {
		return jsonError(c, http.StatusConflict, "Unable to reschedule booking")
	}
	md, _ := audit.FromContext(c.Request().Context())
	if strings.TrimSpace(md.Reason) == "" {
		md.Reason = strings.TrimSpace(req.Why)
	}
	h.Audit.RecordTransition(c.Request().Context(), "booking_rescheduled", "booking", req.BookingID, tenantID, string(resource.Status), string(booking.Status), md, time.Now().UTC())
	return c.JSON(http.StatusOK, map[string]any{"booking": booking})
}

func (h *BookingHandler) CheckIn(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "tenant context required"})
	}
	bookingID := strings.TrimSpace(c.Param("id"))
	if bookingID == "" {
		return jsonError(c, http.StatusBadRequest, "booking id is required")
	}
	resource, err := h.Bookings.Store.GetBooking(c.Request().Context(), tenantID, bookingID)
	if err != nil {
		return jsonError(c, http.StatusNotFound, "Booking not found")
	}
	if err := authorizeBookingAccess(c, resource.TenantID, resource.UserID); err != nil {
		return jsonError(c, http.StatusForbidden, "Access denied")
	}
	if err := h.Bookings.CheckIn(c.Request().Context(), tenantID, bookingID, time.Now().UTC()); err != nil {
		return jsonError(c, http.StatusConflict, "Only confirmed bookings can be checked in")
	}
	md, _ := audit.FromContext(c.Request().Context())
	h.Audit.RecordTransition(c.Request().Context(), "booking_checked_in", "booking", bookingID, tenantID, string(resource.Status), string(model.BookingStatusCheckedIn), md, time.Now().UTC())
	return c.JSON(http.StatusOK, map[string]string{"status": "checked_in"})
}

func (h *BookingHandler) ListRooms(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "tenant context required"})
	}
	rooms, err := h.Bookings.Store.ListRooms(c.Request().Context(), tenantID)
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to list rooms")
	}
	return c.JSON(http.StatusOK, map[string]any{"items": rooms})
}

func (h *BookingHandler) ListInstructors(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "tenant context required"})
	}
	instructors, err := h.Bookings.Store.ListInstructors(c.Request().Context(), tenantID)
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to list instructors")
	}
	return c.JSON(http.StatusOK, map[string]any{"items": instructors})
}

func authorizeBookingAccess(c echo.Context, resourceTenantID, resourceUserID string) error {
	claimsTenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return errors.New("missing tenant claims")
	}
	if claimsTenantID != resourceTenantID {
		return errors.New("tenant mismatch")
	}
	actorUserID, actorRole := actorIdentity(c)
	if actorRole == "admin" {
		return nil
	}
	if resourceUserID == "" || actorUserID == "" || resourceUserID != actorUserID {
		return errors.New("booking ownership mismatch")
	}
	return nil
}

func parseFlexTime(val string) (time.Time, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return time.Time{}, nil
	}
	// Try RFC3339
	if t, err := time.Parse(time.RFC3339, val); err == nil {
		return t, nil
	}
	// Try datetime-local format from browser: 2006-01-02T15:04
	if t, err := time.Parse("2006-01-02T15:04", val); err == nil {
		return t, nil
	}
	// Try format with seconds: 2006-01-02T15:04:05
	if t, err := time.Parse("2006-01-02T15:04:05", val); err == nil {
		return t, nil
	}
	// Try YYYY-MM-DD
	if t, err := time.Parse("2006-01-02", val); err == nil {
		return t, nil
	}
	return time.Time{}, errors.New("invalid time format")
}
