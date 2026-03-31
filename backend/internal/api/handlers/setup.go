package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"trainingops/internal/security"
	"trainingops/internal/service"
)

type SetupHandler struct {
	Setup *service.SetupService
}

type setupTenantRequest struct {
	TenantName    string `json:"tenant_name"`
	TenantSlug    string `json:"tenant_slug"`
	AdminUsername string `json:"admin_username"`
	AdminEmail    string `json:"admin_email"`
	AdminPassword string `json:"admin_password"`
}

func (h *SetupHandler) Status(c echo.Context) error {
	needsSetup, err := h.Setup.NeedsSetup(c.Request().Context())
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to check setup status")
	}
	if needsSetup {
		return c.JSON(http.StatusNotFound, map[string]any{"needs_setup": true})
	}
	return c.JSON(http.StatusOK, map[string]any{"needs_setup": false})
}

func (h *SetupHandler) BootstrapTenant(c echo.Context) error {
	var req setupTenantRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	req.TenantName = strings.TrimSpace(req.TenantName)
	req.TenantSlug = strings.TrimSpace(req.TenantSlug)
	req.AdminUsername = strings.TrimSpace(req.AdminUsername)
	req.AdminEmail = strings.ToLower(strings.TrimSpace(req.AdminEmail))
	if req.TenantName == "" || req.AdminUsername == "" || req.AdminEmail == "" || strings.TrimSpace(req.AdminPassword) == "" {
		return jsonError(c, http.StatusBadRequest, "tenant_name, admin_username, admin_email, and admin_password are required")
	}
	if err := security.ValidatePassword(req.AdminPassword); err != nil {
		return jsonError(c, http.StatusBadRequest, "Admin password does not meet policy requirements")
	}

	tenant, err := h.Setup.BootstrapTenant(
		c.Request().Context(),
		req.TenantName,
		req.TenantSlug,
		req.AdminEmail,
		req.AdminUsername,
		req.AdminPassword,
		time.Now().UTC(),
	)
	if err != nil {
		if err == service.ErrSetupAlreadyCompleted {
			return jsonError(c, http.StatusConflict, "Setup already completed")
		}
		return jsonError(c, http.StatusBadRequest, "Unable to bootstrap tenant")
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"status":       "setup_completed",
		"tenant_id":    tenant.ID,
		"tenant_name":  tenant.Name,
		"tenant_slug":  tenant.Slug,
		"admin_email":  security.MaskEmail(req.AdminEmail),
		"admin_role":   "administrator",
		"next_action":  "login",
		"needs_setup":  false,
		"setup_at_utc": time.Now().UTC().Format(time.RFC3339),
	})
}
