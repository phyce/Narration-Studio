package tts

import (
	"encoding/json"
	"fmt"
	"nstudio/app/cache"
	"nstudio/app/common/audio"
	"nstudio/app/common/response"
	"nstudio/app/common/status"
	"nstudio/app/common/util"
	"nstudio/app/enums/Engines"
	"nstudio/app/tts/engine/elevenlabs"
	"nstudio/app/tts/engine/mssapi4"
	"nstudio/app/tts/engine/openai"
	"nstudio/app/tts/engine/piper"
	"nstudio/app/tts/modelManager"
	"nstudio/app/tts/profile"
	"strconv"
)

func GenerateSpeech(messages []util.CharacterMessage, saveOutput bool, profileID string) error {
	profileManager := profile.GetManager()
	cacheManager := cache.GetManager()

	status.Set(status.Generating, "")

	for _, message := range messages {
		var rawAudio []byte
		var foundInCache bool

		if !saveOutput && cacheManager.IsEnabled() {
			rawAudio, foundInCache = cacheManager.GetCachedAudio(profileID, message.Character, message.Text)
			if foundInCache {
				status.Set(status.Playing, "Using cached audio")

				if len(rawAudio) == 0 {
					return response.NewWarn(fmt.Sprintf("Cached audio is empty for character: %s for message: %s",
						message.Character,
						message.Text,
					))
				}

				status.Set(status.Playing, "")
				audio.PlayRawAudioBytes(rawAudio)
				status.Set(status.Ready, "")
				return nil
			}
		}

		voice, err := profileManager.GetOrAllocateVoice(profileID, message.Character)
		if err != nil {
			return response.Err(err)
		}

		message.Voice = *voice

		selectedEngineInstance, releaseFunc, ok := modelManager.GetEngineInstance(voice.Engine, voice.Model)
		if !ok {
			return response.Err(fmt.Errorf("Failed to retrieve engine instance: %s/%s", voice.Engine, voice.Model))
		}

		if saveOutput {
			err := selectedEngineInstance.Save([]util.CharacterMessage{message}, false)
			releaseFunc()
			if err != nil {
				return response.Err(err)
			}

			// Cache the generated audio if enabled
			// Note: For Save mode, we'd need to read back the saved file to cache it
			// For now, caching only works in play mode
		} else {
			status.Set(status.Playing, "")
			message.Voice = *voice

			payload, err := preparePayload(message)
			if err != nil {
				return response.Err(err)
			}

			rawAudio, err = selectedEngineInstance.Generate(message.Voice.Model, payload)
			releaseFunc()
			if err != nil {
				return response.Err(err)
			}

			audio.PlayRawAudioBytes(rawAudio)

			if cacheManager != nil && cacheManager.IsEnabled() {
				go func(data []byte) {
					voiceKey := fmt.Sprintf("%s:%s:%s", voice.Engine, voice.Model, voice.Voice)
					if err := cacheManager.CacheAudio(profileID, message.Character, message.Text, voiceKey, data); err != nil {
						response.Warn("Background caching failed: %v\n", err)
					}
				}(rawAudio)
			}
		}
	}

	status.Set(status.Ready, "")
	return nil
}

func GenerateAudio(voice *util.CharacterVoice, text string) (*audio.Audio, error) {
	engineInstance, releaseFunc, ok := modelManager.GetEngineInstance(voice.Engine, voice.Model)
	if !ok {
		return nil, response.Err(fmt.Errorf("failed to get engine instance: %s/%s", voice.Engine, voice.Model))
	}
	defer releaseFunc()

	message := util.CharacterMessage{
		Character: voice.Name,
		Text:      text,
		Voice:     *voice,
		Save:      false,
	}

	payload, err := preparePayload(message)
	if err != nil {
		return nil, response.Err(err)
	}

	audioObj, err := engineInstance.GenerateAudio(message.Voice.Model, payload)
	if err != nil {
		return nil, response.Err(err)
	}

	return audioObj, nil
}

func GenerateRawAudio(voice *util.CharacterVoice, text string) ([]byte, error) {
	// Use new GenerateAudio function and convert to raw PCM for backward compatibility
	audioObj, err := GenerateAudio(voice, text)
	if err != nil {
		return nil, err
	}

	return audioObj.ToPCM()
}

func preparePayload(message util.CharacterMessage) ([]byte, error) {
	switch message.Voice.Engine {
	case string(Engines.Piper):
		speakerID, _ := strconv.Atoi(message.Voice.Voice)
		payload := piper.PiperInputLite{
			Text:      message.Text,
			SpeakerID: speakerID,
		}
		result, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}

		return append(result, '\n'), nil

	case string(Engines.OpenAI):
		payload := openai.OpenAIRequest{
			Model: message.Voice.Model,
			Input: message.Text,
			Voice: message.Voice.Voice,
		}
		return json.Marshal(payload)

	case string(Engines.MsSapi4):
		payload := mssapi4.MsSapi4Request{
			Text:  message.Text,
			Voice: message.Voice.Voice,
		}
		return json.Marshal(payload)

	case string(Engines.ElevenLabs):
		payload := elevenlabs.ElevenLabsRequest{
			Text:    message.Text,
			ModelID: message.Voice.Model,
		}
		return json.Marshal(payload)
	}

	return nil, response.NewWarn(fmt.Sprintf("Unsupported engine: %s", message.Voice.Engine))
}
