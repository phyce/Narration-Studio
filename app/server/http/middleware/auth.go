package middleware

import (
	"net/http"
	"nstudio/app/config"
	"strings"

	"github.com/labstack/echo/v4"
)

func AuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(context echo.Context) error {
		authSettings := config.GetSettings().Server.Auth

		if authSettings.Key == "" && authSettings.AdminKey == "" {
			return next(context)
		}

		authHeader := context.Request().Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")

			if token == authSettings.Key || token == authSettings.AdminKey {
				return next(context)
			}
		}

		queryAuth := context.QueryParam("auth")
		if queryAuth != "" && (queryAuth == authSettings.Key || queryAuth == authSettings.AdminKey) {
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

		if authSettings.AdminKey == "" {
			return next(context)
			//return context.JSON(http.StatusForbidden, map[string]string{
			//	"error":   "Forbidden",
			//	"message": "Admin key not configured",
			//})
		}

		authHeader := context.Request().Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == authSettings.AdminKey {
				return next(context)
			}
		}

		queryAuth := context.QueryParam("auth")
		if queryAuth == authSettings.AdminKey {
			return next(context)
		}

		return context.JSON(http.StatusForbidden, map[string]string{
			"error":   "Forbidden",
			"message": "Admin authentication required",
		})
	}
}
