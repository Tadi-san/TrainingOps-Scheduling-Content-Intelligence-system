package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

var ErrForbiddenTenant = errors.New("forbidden resource access")

func RequireRole(roles ...string) echo.MiddlewareFunc {
	allowed := map[string]bool{}
	for _, role := range roles {
		allowed[strings.ToLower(strings.TrimSpace(role))] = true
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role := strings.ToLower(strings.TrimSpace(c.Request().Header.Get("X-Actor-Role")))
			if role == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing role claim"})
			}
			if role == "admin" {
				return next(c)
			}
			if !allowed[role] {
				return c.JSON(http.StatusForbidden, map[string]string{"error": "insufficient role privileges"})
			}
			return next(c)
		}
	}
}

func VerifyResourceAccess(resourceTenantID, claimsTenantID string) error {
	if resourceTenantID == "" || claimsTenantID == "" {
		return ErrForbiddenTenant
	}
	if resourceTenantID != claimsTenantID {
		return ErrForbiddenTenant
	}
	return nil
}
