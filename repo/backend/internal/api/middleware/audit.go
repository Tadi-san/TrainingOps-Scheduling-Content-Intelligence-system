package middleware

import (
	"context"
	"time"

	"github.com/labstack/echo/v4"

	"trainingops/internal/audit"
)

type auditContextKey string

const auditKey auditContextKey = "audit"

func AuditContext() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			md := audit.Metadata{
				ActorUserID: c.Request().Header.Get("X-Actor-User-ID"),
				Reason:      c.Request().Header.Get("X-Change-Reason"),
				Who:         c.Request().Header.Get("X-Actor-Name"),
				When:        time.Now().UTC().Format(time.RFC3339),
			}
			ctx := audit.WithMetadata(c.Request().Context(), md)
			c.SetRequest(c.Request().WithContext(context.WithValue(ctx, auditKey, md)))
			return next(c)
		}
	}
}
