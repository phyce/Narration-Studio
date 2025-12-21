package http

import (
	"fmt"
	"net/http"
	"nstudio/app/common/response"
	"nstudio/app/server/http/responses"
	"nstudio/app/server/stats"
	"strings"

	"nstudio/app/cache"
	"nstudio/app/common/audio"
	"nstudio/app/common/util"
	"nstudio/app/tts"
	"nstudio/app/tts/profile"

	"github.com/charmbracelet/log"
	"github.com/labstack/echo/v4"
)

func handleProfileTTSRequest(context echo.Context) error {
	var request ProfileTTSRequest

	if err := context.Bind(&request); err != nil {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Invalid request format",
			Code:    400,
		})
	}

	if request.Profile == "" {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Profile field is required",
			Code:    400,
		})
	}

	if request.Character == "" {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Character field is required",
			Code:    400,
		})
	}

	if strings.TrimSpace(request.Text) == "" {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Text field is required",
			Code:    400,
		})
	}

	if len(request.Text) > 10000 {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Text too long (max 10000 characters)",
			Code:    400,
		})
	}

	format := "wav"
	//if request.Options != nil {
	//	if formatOption, exists := request.Options["format"]; exists {
	//		if formatStr, ok := formatOption.(string); ok {
	//			format = strings.ToLower(formatStr)
	//		}
	//	}
	//}

	//validFormats := []string{"wav", "flac", "ogg"}
	//formatValid := false
	//for _, validFormat := range validFormats {
	//	if format == validFormat {
	//		formatValid = true
	//		break
	//	}
	//}
	//if !formatValid {
	//	return context.JSON(http.StatusBadRequest, ErrorResponse{
	//		Success: false,
	//		Error:   "Invalid format. Supported formats: wav, flac, ogg",
	//		Code:    400,
	//	})
	//}
	log.Info("about to get manager")

	manager := profile.GetManager()
	voice, err := manager.GetOrAllocateVoice(request.Profile, request.Character)
	if err != nil {
		return context.JSON(http.StatusInternalServerError, responses.ErrorResponse{
			Success: false,
			Error:   "Failed to get voice allocation: " + err.Error(),
			Code:    500,
		})
	}

	voiceKey := fmt.Sprintf("%s:%s:%s", voice.Engine, voice.Model, voice.Voice)

	var rawAudioData []byte
	cacheManager := cache.GetManager()
	if cacheManager.IsEnabled() {
		cachedAudio, found := cacheManager.GetCachedAudio(request.Profile, request.Character, request.Text)
		if found {
			rawAudioData = cachedAudio
		}
	}

	if rawAudioData == nil {
		rawAudioData, err = tts.GenerateRawAudio(voice, request.Text)
		if err != nil {
			return context.JSON(http.StatusInternalServerError, responses.ErrorResponse{
				Success: false,
				Error:   "Failed to generate speech: " + err.Error(),
				Code:    500,
			})
		}

		if cacheManager.IsEnabled() {
			if err := cacheManager.CacheAudio(request.Profile, request.Character, request.Text, voiceKey, rawAudioData); err != nil {
				response.Warn("failed to cache audio: %v", err)
			}
		}
	}

	stats.IncrementMessages()

	audioData, err := audio.ConvertRawToFormat(rawAudioData, format)
	if err != nil {
		return context.JSON(http.StatusInternalServerError, responses.ErrorResponse{
			Success: false,
			Error:   "Failed to convert audio format: " + err.Error(),
			Code:    500,
		})
	}

	contentType := audio.GetContentType(format)
	filename := fmt.Sprintf("tts_%s_%s.%s", request.Profile, request.Character, format)

	context.Response().Header().Set("Content-Type", contentType)
	context.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	return context.Blob(http.StatusOK, contentType, audioData)
}

func handleSimpleTTS(context echo.Context) error {
	engineId := context.Param("engineId")
	modelId := context.Param("modelId")
	voiceId := context.Param("voiceId")

	if engineId == "" {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Engine ID is required",
			Code:    400,
		})
	}

	if modelId == "" {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Model ID is required",
			Code:    400,
		})
	}

	if voiceId == "" {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Voice ID is required",
			Code:    400,
		})
	}

	var request SimpleTTSRequest

	if err := context.Bind(&request); err != nil {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Invalid request format",
			Code:    400,
		})
	}

	if strings.TrimSpace(request.Text) == "" {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Text field is required",
			Code:    400,
		})
	}

	if len(request.Text) > 10000 {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Text too long (max 10000 characters)",
			Code:    400,
		})
	}

	format := "wav"
	if request.Options != nil {
		if formatOption, exists := request.Options["format"]; exists {
			if formatStr, ok := formatOption.(string); ok {
				format = strings.ToLower(formatStr)
			}
		}
	}

	validFormats := []string{"wav", "flac", "ogg"}
	formatValid := false
	for _, validFormat := range validFormats {
		if format == validFormat {
			formatValid = true
			break
		}
	}
	if !formatValid {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Invalid format. Supported formats: wav, flac, ogg",
			Code:    400,
		})
	}

	voice := &util.CharacterVoice{
		Engine: engineId,
		Model:  modelId,
		Voice:  voiceId,
	}

	rawAudioData, err := tts.GenerateRawAudio(voice, request.Text)
	if err != nil {
		return context.JSON(http.StatusInternalServerError, responses.ErrorResponse{
			Success: false,
			Error:   "Failed to generate speech: " + err.Error(),
			Code:    500,
		})
	}

	stats.IncrementMessages()

	audioData, err := audio.ConvertRawToFormat(rawAudioData, format)
	if err != nil {
		return context.JSON(http.StatusInternalServerError, responses.ErrorResponse{
			Success: false,
			Error:   "Failed to convert audio format: " + err.Error(),
			Code:    500,
		})
	}

	contentType := audio.GetContentType(format)
	filename := fmt.Sprintf("tts_%s_%s_%s.%s", engineId, modelId, voiceId, format)

	context.Response().Header().Set("Content-Type", contentType)
	context.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	return context.Blob(http.StatusOK, contentType, audioData)
}
