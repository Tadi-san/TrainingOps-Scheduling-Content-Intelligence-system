package handlers

import (
	"strings"

	"github.com/labstack/echo/v4"
)

func actorIdentity(c echo.Context) (userID string, role string) {
	userID = strings.TrimSpace(c.Request().Header.Get("X-Actor-User-ID"))
	role = strings.ToLower(strings.TrimSpace(c.Request().Header.Get("X-Actor-Role")))
	return userID, role
}
