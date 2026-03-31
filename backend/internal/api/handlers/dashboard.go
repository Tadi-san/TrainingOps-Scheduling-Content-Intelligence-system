package handlers

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"trainingops/internal/service"
	"trainingops/internal/tenant"
)

type DashboardHandler struct {
	Dashboard *service.DashboardService
}

func (h *DashboardHandler) Get(c echo.Context) error {
	role := c.Param("role")
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok || tenantID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "tenant context is required"})
	}
	actorRole := c.Request().Header.Get("X-Actor-Role")
	if actorRole == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "actor role claim missing"})
	}
	if actorRole != "admin" && actorRole != role {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "role does not have access to this workspace"})
	}
	data, err := h.Dashboard.Build(c.Request().Context(), tenantID, role, time.Now().UTC())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, data)
}
