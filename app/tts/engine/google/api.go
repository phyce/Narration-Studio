package google

import (
	"context"
	"fmt"
	"nstudio/app/common/response"
	"nstudio/app/common/util"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
	"strings"

	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"cloud.google.com/go/texttospeech/apiv1/texttospeechpb"
	"google.golang.org/api/option"
)

func (google *Google) sendRequest(data GoogleRequest) ([]byte, error) {
	ctx := context.Background()
	apiKey := config.GetEngine().Api.Google.ApiKey
	if apiKey == "" {
		return nil, response.Err(fmt.Errorf("Google Cloud API key is not set"))
	}

	client, err := texttospeech.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, response.Err(fmt.Errorf("Failed to create Google TTS client: %v", err))
	}
	defer client.Close()

	audioEncoding := texttospeechpb.AudioEncoding_MP3
	if data.AudioConfig.AudioEncoding == "LINEAR16" {
		audioEncoding = texttospeechpb.AudioEncoding_LINEAR16
	} else if data.AudioConfig.AudioEncoding == "OGG_OPUS" {
		audioEncoding = texttospeechpb.AudioEncoding_OGG_OPUS
	} else if data.AudioConfig.AudioEncoding == "MULAW" {
		audioEncoding = texttospeechpb.AudioEncoding_MULAW
	} else if data.AudioConfig.AudioEncoding == "ALAW" {
		audioEncoding = texttospeechpb.AudioEncoding_ALAW
	}

	request := &texttospeechpb.SynthesizeSpeechRequest{
		Input: &texttospeechpb.SynthesisInput{
			InputSource: &texttospeechpb.SynthesisInput_Text{
				Text: data.Input.Text,
			},
		},
		Voice: &texttospeechpb.VoiceSelectionParams{
			LanguageCode: data.Voice.LanguageCode,
			Name:         data.Voice.Name,
			ModelName:    data.Voice.ModelName,
		},
		AudioConfig: &texttospeechpb.AudioConfig{
			AudioEncoding: audioEncoding,
			SpeakingRate:  data.AudioConfig.SpeakingRate,
			Pitch:         data.AudioConfig.Pitch,
		},
	}

	if data.Voice.SsmlGender != "" {
		switch data.Voice.SsmlGender {
		case "MALE":
			request.Voice.SsmlGender = texttospeechpb.SsmlVoiceGender_MALE
		case "FEMALE":
			request.Voice.SsmlGender = texttospeechpb.SsmlVoiceGender_FEMALE
		case "NEUTRAL":
			request.Voice.SsmlGender = texttospeechpb.SsmlVoiceGender_NEUTRAL
		}
	}

	synthesisResponse, err := client.SynthesizeSpeech(ctx, request)
	if err != nil {
		return nil, response.Err(fmt.Errorf("Google TTS request failed: %v", err))
	}

	response.Success(util.MessageData{
		Summary: "Request succeeded",
		Detail:  "Google TTS generated audio",
	})

	return synthesisResponse.AudioContent, nil
}

func (google *Google) fetchVoices(model string) ([]engine.Voice, error) {
	google.mu.RLock()
	if len(google.voiceCache) > 0 {
		if voices, ok := google.voiceCache[model]; ok {
			google.mu.RUnlock()
			return voices, nil
		}
		google.mu.RUnlock()
		return []engine.Voice{}, nil
	}
	google.mu.RUnlock()

	google.mu.Lock()
	defer google.mu.Unlock()

	if len(google.voiceCache) > 0 {
		if voices, ok := google.voiceCache[model]; ok {
			return voices, nil
		}
		return []engine.Voice{}, nil
	}

	ctx := context.Background()
	apiKey := config.GetEngine().Api.Google.ApiKey
	if apiKey == "" {
		return nil, response.Err(fmt.Errorf("Google Cloud API key is not set"))
	}

	client, err := texttospeech.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, response.Err(fmt.Errorf("Failed to create Google TTS client: %v", err))
	}
	defer client.Close()

	request := &texttospeechpb.ListVoicesRequest{}
	dataResponse, err := client.ListVoices(ctx, request)
	if err != nil {
		return nil, response.Err(fmt.Errorf("Failed to list voices: %v", err))
	}

	// Initialize cache map if nil (safety check)
	if google.voiceCache == nil {
		google.voiceCache = make(map[string][]engine.Voice)
	}

	for _, v := range dataResponse.Voices {
		gender := "Unknown"
		switch v.SsmlGender {
		case texttospeechpb.SsmlVoiceGender_MALE:
			gender = "Male"
		case texttospeechpb.SsmlVoiceGender_FEMALE:
			gender = "Female"
		case texttospeechpb.SsmlVoiceGender_NEUTRAL:
			gender = "Neutral"
		}

		langCode := ""
		if len(v.LanguageCodes) > 0 {
			langCode = v.LanguageCodes[0]
		}

		voice := engine.Voice{
			ID:     v.Name,
			Name:   fmt.Sprintf("%s (%s)", v.Name, langCode),
			Gender: gender,
		}

		if strings.Contains(v.Name, "Studio") {
			google.voiceCache["studio"] = append(google.voiceCache["studio"], voice)
		} else if strings.Contains(v.Name, "Neural2") {
			google.voiceCache["neural2"] = append(google.voiceCache["neural2"], voice)
		} else if strings.Contains(v.Name, "Wavenet") {
			google.voiceCache["wavenet"] = append(google.voiceCache["wavenet"], voice)
		} else if strings.Contains(v.Name, "Polyglot") {
			google.voiceCache["polyglot"] = append(google.voiceCache["polyglot"], voice)
		} else if strings.Contains(v.Name, "Chirp3-HD") {
			google.voiceCache["chirp-3-hd"] = append(google.voiceCache["chirp-3-hd"], voice)
		} else if strings.Contains(v.Name, "Chirp") {
			google.voiceCache["chirp"] = append(google.voiceCache["chirp"], voice)
		} else if strings.Contains(v.Name, "Standard") {
			google.voiceCache["standard"] = append(google.voiceCache["standard"], voice)
		} else if strings.Contains(v.Name, "News") || strings.Contains(v.Name, "Casual") || strings.Contains(v.Name, "Journey") {
			google.voiceCache["gemini-tts"] = append(google.voiceCache["gemini-tts"], voice)
		} /* else {
			google.voiceCache["chirp-3"] = append(google.voiceCache["chirp-3"], voice)
		}*/
	}

	if voices, ok := google.voiceCache[model]; ok {
		return voices, nil
	}

	return []engine.Voice{}, nil
}
