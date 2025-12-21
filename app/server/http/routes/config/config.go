package server

import (
	"net/http"
	"nstudio/app/config"
	"nstudio/app/server/http/responses"

	"github.com/labstack/echo/v4"
)

func Get(context echo.Context) error {
	config := config.Get()

	return context.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"config":  config,
	})
}

func Update(context echo.Context) error {
	var newConfig config.Base

	if err := context.Bind(&newConfig); err != nil {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Invalid configuration format",
			Code:    400,
		})
	}

	currentConfig := config.Get()
	newConfig.Info = currentConfig.Info

	if err := config.Set(newConfig); err != nil {
		return context.JSON(http.StatusInternalServerError, responses.ErrorResponse{
			Success: false,
			Error:   "Failed to save configuration: " + err.Error(),
			Code:    500,
		})
	}

	return context.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Configuration updated successfully",
		"config":  config.Get(),
	})
}

func Patch(context echo.Context) error {
	var request struct {
		Path  string `json:"path" validate:"required"`
		Value string `json:"value"`
	}

	if err := context.Bind(&request); err != nil {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Invalid request format",
			Code:    400,
		})
	}

	if request.Path == "" {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Path is required",
			Code:    400,
		})
	}

	if err := config.SetValueToPath(request.Path, request.Value); err != nil {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Failed to set config value: " + err.Error(),
			Code:    400,
		})
	}

	return context.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Config value updated successfully",
		"path":    request.Path,
		"value":   request.Value,
	})
}

func GetValue(context echo.Context) error {
	path := context.QueryParam("path")

	if path == "" {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Path query parameter is required",
			Code:    400,
		})
	}

	value, err := config.GetValueFromPath(path)
	if err != nil {
		return context.JSON(http.StatusNotFound, responses.ErrorResponse{
			Success: false,
			Error:   "Config path not found: " + err.Error(),
			Code:    404,
		})
	}

	return context.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"path":    path,
		"value":   value,
	})
}

func GetSchema(context echo.Context) error {
	schema, err := config.GetConfigSchema()
	if err != nil {
		return context.JSON(http.StatusInternalServerError, responses.ErrorResponse{
			Success: false,
			Error:   "Failed to get config schema: " + err.Error(),
			Code:    500,
		})
	}

	return context.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"schema":  schema,
	})
}
