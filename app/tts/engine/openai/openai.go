package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gopxl/beep"
	"github.com/gopxl/beep/flac"
	"github.com/gopxl/beep/speaker"
	"io"
	"net/http"
	"nstudio/app/common/response"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
	"nstudio/app/tts/util"
	"nstudio/app/tts/voiceManager"
)

type Model struct {
	Voices []engine.Voice `json:"voice"`
}

type OpenAI struct {
	Models     map[string]Model
	apiKey     string
	outputType string
}

var voices = []engine.Voice{
	engine.Voice{ID: "alloy", Name: "Alloy", Gender: ""},
	engine.Voice{ID: "echo", Name: "Echo", Gender: ""},
	engine.Voice{ID: "fable", Name: "Fable", Gender: ""},
	engine.Voice{ID: "onyx", Name: "Onyx", Gender: ""},
	engine.Voice{ID: "nova", Name: "Nova", Gender: ""},
	engine.Voice{ID: "shimmer", Name: "Shimmer", Gender: ""},
}

func (openAI *OpenAI) Initialize() error {
	openAI.apiKey = *config.GetInstance().GetSetting("openAiApiKey").String
	//openAI.outputType = *config.GetInstance().GetSetting("openAiOutputType").String
	openAI.outputType = "flac"

	//TODO add api key check

	return nil
}

func (openAI *OpenAI) Start(modelName string) error {
	return nil
}
func (openAI *OpenAI) Stop(modelName string) error {
	return nil
}

func (openAI *OpenAI) Play(message util.CharacterMessage) error {
	response.Debug(response.Data{
		Summary: "OpenAI playing:" + message.Character,
		Detail:  message.Text,
	})

	voice, err := voiceManager.GetInstance().GetVoice(message.Character, false)
	if err != nil {
		return util.TraceError(err)
	}

	input := OpenAIRequest{
		Voice:          voice.Voice,
		Input:          message.Text,
		Model:          voice.Model,
		ResponseFormat: openAI.outputType,
		Speed:          1,
	}

	audioClip, err := openAI.sendRequest(input)
	if err != nil {
		return util.TraceError(err)
	}

	err = playFLACAudioBytes(audioClip)
	if err != nil {
		return util.TraceError(err)
	}

	return response.Success(response.Data{
		Summary: "OpenAI finished playing flac",
	})
}

func (openAI *OpenAI) Save(messages []util.CharacterMessage, play bool) error {
	return nil
}

// TODO mb remove this from interface?
func (openAI *OpenAI) Generate(model string, jsonBytes []byte) ([]byte, error) {
	return make([]byte, 0), nil
}

func (openAI *OpenAI) GetVoices(model string) ([]engine.Voice, error) {
	return voices, nil
}

type OpenAIRequest struct {
	Voice          string  `json:"voice"`
	Input          string  `json:"input"`
	Model          string  `json:"model"`
	ResponseFormat string  `json:"response_format"`
	Speed          float64 `json:"speed"`
}

func (openAI *OpenAI) sendRequest(data OpenAIRequest) ([]byte, error) {

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, util.TraceError(fmt.Errorf("failed to marshal request body: %v", err))
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/audio/speech", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, util.TraceError(fmt.Errorf("failed to create HTTP request: %v", err))
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", openAI.apiKey))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, util.TraceError(fmt.Errorf("failed to send HTTP request: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, util.TraceError(fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes)))
	}

	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, util.TraceError(fmt.Errorf("failed to read response body: %v", err))
	}

	response.Success(response.Data{
		Summary: "Request succeeded?",
		Detail:  "Response Status: " + resp.Status,
	})

	return responseData, nil
}

func playFLACAudioBytes(audioClip []byte) error {
	audioReader := io.NopCloser(bytes.NewReader(audioClip))

	streamer, format, err := flac.Decode(audioReader)
	if err != nil {
		return err
	}
	defer streamer.Close()

	sampleRate := beep.SampleRate(48000)

	//skipping, speaker already initialized, with:
	/*
		format := beep.Format{
			SampleRate:  48000,
			NumChannels: 1,
			Precision:   2,
		}
	*/
	//speaker.Init(sampleRate, sampleRate.N(time.Second/10))

	resampled := beep.Resample(4, format.SampleRate, sampleRate, streamer)

	done := make(chan bool)
	speaker.Play(beep.Seq(resampled, beep.Callback(func() {
		done <- true
	})))

	<-done

	return nil
}
