package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"trainingops/internal/model"
	"trainingops/internal/tenant"
)

type scheduleStore interface {
	CreateClassPeriod(ctx context.Context, period model.ClassPeriod) (*model.ClassPeriod, error)
	ListClassPeriods(ctx context.Context, tenantID string) ([]model.ClassPeriod, error)
	UpdateClassPeriod(ctx context.Context, tenantID string, period model.ClassPeriod) (*model.ClassPeriod, error)
	DeleteClassPeriod(ctx context.Context, tenantID, periodID string) error
	CreateBlackoutDate(ctx context.Context, item model.BlackoutDate) (*model.BlackoutDate, error)
	ListBlackoutDates(ctx context.Context, tenantID string) ([]model.BlackoutDate, error)
	DeleteBlackoutDate(ctx context.Context, tenantID, blackoutID string) error
	CheckScheduleConflicts(ctx context.Context, tenantID string, startAt, endAt time.Time) (model.ScheduleConflict, error)
}

type ScheduleHandler struct {
	Store scheduleStore
}

type classPeriodRequest struct {
	Title     string `json:"title"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Weekday   int    `json:"weekday"`
}

type blackoutRequest struct {
	Date   string `json:"date"`
	Reason string `json:"reason"`
}

type scheduleConflictRequest struct {
	StartAt time.Time `json:"start_at"`
	EndAt   time.Time `json:"end_at"`
}

func (h *ScheduleHandler) CreateClassPeriod(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return jsonError(c, http.StatusUnauthorized, "tenant context required")
	}
	var req classPeriodRequest
	if err := c.Bind(&req); err != nil {
		return jsonError(c, http.StatusBadRequest, "invalid request body")
	}
	if strings.TrimSpace(req.Title) == "" || req.StartTime == "" || req.EndTime == "" || req.Weekday < 0 || req.Weekday > 6 {
		return jsonError(c, http.StatusBadRequest, "title, start_time, end_time and weekday are required")
	}
	period, err := h.Store.CreateClassPeriod(c.Request().Context(), model.ClassPeriod{
		TenantID:  tenantID,
		Title:     req.Title,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Weekday:   req.Weekday,
	})
	if err != nil {
		return jsonError(c, http.StatusBadRequest, "Unable to create class period")
	}
	return c.JSON(http.StatusCreated, period)
}

func (h *ScheduleHandler) ListClassPeriods(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return jsonError(c, http.StatusUnauthorized, "tenant context required")
	}
	items, err := h.Store.ListClassPeriods(c.Request().Context(), tenantID)
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to list class periods")
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *ScheduleHandler) UpdateClassPeriod(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return jsonError(c, http.StatusUnauthorized, "tenant context required")
	}
	var req classPeriodRequest
	if err := c.Bind(&req); err != nil {
		return jsonError(c, http.StatusBadRequest, "invalid request body")
	}
	periodID := strings.TrimSpace(c.Param("id"))
	if periodID == "" {
		return jsonError(c, http.StatusBadRequest, "period id is required")
	}
	updated, err := h.Store.UpdateClassPeriod(c.Request().Context(), tenantID, model.ClassPeriod{
		ID:        periodID,
		TenantID:  tenantID,
		Title:     req.Title,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Weekday:   req.Weekday,
	})
	if err != nil {
		return jsonError(c, http.StatusBadRequest, "Unable to update class period")
	}
	return c.JSON(http.StatusOK, updated)
}

func (h *ScheduleHandler) DeleteClassPeriod(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return jsonError(c, http.StatusUnauthorized, "tenant context required")
	}
	periodID := strings.TrimSpace(c.Param("id"))
	if periodID == "" {
		return jsonError(c, http.StatusBadRequest, "period id is required")
	}
	if err := h.Store.DeleteClassPeriod(c.Request().Context(), tenantID, periodID); err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to delete class period")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *ScheduleHandler) CreateBlackoutDate(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return jsonError(c, http.StatusUnauthorized, "tenant context required")
	}
	var req blackoutRequest
	if err := c.Bind(&req); err != nil {
		return jsonError(c, http.StatusBadRequest, "invalid request body")
	}
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(req.Date))
	if err != nil {
		return jsonError(c, http.StatusBadRequest, "date must be YYYY-MM-DD")
	}
	item, err := h.Store.CreateBlackoutDate(c.Request().Context(), model.BlackoutDate{
		TenantID:     tenantID,
		BlackoutDate: parsed.UTC(),
		Reason:       strings.TrimSpace(req.Reason),
	})
	if err != nil {
		return jsonError(c, http.StatusBadRequest, "Unable to create blackout date")
	}
	return c.JSON(http.StatusCreated, item)
}

func (h *ScheduleHandler) ListBlackoutDates(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return jsonError(c, http.StatusUnauthorized, "tenant context required")
	}
	items, err := h.Store.ListBlackoutDates(c.Request().Context(), tenantID)
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to list blackout dates")
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *ScheduleHandler) DeleteBlackoutDate(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return jsonError(c, http.StatusUnauthorized, "tenant context required")
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return jsonError(c, http.StatusBadRequest, "blackout id is required")
	}
	if err := h.Store.DeleteBlackoutDate(c.Request().Context(), tenantID, id); err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to delete blackout date")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *ScheduleHandler) CheckConflicts(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return jsonError(c, http.StatusUnauthorized, "tenant context required")
	}
	var req scheduleConflictRequest
	if err := c.Bind(&req); err != nil {
		return jsonError(c, http.StatusBadRequest, "invalid request body")
	}
	if req.StartAt.IsZero() || req.EndAt.IsZero() || !req.EndAt.After(req.StartAt) {
		return jsonError(c, http.StatusBadRequest, "start_at and end_at must be valid")
	}
	conflict, err := h.Store.CheckScheduleConflicts(c.Request().Context(), tenantID, req.StartAt.UTC(), req.EndAt.UTC())
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to check schedule conflicts")
	}
	return c.JSON(http.StatusOK, conflict)
}
