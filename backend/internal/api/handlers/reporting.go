package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"trainingops/internal/repository"
	"trainingops/internal/service"
	"trainingops/internal/tenant"
)

type ReportingHandler struct {
	Reports  *service.ReportingService
	Bookings repository.BookingStore
}

func (h *ReportingHandler) BookingsCSV(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return jsonError(c, http.StatusUnauthorized, "tenant context required")
	}
	bookings, err := h.Bookings.ListBookings(c.Request().Context(), tenantID)
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to load bookings")
	}
	report := h.Reports.BookingsCSV(bookings)
	file, err := h.Reports.WriteReport(report)
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to generate report")
	}
	return c.JSON(http.StatusOK, file)
}

func (h *ReportingHandler) CompliancePDF(c echo.Context) error {
	report := h.Reports.CompliancePDF("Compliance export", "TrainingOps Internal", []string{
		"All PII is masked through the API layer.",
		"Booking and task transitions are audited.",
		"Tenant isolation is enforced at the repository boundary.",
	})
	file, err := h.Reports.WriteReport(report)
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to generate report")
	}
	return c.JSON(http.StatusOK, file)
}

func (h *ReportingHandler) Download(c echo.Context) error {
	path, err := h.Reports.Resolve(c.Param("filename"))
	if err != nil {
		return jsonError(c, http.StatusNotFound, "Report not found")
	}
	http.ServeFile(c.ResponseWriter(), c.Request(), path)
	return nil
}
