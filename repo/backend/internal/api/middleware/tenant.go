package middleware

import (
	"github.com/labstack/echo/v4"

	"trainingops/internal/tenant"
)

func TenantContext() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tenantID := c.Request().Header.Get("X-Tenant-ID")
			if tenantID == "" {
				tenantID = c.QueryParam("tenant_id")
			}

			c.SetRequest(c.Request().WithContext(tenant.WithID(c.Request().Context(), tenantID)))
			return next(c)
		}
	}
}
