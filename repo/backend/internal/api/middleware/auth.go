package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"trainingops/internal/model"
	"trainingops/internal/repository"
	"trainingops/internal/security"
	"trainingops/internal/tenant"
)

func AuthGuard(signingKey string, sessions repository.SessionStore, publicPaths map[string]bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path
			if publicPaths[path] || strings.HasPrefix(path, "/v1/content/share/") {
				return next(c)
			}

			token := bearerToken(c.Request().Header.Get("Authorization"))
			if token == "" {
				if cookie, err := c.Request().Cookie("trainingops_session"); err == nil {
					token = strings.TrimSpace(cookie.Value)
				}
			}
			if token == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
			}

			claims, err := security.VerifySessionToken(signingKey, token, time.Now().UTC())
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid session"})
			}

			now := time.Now().UTC()
			if sessions != nil {
				active, err := sessions.IsActive(c.Request().Context(), claims.TenantID, claims.SessionID, now)
				if err != nil || !active {
					return c.JSON(http.StatusUnauthorized, map[string]string{"error": "session expired or revoked"})
				}
				/*
					if fromCookie {
						nextClaims, rotateErr := rotateSession(c, sessions, claims, now)
						if rotateErr != nil {
							return c.JSON(http.StatusUnauthorized, map[string]string{"error": "session refresh failed"})
						}
						claims = nextClaims
					}
				*/
			}

			request := c.Request()
			request = request.WithContext(tenant.WithID(request.Context(), claims.TenantID))
			request.Header.Set("X-Tenant-ID", claims.TenantID)
			request.Header.Set("X-Actor-User-ID", claims.UserID)
			request.Header.Set("X-Actor-Email", claims.Email)
			request.Header.Set("X-Actor-Role", claims.Role)
			request.Header.Set("X-Session-ID", claims.SessionID)
			c.SetRequest(request)
			setSessionCookie(c, signingKey, claims, now)

			return next(c)
		}
	}
}

func bearerToken(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func setSessionCookie(c echo.Context, signingKey string, claims security.SessionClaims, now time.Time) {
	claims.ExpiresAt = now.Add(24 * time.Hour).Unix()
	token, err := security.SignSessionToken(signingKey, claims)
	if err != nil {
		return
	}
	http.SetCookie(c.ResponseWriter(), &http.Cookie{
		Name:     "trainingops_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   useSecureCookie(c),
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(claims.ExpiresAt, 0).UTC(),
	})
}

func useSecureCookie(c echo.Context) bool {
	if c.Request().TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(c.Request().Header.Get("X-Forwarded-Proto")), "https")
}

func rotateSession(c echo.Context, sessions repository.SessionStore, current security.SessionClaims, now time.Time) (security.SessionClaims, error) {
	nextSessionID := security.GenerateUUID()
	next := model.Session{
		ID:               nextSessionID,
		TenantID:         current.TenantID,
		UserID:           current.UserID,
		RefreshTokenHash: fmt.Sprintf("rt_%d", now.UnixNano()),
		ExpiresAt:        now.Add(24 * time.Hour),
		LastUsedAt:       now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := sessions.Rotate(c.Request().Context(), current.TenantID, current.SessionID, next, now); err != nil {
		return security.SessionClaims{}, err
	}
	current.SessionID = nextSessionID
	current.ExpiresAt = next.ExpiresAt.Unix()
	return current, nil
}
