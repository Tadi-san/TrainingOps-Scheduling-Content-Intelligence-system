package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

func StructuredLogging(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			capture := &statusCaptureWriter{ResponseWriter: c.ResponseWriter(), status: http.StatusOK}
			c.SetResponseWriter(capture)
			err := next(c)
			duration := time.Since(start)
			status := capture.status
			if err != nil && status == http.StatusOK {
				status = http.StatusInternalServerError
			}
			logger.Info("http_request",
				"method", c.Request().Method,
				"path", c.Request().URL.Path,
				"status", status,
				"duration_ms", duration.Milliseconds(),
				"actor_user_id", c.Request().Header.Get("X-Actor-User-ID"),
				"tenant_id", c.Request().Header.Get("X-Tenant-ID"),
			)
			return err
		}
	}
}

type statusCaptureWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusCaptureWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
