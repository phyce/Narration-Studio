package native

import (
	"encoding/json"
	"fmt"
	"nstudio/app/common/audio"
	"nstudio/app/common/response"
	"nstudio/app/common/util"
	"nstudio/app/common/util/fileIndex"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
)

// <editor-fold desc="Engine Interface">

func (piper *Piper) Initialize() error {
	settings := config.GetEngine().Local.Piper

	if piper.models == nil {
		piper.models = make(map[string]*PiperNativeInstance)
	}

	if settings.ModelsDirectory == "" {
		piper.autoDetect(&settings)
	}

	if settings.ModelsDirectory == "" {
		return response.Err(fmt.Errorf("piper models directory is not set"))
	}
	piper.modelsDir = settings.ModelsDirectory
	piper.useGPU = settings.UseGPU

	// espeak-ng-data is always located next to the DLLs (i.e. next to the executable)
	exePath, err := os.Executable()
	if err != nil {
		return response.Err(fmt.Errorf("could not determine executable path: %w", err))
	}
	espeakDir := filepath.Join(filepath.Dir(exePath), "espeak-ng-data")
	if _, err := os.Stat(espeakDir); os.IsNotExist(err) {
		return response.Err(fmt.Errorf("espeak-ng-data not found next to executable: %s", espeakDir))
	}
	piper.espeakDataDir = espeakDir

	return nil
}

// autoDetect looks for a models directory next to the running executable.
func (piper *Piper) autoDetect(settings *config.Piper) {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	candidate := filepath.Join(filepath.Dir(exePath), "models")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		settings.ModelsDirectory = candidate
	}
}

func (piper *Piper) Start(modelName string) error {
	err := piper.Initialize()
	if err != nil {
		configuration := config.Get()
		configuration.ModelToggles[fmt.Sprintf("piper:%s", modelName)] = false
		if setErr := config.Set(configuration); setErr != nil {
			return setErr
		}
		return response.Err(err)
	}

	if _, exists := piper.models[modelName]; exists {
		response.Debug(util.MessageData{
			Summary: fmt.Sprintf("piper model '%s' already loaded", modelName),
		})
		return nil
	}

	// Load voice metadata
	metadataPath := filepath.Join(piper.modelsDir, modelName, fmt.Sprintf("%s.metadata.json", modelName))
	response.Debug(util.MessageData{
		Summary: "piper metadata for model: " + modelName,
		Detail:  metadataPath,
	})

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return response.Err(err)
	}

	var voices []engine.Voice
	if err := json.Unmarshal(data, &voices); err != nil {
		return response.Err(err)
	}

	// Create synthesizer
	onnxPath := filepath.Join(piper.modelsDir, modelName, fmt.Sprintf("%s.onnx", modelName))
	configPath := onnxPath + ".json"

	response.Debug(util.MessageData{
		Summary: fmt.Sprintf("piper loading model: %s", modelName),
		Detail:  fmt.Sprintf("onnx=%s espeak=%s", onnxPath, piper.espeakDataDir),
	})

	synth, err := NewSynthesizer(onnxPath, configPath, piper.espeakDataDir, piper.useGPU)
	if err != nil {
		return response.Err(err)
	}

	piper.models[modelName] = &PiperNativeInstance{
		synth:  synth,
		Voices: voices,
	}

	response.Debug(util.MessageData{
		Summary: fmt.Sprintf("piper model '%s' loaded successfully", modelName),
	})

	return nil
}

func (piper *Piper) Stop(modelName string) error {
	instance, exists := piper.models[modelName]
	if !exists {
		response.Debug(util.MessageData{
			Summary: fmt.Sprintf("piper instance for %s is not running", modelName),
		})
		return nil
	}

	instance.synth.Free()
	delete(piper.models, modelName)

	response.Debug(util.MessageData{
		Summary: fmt.Sprintf("piper stopped model: %s", modelName),
	})

	return nil
}

