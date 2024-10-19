package elevenlabs

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/gopxl/beep"
	"github.com/gopxl/beep/speaker"
	"io"
	"log"
	"net/http"
	"nstudio/app/common/response"
	"nstudio/app/tts/engine"
	"nstudio/app/tts/util"
	"nstudio/app/tts/voiceManager"
)

type ElevenLabs struct {
	Models     map[string]Model
	apiKey     string
	outputType string
}

var voices = make([]engine.Voice, 0)

// <editor-fold desc="Engine Interface">
func (labs *ElevenLabs) Initialize() error {
	var err error
	voices, err = FetchVoices()
	if err != nil {
		return util.TraceError(err)
	}

	labs.apiKey, err = getApiKey()
	if err != nil {
		return util.TraceError(err)
	}

	labs.outputType = "pcm_24000"

	//TODO add api key check

	return nil
}

func (labs *ElevenLabs) Start(modelName string) error {
	return nil
}

func (labs *ElevenLabs) Stop(modelName string) error {
	return nil
}

func (labs *ElevenLabs) Play(message util.CharacterMessage) error {
	response.Debug(response.Data{
		Summary: "Elevenlabs playing:" + message.Character,
		Detail:  message.Text,
	})

	voice, err := voiceManager.GetInstance().GetVoice(message.Character, false)
	if err != nil {
		return util.TraceError(err)
	}

	input := ElevenLabsRequest{
		Text:    message.Text,
		ModelID: voice.Model,
		VoiceSettings: VoiceSettings{
			Stability:       0.5,
			SimilarityBoost: 0.5,
		},
	}

	audioClip, err := labs.sendRequest(voice.Voice, input)
	if err != nil {
		return util.TraceError(err)
	}

	err = playPCMAudioBytes(audioClip) // Assuming PCM format based on output_format
	if err != nil {
		return util.TraceError(err)
	}

	return response.Success(response.Data{
		Summary: "ElevenLabs finished playing audio",
	})

	return nil
}

func (labs *ElevenLabs) Save(messages []util.CharacterMessage, play bool) error {
	return nil
}

func (labs *ElevenLabs) Generate(model string, payload []byte) ([]byte, error) {
	return make([]byte, 0), nil
}

func (labs *ElevenLabs) GetVoices(model string) ([]engine.Voice, error) {
	return voices, nil
}

// </editor-fold>

// <editor-fold desc="Other">
func (labs *ElevenLabs) sendRequest(voiceID string, data ElevenLabsRequest) ([]byte, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, util.TraceError(fmt.Errorf("failed to marshal request body: %v", err))
	}

	url := fmt.Sprintf("https://api.elevenlabs.io/v1/text-to-speech/%s?output_format=%s", voiceID, labs.outputType)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, util.TraceError(fmt.Errorf("failed to create HTTP request: %v", err))
	}

	req.Header.Set("xi-api-key", labs.apiKey)
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
		Summary: "ElevenLabs request succeeded",
		Detail:  "Response Status: " + resp.Status,
	})

	return responseData, nil
}

func playPCMAudioBytes(audioClip []byte) error {
	// Create a reader for the PCM data
	audioDataReader := bytes.NewReader(audioClip)

	// Define the original audio format (24,000 Hz, mono, 16-bit PCM)
	originalFormat := beep.Format{
		SampleRate:  24000, // Original sample rate
		NumChannels: 1,     // Mono
		Precision:   2,     // 16-bit PCM
	}

	// Create a Streamer that reads the PCM data and converts it to float64 samples
	streamer := beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		for i := range samples {
			// Each sample requires 2 bytes (16-bit PCM)
			if audioDataReader.Len() < 2 {
				return i, false
			}

			var sample int16
			err := binary.Read(audioDataReader, binary.LittleEndian, &sample)
			if err != nil {
				log.Printf("Error reading PCM data: %v", err)
				return i, false
			}

			// Convert the sample to float64 in range [-1.0, 1.0]
			flSample := float64(sample) / (1 << 15)

			// Since the speaker is initialized as mono, duplicate the sample for both channels
			samples[i][0] = flSample // Left channel
			samples[i][1] = flSample // Right channel

			n++
		}
		return len(samples), true
	})

	// Resample the audio from 24,000 Hz to 48,000 Hz
	resampler := beep.Resample(4, originalFormat.SampleRate, beep.SampleRate(48000), streamer)

	// Create a channel to signal when playback is done
	done := make(chan bool)

	// Play the resampled audio and signal when done
	speaker.Play(beep.Seq(resampler, beep.Callback(func() {
		done <- true
	})))

	// Wait until playback is finished
	<-done

	return nil
}

// </editor-fold>
