package handlers

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"trainingops/internal/audit"
	"trainingops/internal/repository/postgres"
	"trainingops/internal/service"
	"trainingops/internal/tenant"
)

type LearnerHandler struct {
	Store *postgres.Store
	Audit *service.AuditRecorder
}

type reserveRequest struct {
	BookingID string `json:"booking_id"`
	Why       string `json:"why"`
}

func (h *LearnerHandler) Catalog(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return jsonError(c, http.StatusUnauthorized, "tenant context required")
	}
	filter := postgres.LearnerCatalogFilter{
		RoomID:       strings.TrimSpace(c.QueryParam("room_id")),
		InstructorID: strings.TrimSpace(c.QueryParam("instructor_id")),
	}
	if from := strings.TrimSpace(c.QueryParam("from")); from != "" {
		if parsed, err := time.Parse(time.RFC3339, from); err == nil {
			filter.From = &parsed
		}
	}
	if to := strings.TrimSpace(c.QueryParam("to")); to != "" {
		if parsed, err := time.Parse(time.RFC3339, to); err == nil {
			filter.To = &parsed
		}
	}
	items, err := h.Store.ListLearnerCatalog(c.Request().Context(), tenantID, filter)
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to load learner catalog")
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *LearnerHandler) Reserve(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return jsonError(c, http.StatusUnauthorized, "tenant context required")
	}
	actorUserID, _ := actorIdentity(c)
	var req reserveRequest
	if err := c.Bind(&req); err != nil {
		return jsonError(c, http.StatusBadRequest, "invalid request body")
	}
	req.BookingID = strings.TrimSpace(req.BookingID)
	if req.BookingID == "" {
		return jsonError(c, http.StatusBadRequest, "booking_id is required")
	}
	reservation, err := h.Store.ReserveLearnerSeat(c.Request().Context(), tenantID, req.BookingID, actorUserID, time.Now().UTC())
	if err != nil {
		return jsonError(c, http.StatusConflict, "Unable to reserve seat")
	}
	md, _ := audit.FromContext(c.Request().Context())
	if strings.TrimSpace(md.Reason) == "" {
		md.Reason = strings.TrimSpace(req.Why)
	}
	h.Audit.RecordTransition(c.Request().Context(), "learner_reserved", "reservation", reservation.ID, tenantID, "", reservation.Status, md, time.Now().UTC())
	return c.JSON(http.StatusCreated, map[string]any{"reservation": reservation})
}

func (h *LearnerHandler) MyReservations(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return jsonError(c, http.StatusUnauthorized, "tenant context required")
	}
	actorUserID, _ := actorIdentity(c)
	items, err := h.Store.ListLearnerReservations(c.Request().Context(), tenantID, actorUserID)
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to load reservations")
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *LearnerHandler) DownloadApproved(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return jsonError(c, http.StatusUnauthorized, "tenant context required")
	}
	fileID := strings.TrimSpace(c.Param("file_id"))
	if fileID == "" {
		return jsonError(c, http.StatusBadRequest, "file_id is required")
	}
	file, err := h.Store.GetApprovedFile(c.Request().Context(), tenantID, fileID)
	if err != nil {
		return jsonError(c, http.StatusNotFound, "Approved file not found")
	}
	content, err := os.ReadFile(file.FilePath)
	if err != nil {
		return jsonError(c, http.StatusNotFound, "File not found on disk")
	}
	actorUserID, _ := actorIdentity(c)
	downloaderEmail := strings.TrimSpace(c.Request().Header.Get("X-Actor-Email"))
	if downloaderEmail == "" {
		downloaderEmail = actorUserID
	}
	stamp := fmt.Sprintf("Confidential | downloader=%s | timestamp=%s", downloaderEmail, time.Now().UTC().Format(time.RFC3339))
	c.ResponseWriter().Header().Set("X-Watermark", stamp)
	c.ResponseWriter().Header().Set("X-Content-Type-Options", "nosniff")

	if strings.HasPrefix(file.MimeType, "text/") {
		content = append([]byte(stamp+"\n"), content...)
	} else {
		content = append(content, []byte("\n"+stamp)...)
	}
	md, _ := audit.FromContext(c.Request().Context())
	md.Reason = "approved_file_download"
	h.Audit.RecordTransition(c.Request().Context(), "learner_downloaded_file", "file", file.ID, tenantID, "", "downloaded", md, time.Now().UTC())
	c.ResponseWriter().Header().Set("Content-Type", file.MimeType)
	c.ResponseWriter().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", file.FileName))
	c.ResponseWriter().WriteHeader(http.StatusOK)
	_, _ = c.ResponseWriter().Write(content)
	return nil
}
