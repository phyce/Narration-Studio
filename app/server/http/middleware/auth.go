package middleware

import (
	"crypto/subtle"
	"net/http"
	"nstudio/app/config"
	"strings"

	"github.com/labstack/echo/v4"
)

// secureEqual reports whether a and b are equal using a constant-time
// comparison. An empty configured key (b) never matches, so unset keys
// cannot be satisfied by an empty token.
func secureEqual(a, b string) bool {
	if b == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func AuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(context echo.Context) error {
		authSettings := config.GetSettings().Server.Auth

		// SECURITY: When neither Key nor AdminKey is configured, the server
		// runs in an intentional unauthenticated mode so users can operate a
		// local, keyless instance. This fail-open behavior applies ONLY to the
		// non-admin middleware; AdminAuthMiddleware always requires a key.
		if authSettings.Key == "" && authSettings.AdminKey == "" {
			return next(context)
		}

		authHeader := context.Request().Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")

			if secureEqual(token, authSettings.Key) || secureEqual(token, authSettings.AdminKey) {
				return next(context)
			}
		}

		queryAuth := context.QueryParam("auth")
		if secureEqual(queryAuth, authSettings.Key) || secureEqual(queryAuth, authSettings.AdminKey) {
			return next(context)
		}

		return context.JSON(http.StatusUnauthorized, map[string]string{
			"error":   "Unauthorized",
			"message": "Valid authentication required",
		})
	}
}

func AdminAuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(context echo.Context) error {
		authSettings := config.GetSettings().Server.Auth

		// SECURITY: Default-deny. Admin endpoints (e.g. /config, which reads and
		// writes API keys and engine paths) must never be exposed without an
		// explicitly configured admin key.
		if authSettings.AdminKey == "" {
			return context.JSON(http.StatusForbidden, map[string]string{
				"error":   "Forbidden",
				"message": "Admin key not configured",
			})
		}

		authHeader := context.Request().Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if secureEqual(token, authSettings.AdminKey) {
				return next(context)
			}
		}

		queryAuth := context.QueryParam("auth")
		if secureEqual(queryAuth, authSettings.AdminKey) {
			return next(context)
		}

		return context.JSON(http.StatusForbidden, map[string]string{
			"error":   "Forbidden",
			"message": "Admin authentication required",
		})
	}
}
