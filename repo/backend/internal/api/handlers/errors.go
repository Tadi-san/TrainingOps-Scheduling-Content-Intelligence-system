package handlers

import "github.com/labstack/echo/v4"

func jsonError(c echo.Context, status int, message string) error {
	return c.JSON(status, map[string]string{"error": message})
}
