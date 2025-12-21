package profiles

import (
	"net/http"
	"nstudio/app/server/http/responses"
	"nstudio/app/tts/profile"

	"github.com/labstack/echo/v4"
)

func GetVoices(context echo.Context) error {
	profileID := context.Param("profileId")

	if profileID == "" {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Profile ID is required",
			Code:    400,
		})
	}

	manager := profile.GetManager()
	prof, err := manager.GetProfile(profileID)
	if err != nil {
		return context.JSON(http.StatusNotFound, responses.ErrorResponse{
			Success: false,
			Error:   "Profile not found: " + err.Error(),
			Code:    404,
		})
	}

	return context.JSON(http.StatusOK, map[string]interface{}{
		"profile":    profileID,
		"voices":     prof.Voices,
		"characters": prof.GetCharacters(),
		"count":      len(prof.Voices),
	})
}

func GetCharacterVoice(context echo.Context) error {
	profileID := context.Param("profileId")
	character := context.Param("character")

	if profileID == "" || character == "" {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Profile ID and character name are required",
			Code:    400,
		})
	}

	manager := profile.GetManager()
	voice, err := manager.GetVoiceConfig(profileID, character)
	if err != nil {
		return context.JSON(http.StatusNotFound, responses.ErrorResponse{
			Success: false,
			Error:   err.Error(),
			Code:    404,
		})
	}

	return context.JSON(http.StatusOK, map[string]interface{}{
		"profile":   profileID,
		"character": character,
		"voice":     voice,
	})
}

func SetCharacterVoice(context echo.Context) error {
	profileID := context.Param("profileId")
	character := context.Param("character")

	if profileID == "" || character == "" {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Profile ID and character name are required",
			Code:    400,
		})
	}

	var voiceConfig map[string]interface{}
	if err := context.Bind(&voiceConfig); err != nil {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Invalid voice configuration",
			Code:    400,
		})
	}

	manager := profile.GetManager()

	voice, err := manager.GetOrAllocateVoice(profileID, character)
	if err != nil {
		return context.JSON(http.StatusInternalServerError, responses.ErrorResponse{
			Success: false,
			Error:   "Failed to set voice: " + err.Error(),
			Code:    500,
		})
	}

	return context.JSON(http.StatusOK, map[string]interface{}{
		"success":   true,
		"profile":   profileID,
		"character": character,
		"voice":     voice,
	})
}

func DeleteCharacterVoice(context echo.Context) error {
	profileID := context.Param("profileId")
	character := context.Param("character")

	if profileID == "" || character == "" {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Profile ID and character name are required",
			Code:    400,
		})
	}

	manager := profile.GetManager()
	if err := manager.RemoveVoiceConfig(profileID, character); err != nil {
		return context.JSON(http.StatusNotFound, responses.ErrorResponse{
			Success: false,
			Error:   err.Error(),
			Code:    404,
		})
	}

	return context.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Character voice removed successfully",
	})
}
