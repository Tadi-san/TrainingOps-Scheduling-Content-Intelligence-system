package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"trainingops/internal/service"
	"trainingops/internal/tenant"
)

type IngestionHandler struct {
	Bot *service.IngestionBot
}

func (h *IngestionHandler) Run(c echo.Context) error {
	tenantID, ok := tenant.ID(c.Request().Context())
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "tenant context required"})
	}
	actorUserID := strings.TrimSpace(c.Request().Header.Get("X-Actor-User-ID"))
	var req struct {
		URL  string `json:"url"`
		Body string `json:"body"`
	}
	if err := c.Bind(&req); err != nil {
		req.URL = "http://offline.local/content"
		req.Body = "captcha detected by local proxy pool"
	}
	if strings.TrimSpace(req.URL) == "" {
		req.URL = "http://offline.local/content"
	}
	result, err := h.Bot.RunPersistent(c.Request().Context(), tenantID, actorUserID, req.URL, req.Body, time.Now().UTC())
	if err != nil {
		if err == service.ErrIngestionRateLimited {
			return c.JSON(http.StatusTooManyRequests, map[string]string{"error": "ingestion rate limited"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "ingestion failed"})
	}
	return c.JSON(http.StatusOK, result)
}
