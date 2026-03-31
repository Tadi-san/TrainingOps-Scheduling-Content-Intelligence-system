package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"trainingops/internal/model"
	"trainingops/internal/security"
	"trainingops/internal/service"
	"trainingops/internal/tenant"
)

type AuthHandler struct {
	Auth       *service.AuthService
	Setup      *service.SetupService
	TokenKey   string
	TokenTTL   time.Duration
	TokenClock func() time.Time
}

type registerRequest struct {
	TenantID    string `json:"tenant_id"`
	TenantName  string `json:"tenant_name"`
	TenantSlug  string `json:"tenant_slug"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
	Role        string `json:"role"`
}

type loginRequest struct {
	TenantID string `json:"tenant_id"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

const defaultTenantID = "tenant-1"

func (h *AuthHandler) Register(c echo.Context) error {
	var req registerRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	req.TenantID = strings.TrimSpace(req.TenantID)
	req.TenantName = strings.TrimSpace(req.TenantName)
	req.TenantSlug = strings.TrimSpace(req.TenantSlug)
	if req.TenantID == "" {
		if tenantID, ok := tenant.ID(c.Request().Context()); ok {
			req.TenantID = tenantID
		}
	}
	if req.TenantID == "" && h.Setup != nil {
		needsSetup, err := h.Setup.NeedsSetup(c.Request().Context())
		if err == nil && needsSetup {
			if req.TenantName == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "tenant_name is required for first user registration"})
			}
			createdTenant, createErr := h.Setup.Tenants.CreateTenant(c.Request().Context(), req.TenantName, req.TenantSlug, time.Now().UTC())
			if createErr != nil {
				return jsonError(c, http.StatusBadRequest, "Unable to create tenant for first user")
			}
			req.TenantID = createdTenant.ID
			req.Role = "admin"
		}
	}
	if req.TenantID == "" {
		req.TenantID = defaultTenantID
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Role = normalizeRole(req.Role)

	if req.TenantID == "" || req.Email == "" || req.Password == "" || req.DisplayName == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "tenant_id, email, display_name, and password are required"})
	}

	user := model.User{
		ID:          newUUIDString(),
		TenantID:    req.TenantID,
		Email:       req.Email,
		DisplayName: req.DisplayName,
		Role:        req.Role,
	}

	if err := h.Auth.Register(c.Request().Context(), user, req.Password, time.Now().UTC()); err != nil {
		return jsonError(c, http.StatusBadRequest, "Unable to register user")
	}

	return c.JSON(http.StatusCreated, map[string]string{
		"user_id":   user.ID,
		"tenant_id": req.TenantID,
		"email":     security.MaskEmail(req.Email),
		"status":    "registered",
	})
}

func (h *AuthHandler) Login(c echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	req.TenantID = strings.TrimSpace(req.TenantID)
	if req.TenantID == "" {
		if tenantID, ok := tenant.ID(c.Request().Context()); ok {
			req.TenantID = tenantID
		}
	}
	if req.TenantID == "" {
		req.TenantID = defaultTenantID
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if req.TenantID == "" || req.Email == "" || req.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "tenant_id, email, and password are required"})
	}

	now := h.now()
	user, err := h.Auth.Authenticate(c.Request().Context(), req.TenantID, req.Email, req.Password, now)
	if err != nil {
		status := http.StatusUnauthorized
		if err == service.ErrAccountLocked {
			status = http.StatusLocked
			return jsonError(c, status, "Account is temporarily locked")
		}
		return jsonError(c, status, "Invalid credentials")
	}

	session, err := h.Auth.CreateSession(c.Request().Context(), *user, now)
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "Unable to start session")
	}
	token, err := security.SignSessionToken(h.TokenKey, security.SessionClaims{
		UserID:    user.ID,
		TenantID:  user.TenantID,
		Email:     user.Email,
		Role:      normalizeRole(user.Role),
		SessionID: session.ID,
		ExpiresAt: session.ExpiresAt.Unix(),
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "unable to issue token"})
	}
	http.SetCookie(c.ResponseWriter(), &http.Cookie{
		Name:     "trainingops_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   shouldUseSecureCookie(c),
		SameSite: http.SameSiteStrictMode,
		Expires:  session.ExpiresAt.UTC(),
	})

	return c.JSON(http.StatusOK, map[string]any{
		"user_id":      user.ID,
		"tenant_id":    user.TenantID,
		"email":        security.MaskEmail(user.Email),
		"display_name": user.DisplayName,
		"role":         normalizeRole(user.Role),
		"status":       "authenticated",
		"access_token": token,
		"token_type":   "Bearer",
		"expires_at":   session.ExpiresAt.UTC().Format(time.RFC3339),
		"session_id":   session.ID,
	})
}

func (h *AuthHandler) Logout(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return jsonError(c, http.StatusUnauthorized, "tenant context required")
	}
	sessionID := c.Request().Header.Get("X-Session-ID")
	if sessionID != "" && h.Auth.Sessions != nil {
		_ = h.Auth.Sessions.Revoke(c.Request().Context(), tenantID, sessionID, time.Now().UTC())
	}
	http.SetCookie(c.ResponseWriter(), &http.Cookie{
		Name:     "trainingops_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   shouldUseSecureCookie(c),
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(0, 0).UTC(),
	})
	return c.JSON(http.StatusOK, map[string]string{"status": "revoked"})
}

func (h *AuthHandler) Session(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return jsonError(c, http.StatusUnauthorized, "tenant context required")
	}
	userID := strings.TrimSpace(c.Request().Header.Get("X-Actor-User-ID"))
	email := strings.TrimSpace(c.Request().Header.Get("X-Actor-Email"))
	role := normalizeRole(c.Request().Header.Get("X-Actor-Role"))
	sessionID := strings.TrimSpace(c.Request().Header.Get("X-Session-ID"))
	if userID == "" || email == "" || sessionID == "" {
		return jsonError(c, http.StatusUnauthorized, "session not found")
	}
	return c.JSON(http.StatusOK, map[string]any{
		"user_id":      userID,
		"tenant_id":    tenantID,
		"email":        security.MaskEmail(email),
		"display_name": "",
		"role":         role,
		"status":       "authenticated",
		"session_id":   sessionID,
		"expires_at":   time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339),
	})
}

func normalizeRole(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "administrator":
		return "admin"
	case "admin", "coordinator", "instructor", "learner":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "learner"
	}
}

func (h *AuthHandler) now() time.Time {
	if h.TokenClock != nil {
		return h.TokenClock().UTC()
	}
	return time.Now().UTC()
}

func shouldUseSecureCookie(c echo.Context) bool {
	if c.Request().TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(c.Request().Header.Get("X-Forwarded-Proto")), "https")
}
