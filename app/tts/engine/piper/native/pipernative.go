package native

import (
	"context"
	"encoding/json"
	"fmt"
	"nstudio/app/common/audio"
	"nstudio/app/common/response"
	"nstudio/app/common/util"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
	"os"
	"path/filepath"
	"strconv"

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
		return response.Err("piper models directory is not set")
	}
	piper.modelsDir = settings.ModelsDirectory
	piper.useGPU = settings.UseGPU

	// Use configured espeak data dir, or search for it
	espeakDir := ""
	if settings.EspeakDataDir != "" {
		if _, err := os.Stat(settings.EspeakDataDir); err == nil {
			espeakDir = settings.EspeakDataDir
		}
	}

	if espeakDir == "" {
		exePath, err := os.Executable()
		if err != nil {
			return response.Err("could not determine executable path: %w", err)
		}
		exeDir := filepath.Dir(exePath)

		searchPaths := []string{
			filepath.Join(exeDir, "espeak-ng-data"),
			filepath.Join(exeDir, "engines", "piper", "espeak-ng-data"),
			filepath.Join(exeDir, "..", "windows", "installer", "engines", "piper", "espeak-ng-data"),
		}

		for _, candidate := range searchPaths {
			if _, err := os.Stat(candidate); err == nil {
				espeakDir = candidate
				break
			}
		}
	}

	if espeakDir == "" {
		return response.Err("espeak-ng-data not found")
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
		return response.Err("%w", err)
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
		return response.Err("%w", err)
	}

	var voices []engine.Voice
	if err := json.Unmarshal(data, &voices); err != nil {
		return response.Err("%w", err)
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
		return response.Err("%w", err)
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

func (piper *Piper) Synthesize(ctx context.Context, request engine.SpeechRequest) (*audio.Stream, error) {
	speakerID, _ := strconv.Atoi(request.Voice)

	pcmBytes, sampleRate, err := piper.synthesizePCM(request.Model, request.Text, speakerID)
	if err != nil {
		return nil, response.Err("%w", err)
	}

	return audio.NewBufferedStream(audio.StreamFormat{SampleRate: sampleRate, Channels: 1, BitDepth: 16}, pcmBytes), nil
}

func (piper *Piper) synthesizePCM(model, text string, speakerID int) ([]byte, int, error) {
	log.Info("generating in piper")

	instance, exists := piper.models[model]
	if !exists {
		if !config.GetEngineToggles()["piper"][model] {
			return nil, 0, response.Err("model is not enabled: piper:%s", model)
		}

		response.NewWarn("piper model is not running: " + model)

		err := piper.Start(model)
		if err != nil {
			return nil, 0, response.Err("failed to start piper model %s: %v", model, err)
		}

		instance = piper.models[model]
	}

	response.Debug(util.MessageData{
		Summary: fmt.Sprintf("piper synthesizing model=%s speaker=%d", model, speakerID),
		Detail:  text,
	})

	instance.mu.Lock()
	defer instance.mu.Unlock()

	opts := instance.synth.DefaultOptions()
	opts.SpeakerID = speakerID

	pcmBytes, sampleRate, err := instance.synth.Synthesize(text, &opts)
	if err != nil {
		return nil, 0, response.Err("%w", err)
	}
	if sampleRate <= 0 {
		sampleRate = 22050
	}

	return pcmBytes, sampleRate, nil
}

func (piper *Piper) GetVoices(model string) ([]engine.Voice, error) {
	instance, exists := piper.models[model]
	if !exists {
		return nil, response.Err("piper model %s is not initialized", model)
	}

	return instance.Voices, nil
}

func (piper *Piper) FetchModels() map[string]engine.Model {
	return FetchModels()
}

// </editor-fold>
