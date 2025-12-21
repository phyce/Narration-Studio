package http

import (
	"net/http"
	"nstudio/app/server/http/responses"

	"nstudio/app/tts/profile"

	"github.com/labstack/echo/v4"
)

func handleListProfiles(context echo.Context) error {
	manager := profile.GetManager()

	profiles, err := manager.GetAllProfiles()
	if err != nil {
		return context.JSON(http.StatusInternalServerError, responses.ErrorResponse{
			Success: false,
			Error:   "Failed to list profiles: " + err.Error(),
			Code:    500,
		})
	}

	return context.JSON(http.StatusOK, map[string]interface{}{
		"profiles": profiles,
		"count":    len(profiles),
	})
}

func handleGetProfile(context echo.Context) error {
	profileID := context.Param("profileId")

	if profileID == "" {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Profile ID is required",
			Code:    400,
		})
	}

	manager := profile.GetManager()
	requestedProfile, err := manager.GetProfile(profileID)
	if err != nil {
		return context.JSON(http.StatusNotFound, responses.ErrorResponse{
			Success: false,
			Error:   "Profile not found: " + err.Error(),
			Code:    404,
		})
	}

	return context.JSON(http.StatusOK, requestedProfile)
}

func handleCreateProfile(context echo.Context) error {
	var request ProfileCreateRequest

	if err := context.Bind(&request); err != nil {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Invalid request format",
			Code:    400,
		})
	}

	if request.ID == "" {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Profile ID is required",
			Code:    400,
		})
	}

	manager := profile.GetManager()
	newProfile, err := manager.CreateProfile(request.ID, request.Name, request.Description)
	if err != nil {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Failed to create profile: " + err.Error(),
			Code:    400,
		})
	}

	return context.JSON(http.StatusCreated, map[string]interface{}{
		"success": true,
		"profile": newProfile,
	})
}

func handleDeleteProfile(context echo.Context) error {
	profileID := context.Param("profileId")

	if profileID == "" {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Profile ID is required",
			Code:    400,
		})
	}

	manager := profile.GetManager()
	if err := manager.DeleteProfile(profileID); err != nil {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Failed to delete profile: " + err.Error(),
			Code:    400,
		})
	}

	return context.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Profile deleted successfully",
	})
}
