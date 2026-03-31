package handlers

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"trainingops/internal/model"
	"trainingops/internal/repository"
	"trainingops/internal/service"
	"trainingops/internal/tenant"
)

type AnalyticsHandler struct {
	Engine   *service.AnalyticsEngine
	Bookings repository.BookingStore
}

func (h *AnalyticsHandler) Cohorts(c echo.Context) error {
	events := h.eventsForTenant(c)
	return c.JSON(http.StatusOK, map[string]any{
		"cohorts":  h.Engine.ComputeCohorts(events, time.Now().UTC()),
		"features": h.Engine.BuildFeatureStore(h.Engine.ComputeCohorts(events, time.Now().UTC())),
	})
}

func (h *AnalyticsHandler) Anomalies(c echo.Context) error {
	events := h.eventsForTenant(c)
	return c.JSON(http.StatusOK, map[string]any{
		"anomalies": h.Engine.DetectAnomalies(events, time.Now().UTC()),
	})
}

func (h *AnalyticsHandler) eventsForTenant(c echo.Context) []model.ObservabilityEvent {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok || h.Bookings == nil {
		return []model.ObservabilityEvent{}
	}
	bookings, err := h.Bookings.ListBookings(c.Request().Context(), tenantID)
	if err != nil {
		return []model.ObservabilityEvent{}
	}
	events := make([]model.ObservabilityEvent, 0, len(bookings))
	for _, booking := range bookings {
		level := "info"
		detail := "booking succeeded"
		if booking.Status == model.BookingStatusCancelled || booking.Status == model.BookingStatusExpired {
			level = "error"
			detail = "booking failed"
		}
		events = append(events, model.ObservabilityEvent{
			ID:        booking.ID,
			TenantID:  booking.TenantID,
			Type:      "booking",
			Level:     level,
			Detail:    detail,
			CreatedAt: booking.UpdatedAt,
		})
	}
	return events
}
