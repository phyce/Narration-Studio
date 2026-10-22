package http

import (
	"bytes"
	"fmt"
	"io"
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

	"github.com/labstack/echo/v4"
)

func canStream(outputFormat string, audioOpts *AudioOptions) bool {
	if outputFormat != "wav" && outputFormat != "pcm" && outputFormat != "pcm_s16le" {
		return false
	}
	if audioOpts == nil {
		return true
	}
	return audioOpts.SampleRate <= 0 && audioOpts.Channels <= 0
}

func streamTTSResponse(echoContext echo.Context, stream *audio.Stream, outputFormat, filename string) bool {
	httpResponse := echoContext.Response()
	httpResponse.Header().Set("Content-Type", audio.GetContentType(outputFormat))
	httpResponse.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	httpResponse.WriteHeader(http.StatusOK)

	flusher, canFlush := httpResponse.Writer.(http.Flusher)
	flush := func() {
		if canFlush {
			flusher.Flush()
		}
	}

	if outputFormat == "wav" {
		if _, err := httpResponse.Write(audio.StreamingWAVHeader(stream.Format)); err != nil {
			stream.Close()
			return false
		}
		flush()
	}

	chunk := make([]byte, 32*1024)
	for {
		count, err := stream.Read(chunk)
		if count > 0 {
			if _, writeErr := httpResponse.Write(chunk[:count]); writeErr != nil {
				stream.Close()
				return false
			}
			flush()
		}
		if err == io.EOF {
			return true
		}
		if err != nil {
			response.Err("streaming TTS response aborted: %w", err)
			return false
		}
	}
}

func audioFileExtension(outputFormat string) string {
	if outputFormat == "pcm" || outputFormat == "pcm_s16le" {
		return "pcm"
	}
	return outputFormat
}