func (piper *Piper) Play(message util.CharacterMessage) error {
	response.Debug(util.MessageData{
		Summary: "piper playing: " + message.Character,
		Detail:  message.Text,
	})

	speakerID, _ := strconv.Atoi(message.Voice.Voice)

	input := PiperInput{
		Text:      strings.ReplaceAll(message.Text, `"`, `\"`),
		SpeakerID: speakerID,
	}

	jsonBytes, err := json.Marshal(input)
	if err != nil {
		return response.Err(err)
	}

	audioClip, err := piper.Generate(message.Voice.Model, jsonBytes)
	if err != nil {
		return response.Err(err)
	}

	audio.PlayRawAudioBytes(audioClip)
	response.Debug(util.MessageData{
		Summary: "piper finished playing audio for: " + message.Character,
		Detail:  message.Text,
	})
	return nil
}

func (piper *Piper) Save(messages []util.CharacterMessage, play bool) error {
	response.Debug(util.MessageData{
		Summary: "piper saving messages",
	})

	err, outputPath := util.ExpandPath(config.GetSettings().OutputPath)
	if err != nil {
		return response.Err(err)
	}

	for _, message := range messages {
		speakerID, _ := strconv.Atoi(message.Voice.Voice)

		outputFilename := util.GenerateFilename(
			message,
			fileIndex.Get(),
			outputPath,
		)

		input := PiperInput{
			Text:      strings.ReplaceAll(message.Text, `"`, `\"`),
			SpeakerID: speakerID,
		}

		jsonBytes, err := json.Marshal(input)
		if err != nil {
			return response.Err(err)
		}

		audioObj, err := piper.GenerateAudio(message.Voice.Model, jsonBytes)
		if err != nil {
			return response.Err(err)
		}

		wavData, err := audioObj.ToWAV()
		if err != nil {
			return response.Err(err)
		}

		if err := os.WriteFile(outputFilename, wavData, 0644); err != nil {
			return response.Err(err)
		}

		if play {
			audio.PlayRawAudioBytes(audioObj.Data)
		}
	}

	return nil
}

func (piper *Piper) Generate(model string, payload []byte) ([]byte, error) {
	log.Info("generating in piper")

	instance, exists := piper.models[model]
	if !exists {
		if !config.GetEngineToggles()["piper"][model] {
			return nil, response.Err(fmt.Errorf("model is not enabled: piper:%s", model))
		}

		response.NewWarn("piper model is not running: " + model)

		err := piper.Start(model)
		if err != nil {
			return nil, response.Err(fmt.Errorf("failed to start piper model %s: %v", model, err))
		}

		instance = piper.models[model]
	}

	var input PiperInput
	if err := json.Unmarshal(payload, &input); err != nil {
		return nil, response.Err(err)
	}

	response.Debug(util.MessageData{
		Summary: fmt.Sprintf("piper synthesizing model=%s speaker=%d", model, input.SpeakerID),
		Detail:  input.Text,
	})

	instance.mu.Lock()
	defer instance.mu.Unlock()

	opts := instance.synth.DefaultOptions()
	opts.SpeakerID = input.SpeakerID

	pcmBytes, _, err := instance.synth.Synthesize(input.Text, &opts)
	if err != nil {
		return nil, response.Err(err)
	}

	return pcmBytes, nil
}

func (piper *Piper) GenerateAudio(model string, payload []byte) (*audio.Audio, error) {
	rawBytes, err := piper.Generate(model, payload)
	if err != nil {
		return nil, err
	}

	return audio.NewAudioFromPCM(rawBytes, 22050, 1, 16), nil
}

func (piper *Piper) GetVoices(model string) ([]engine.Voice, error) {
	instance, exists := piper.models[model]
	if !exists {
		return nil, response.Err(fmt.Errorf("piper model %s is not initialized", model))
	}

	return instance.Voices, nil
}

func (piper *Piper) FetchModels() map[string]engine.Model {
	return FetchModels()
}

// </editor-fold>
