package handlers

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"trainingops/internal/repository/postgres"
	"trainingops/internal/tenant"
)

type AdminHandler struct {
	Store *postgres.Store
}

type tenantPolicyRequest struct {
	Policies map[string]any `json:"policies"`
}

type updateRoleRequest struct {
	Role string `json:"role"`
}

type roomRequest struct {
	Name     string `json:"name"`
	Capacity int    `json:"capacity"`
}

func (h *AdminHandler) ListTenants(c echo.Context) error {
	items, err := h.Store.ListTenantsAdmin(c.Request().Context())
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to list tenants")
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *AdminHandler) UpdateTenantPolicies(c echo.Context) error {
	tenantID := strings.TrimSpace(c.Param("id"))
	if tenantID == "" {
		return jsonError(c, http.StatusBadRequest, "tenant id is required")
	}
	var req tenantPolicyRequest
	if err := c.Bind(&req); err != nil {
		return jsonError(c, http.StatusBadRequest, "invalid request body")
	}
	if req.Policies == nil {
		req.Policies = map[string]any{}
	}
	if err := h.Store.UpdateTenantPolicies(c.Request().Context(), tenantID, req.Policies); err != nil {
		return jsonError(c, http.StatusBadRequest, "Unable to update tenant policies")
	}
	return c.JSON(http.StatusOK, map[string]any{"status": "updated", "tenant_id": tenantID})
}

func (h *AdminHandler) ListUsers(c echo.Context) error {
	items, err := h.Store.ListUsersAdmin(c.Request().Context())
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to list users")
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *AdminHandler) UpdateUserRole(c echo.Context) error {
	userID := strings.TrimSpace(c.Param("id"))
	if userID == "" {
		return jsonError(c, http.StatusBadRequest, "user id is required")
	}
	var req updateRoleRequest
	if err := c.Bind(&req); err != nil {
		return jsonError(c, http.StatusBadRequest, "invalid request body")
	}
	if strings.TrimSpace(req.Role) == "" {
		return jsonError(c, http.StatusBadRequest, "role is required")
	}
	if err := h.Store.UpdateUserRole(c.Request().Context(), userID, req.Role); err != nil {
		return jsonError(c, http.StatusBadRequest, "Unable to update user role")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "updated"})
}

func (h *AdminHandler) ListPermissions(c echo.Context) error {
	items, err := h.Store.ListPermissions(c.Request().Context())
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to list permissions")
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *AdminHandler) ListRooms(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return jsonError(c, http.StatusUnauthorized, "tenant context required")
	}
	items, err := h.Store.ListRoomsByTenant(c.Request().Context(), tenantID)
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to list rooms")
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *AdminHandler) CreateRoom(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return jsonError(c, http.StatusUnauthorized, "tenant context required")
	}
	var req roomRequest
	if err := c.Bind(&req); err != nil {
		return jsonError(c, http.StatusBadRequest, "invalid request body")
	}
	if strings.TrimSpace(req.Name) == "" || req.Capacity < 1 {
		return jsonError(c, http.StatusBadRequest, "name and capacity are required")
	}
	room, err := h.Store.CreateRoom(c.Request().Context(), tenantID, strings.TrimSpace(req.Name), req.Capacity)
	if err != nil {
		return jsonError(c, http.StatusBadRequest, "Unable to create room")
	}
	return c.JSON(http.StatusCreated, map[string]any{"room": room})
}

func (h *AdminHandler) UpdateRoom(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return jsonError(c, http.StatusUnauthorized, "tenant context required")
	}
	roomID := strings.TrimSpace(c.Param("id"))
	if roomID == "" {
		return jsonError(c, http.StatusBadRequest, "room id is required")
	}
	var req roomRequest
	if err := c.Bind(&req); err != nil {
		return jsonError(c, http.StatusBadRequest, "invalid request body")
	}
	if strings.TrimSpace(req.Name) == "" || req.Capacity < 1 {
		return jsonError(c, http.StatusBadRequest, "name and capacity are required")
	}
	room, err := h.Store.UpdateRoom(c.Request().Context(), tenantID, roomID, strings.TrimSpace(req.Name), req.Capacity)
	if err != nil {
		return jsonError(c, http.StatusBadRequest, "Unable to update room")
	}
	return c.JSON(http.StatusOK, map[string]any{"room": room})
}