func audioFromCache(cached []byte) (*audio.Audio, error) {
	if len(cached) == 0 {
		return nil, response.Err("cached audio is empty")
	}
	if len(cached) >= 12 && string(cached[0:4]) == "RIFF" && string(cached[8:12]) == "WAVE" {
		return audio.NewAudioFromWAV(cached)
	}
	return audio.NewAudioFromPCM(cached, 22050, 1, 16), nil
}

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

	audioOpts, err := request.GetAudioOptions()
	if err != nil {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Invalid audio options: " + err.Error(),
			Code:    400,
		})
	}

	outputFormat := "wav"
	if audioOpts != nil && audioOpts.Format != "" {
		outputFormat = strings.ToLower(audioOpts.Format)
	}

	validFormats := []string{"wav", "flac", "ogg", "pcm", "pcm_s16le", "mp3"}
	formatValid := false
	for _, validFormat := range validFormats {
		if outputFormat == validFormat {
			formatValid = true
			break
		}
	}
	if !formatValid {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Invalid format. Supported: wav, flac, ogg, pcm, pcm_s16le, mp3",
			Code:    400,
		})
	}

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

	cacheEnabled := false
	cacheManager := cache.GetManager()
	if cacheManager.IsEnabled() {
		selectedProfile, err := manager.GetProfile(request.Profile)
		if err == nil {
			settings := selectedProfile.GetSettings()
			if settings != nil && settings.CacheEnabled != nil {
				cacheEnabled = *settings.CacheEnabled
			}
		}
	}

	filename := fmt.Sprintf("tts_%s_%s.%s", request.Profile, request.Character, audioFileExtension(outputFormat))

	var audioObject *audio.Audio
	if cacheEnabled {
		if cachedAudio, found := cacheManager.GetCachedAudio(request.Profile, request.Character, request.Text); found {
			cachedObject, err := audioFromCache(cachedAudio)
			if err == nil {
				audioObject = cachedObject
			} else {
				response.Err("ignoring unusable cache entry for %s: %w", request.Character, err)
			}
		}
	}

	if audioObject == nil && canStream(outputFormat, audioOpts) {
		stream, err := tts.SynthesizeStream(context.Request().Context(), voice, request.Text)
		if err != nil {
			return context.JSON(http.StatusInternalServerError, responses.ErrorResponse{
				Success: false,
				Error:   "Failed to generate speech: " + err.Error(),
				Code:    500,
			})
		}

		var capture *bytes.Buffer
		if cacheEnabled {
			capture = &bytes.Buffer{}
			stream = audio.NewTeeStream(stream, capture)
		}

		stats.IncrementMessages()
		completed := streamTTSResponse(context, stream, outputFormat, filename)

		if completed && capture != nil && stream.Err() == nil && stream.TeeErr() == nil && capture.Len() > 0 {
			format := stream.Format
			wavBytes, err := audio.NewAudioFromPCM(capture.Bytes(), format.SampleRate, format.Channels, format.BitDepth).ToWAV()
			if err == nil {
				if err := cacheManager.CacheAudio(request.Profile, request.Character, request.Text, voiceKey, wavBytes); err != nil {
					response.Warn("failed to cache audio: %v", err)
				}
			}
		}
		return nil
	}

	if audioObject == nil {
		audioObject, err = tts.GenerateAudio(voice, request.Text)
		if err != nil {
			return context.JSON(http.StatusInternalServerError, responses.ErrorResponse{
				Success: false,
				Error:   "Failed to generate speech: " + err.Error(),
				Code:    500,
			})
		}

		if cacheEnabled {
			wavBytes, err := audioObject.ToWAV()
			if err == nil {
				if err := cacheManager.CacheAudio(request.Profile, request.Character, request.Text, voiceKey, wavBytes); err != nil {
					response.Warn("failed to cache audio: %v", err)
				}
			}
		}
	}

	stats.IncrementMessages()

	if audioOpts != nil {
		if audioOpts.SampleRate > 0 && audioOpts.SampleRate != audioObject.Metadata.SampleRate {
			if err := audioObject.Resample(audioOpts.SampleRate); err != nil {
				return context.JSON(http.StatusInternalServerError, responses.ErrorResponse{
					Success: false,
					Error:   "Failed to resample audio: " + err.Error(),
					Code:    500,
				})
			}
		}

		if audioOpts.Channels > 0 && audioOpts.Channels != audioObject.Metadata.Channels {
			if err := audioObject.ChangeChannels(audioOpts.Channels); err != nil {
				return context.JSON(http.StatusInternalServerError, responses.ErrorResponse{
					Success: false,
					Error:   "Failed to change channels: " + err.Error(),
					Code:    500,
				})
			}
		}
	}

	audioData, err := audioObject.ToFormat(outputFormat)
	if err != nil {
		return context.JSON(http.StatusInternalServerError, responses.ErrorResponse{
			Success: false,
			Error:   "Failed to convert audio format: " + err.Error(),
			Code:    500,
		})
	}

	contentType := audio.GetContentType(outputFormat)

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

	validFormats := []string{"wav", "flac", "ogg", "pcm", "pcm_s16le"}
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
			Error:   "Invalid format. Supported formats: wav, flac, ogg, pcm, pcm_s16le",
			Code:    400,
		})
	}

	voice := &util.CharacterVoice{
		Engine: engineId,
		Model:  modelId,
		Voice:  voiceId,
	}

	filename := fmt.Sprintf("tts_%s_%s_%s.%s", engineId, modelId, voiceId, audioFileExtension(format))

	if canStream(format, nil) {
		stream, err := tts.SynthesizeStream(context.Request().Context(), voice, request.Text)
		if err != nil {
			return context.JSON(http.StatusInternalServerError, responses.ErrorResponse{
				Success: false,
				Error:   "Failed to generate speech: " + err.Error(),
				Code:    500,
			})
		}

		stats.IncrementMessages()
		streamTTSResponse(context, stream, format, filename)
		return nil
	}

	audioObj, err := tts.GenerateAudio(voice, request.Text)
	if err != nil {
		return context.JSON(http.StatusInternalServerError, responses.ErrorResponse{
			Success: false,
			Error:   "Failed to generate speech: " + err.Error(),
			Code:    500,
		})
	}

	stats.IncrementMessages()

	audioData, err := audioObj.ToFormat(format)
	if err != nil {
		return context.JSON(http.StatusInternalServerError, responses.ErrorResponse{
			Success: false,
			Error:   "Failed to convert audio format: " + err.Error(),
			Code:    500,
		})
	}

	contentType := audio.GetContentType(format)

	context.Response().Header().Set("Content-Type", contentType)
	context.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	return context.Blob(http.StatusOK, contentType, audioData)
}
