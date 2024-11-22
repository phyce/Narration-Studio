package piper

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"nstudio/app/common/audio"
	"nstudio/app/common/issue"
	"nstudio/app/common/process"
	"nstudio/app/common/response"
	"nstudio/app/common/util"
	"nstudio/app/common/util/fileIndex"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unicode/utf8"
)

// <editor-fold desc="Audio Buffer">
type AudioBuffer struct {
	sync.Mutex
	buffer bytes.Buffer
}

func (ab *AudioBuffer) Write(p []byte) (n int, err error) {
	ab.Lock()
	defer ab.Unlock()
	return ab.buffer.Write(p)
}

func (ab *AudioBuffer) Read(p []byte) (n int, err error) {
	ab.Lock()
	defer ab.Unlock()
	return ab.buffer.Read(p)
}

func (ab *AudioBuffer) Reset() {
	ab.Lock()
	defer ab.Unlock()
	ab.buffer.Reset()
}

//</editor-fold>

// <editor-fold desc="Engine Interface">
func (piper *Piper) Initialize() error {
	var err error
	settings := config.GetEngine().Local.Piper

	if settings.Path == "" {
		return issue.Trace(fmt.Errorf("piper executable path is empty"))
	}

	err, piper.piperPath = util.ExpandPath(settings.Path)
	if err != nil {
		return issue.Trace(err)
	}

	if settings.ModelsPath == "" {
		return issue.Trace(fmt.Errorf("Piper:Initialize:modelPathValue: is nil"))
	}

	err, piper.modelPath = util.ExpandPath(settings.ModelsPath)
	if err != nil {
		return issue.Trace(err)
	}

	piper.models = make(map[string]PiperInstance)

	return err
}

func (piper *Piper) Start(modelName string) error {
	var err error
	modelProcessID := piper.GetProcessID(modelName)
	if modelProcessID > 0 {
		response.Debug(response.Data{
			Summary: "Piper model '%s' already exists. " + modelName,
			Detail:  fmt.Sprintf("PID: %d", modelProcessID),
		})
		return nil
	}

	metadataPath := filepath.Join(piper.modelPath, modelName, fmt.Sprintf("%s.metadata.json", modelName))
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return issue.Trace(err)
	}

	var voices []engine.Voice
	if err := json.Unmarshal(data, &voices); err != nil {
		return issue.Trace(err)
	}

	onnxPath := filepath.Join(piper.modelPath, modelName, fmt.Sprintf("%s.onnx", modelName))

	commandArguments := []string{"--model", onnxPath, "--json-input", "--output-raw"}

	command := exec.Command(piper.piperPath, commandArguments...)

	response.Debug(response.Data{
		Summary: fmt.Sprintf("Preparing command: %s %s",
			command.Path,
			strings.Join(command.Args[1:], " "),
		),
	})

	instance := PiperInstance{
		command:   command,
		audioData: &AudioBuffer{},
		Voices:    voices,
	}

	instance.stdin, err = instance.command.StdinPipe()
	if err != nil {
		return issue.Trace(err)
	}

	instance.stderr, err = instance.command.StderrPipe()
	if err != nil {
		return issue.Trace(err)
	}

	instance.stdout, err = instance.command.StdoutPipe()
	if err != nil {
		return issue.Trace(err)
	}

	err = instance.command.Start()
	if err != nil {
		return issue.Trace(err)
	}

	piper.StartAudioCapture(instance)

	piper.models[modelName] = instance

	return nil
}

func (piper *Piper) Stop(modelName string) error {
	defer delete(piper.models, modelName)

	instance, exists := piper.models[modelName]
	if !exists {
		response.Debug(response.Data{
			Summary: fmt.Sprintf("Instance for %s is not running", modelName),
		})
		return nil
	}

	if err := instance.command.Process.Signal(os.Interrupt); err != nil {
		if killErr := instance.command.Process.Kill(); killErr != nil {
			return issue.Trace(fmt.Errorf("failed to kill process for model %s: %v, original issue: %v", modelName, killErr, err))
		}
	}

	err := instance.command.Wait()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				if !(status.Signaled() && status.Signal() == os.Interrupt) {
					return issue.Trace(fmt.Errorf("process for model %s exited with signal: %v", modelName, status.Signal()))
				}
			}
		}

		return issue.Trace(fmt.Errorf("process for model %s exited with issue: %v", modelName, err))
	}

	if instance.stdin != nil {
		if err := instance.stdin.Close(); err != nil {
			return issue.Trace(fmt.Errorf("failed to close stdin for model %s: %v", modelName, err))
		}
	}
	if instance.stdout != nil {
		if err := instance.stdout.Close(); err != nil {
			return issue.Trace(fmt.Errorf("failed to close stdout for model %s: %v", modelName, err))
		}
	}
	if instance.stderr != nil {
		if err := instance.stderr.Close(); err != nil {
			return issue.Trace(fmt.Errorf("failed to close stderr for model %s: %v", modelName, err))
		}
	}

	response.Debug(response.Data{
		Summary: fmt.Sprintf("Stopped model: %s", modelName),
	})

	return nil
}

func (piper *Piper) Play(message util.CharacterMessage) error {
	response.Debug(response.Data{
		Summary: "Piper playing:" + message.Character,
		Detail:  message.Text,
	})

	speakerID, _ := strconv.Atoi(message.Voice.Voice)

	input := PiperInputLite{
		Text:      strings.ReplaceAll(message.Text, `"`, `\"`),
		SpeakerID: speakerID,
	}

	jsonBytes, err := json.Marshal(input)
	if err != nil {
		return issue.Trace(err)
	}
	jsonBytes = append(jsonBytes, '\n')

	audioClip, err := piper.Generate(message.Voice.Model, jsonBytes)
	if err != nil {
		return issue.Trace(err)
	}

	audio.PlayRawAudioBytes(audioClip)
	response.Debug(response.Data{
		Summary: "Finshed playing audio for:" + message.Character,
		Detail:  message.Text,
	})
	return nil
}

func (piper *Piper) Save(messages []util.CharacterMessage, play bool) error {
	response.Debug(response.Data{
		Summary: "Piper saving messages",
	})

	err, outputPath := util.ExpandPath(config.GetSettings().OutputPath)
	if err != nil {
		return issue.Trace(err)
	}

	for _, message := range messages {
		speakerID, _ := strconv.Atoi(message.Voice.Voice)

		outputFilename := util.GenerateFilename(
			message,
			fileIndex.Get(),
			outputPath,
		)

		input := PiperInput{
			Text:       strings.ReplaceAll(message.Text, `"`, `\"`),
			SpeakerID:  speakerID,
			OutputFile: outputFilename,
		}

		jsonBytes, err := json.Marshal(input)
		if err != nil {
			return issue.Trace(err)
		}
		jsonBytes = append(jsonBytes, '\n')

		audioClip, err := piper.Generate(message.Voice.Model, jsonBytes)
		if err != nil {
			return issue.Trace(err)
		}

		if play {
			audio.PlayRawAudioBytes(audioClip)
		}
	}

	return nil
}

func (piper *Piper) Generate(model string, payload []byte) ([]byte, error) {
	if piper.GetProcessID(model) == 0 {
		if !config.GetEngineToggles()["piper"][model] {
			return make([]byte, 0), issue.Trace(fmt.Errorf("Model is not enabled:" + model))
		}

		//no need to return, simply send error
		issue.Trace(fmt.Errorf("Model is not running:" + model))

		err := piper.Start(model)
		if err != nil {
			issue.Trace(fmt.Errorf("Failed to start model %s: %v", model, err))
		}
	}

	if !utf8.Valid(payload) {
		return nil, issue.Trace(fmt.Errorf("input JSON is not valid UTF-8"))
	}

	response.Debug(response.Data{
		Summary: fmt.Sprintf("Sending to piper model: %s payload: %s", model, string(payload)),
	})
	if _, err := piper.models[model].stdin.Write(payload); err != nil {
		return nil, issue.Trace(err)
	}

	endSignal := make(chan bool)
	go func() {
		scanner := bufio.NewScanner(piper.models[model].stderr)
		for scanner.Scan() {
			text := scanner.Text()

			if strings.HasSuffix(text, " sec)") {
				endSignal <- true
				return
			}
		}
	}()
	<-endSignal

	audioBytes := piper.models[model].audioData.buffer.Bytes()
	audioClip := make([]byte, len(audioBytes))
	copy(audioClip, audioBytes)

	piper.models[model].audioData.Reset()

	return audioClip, nil
}

func (piper *Piper) GetVoices(model string) ([]engine.Voice, error) {
	modelData, exists := piper.models[model]
	if !exists {
		return nil, issue.Trace(fmt.Errorf("model %s is not initialized", model))
	}
	return modelData.Voices, nil
}

func (piper *Piper) FetchModels() map[string]engine.Model {
	return FetchModels()
}

// </editor-fold>

// <editor-fold desc="Other">
func (piper *Piper) StartAudioCapture(instance PiperInstance) {
	go func() {
		_, err := io.Copy(instance.audioData, instance.stdout)
		if err != nil {
			response.Error(response.Data{
				Summary: "Failed to start capturing audio",
				Detail:  instance.command.String() + "\n" + err.Error(),
			})
		}
	}()
}

func (piper *Piper) GetProcessID(modelName string) int {
	instance, exists := piper.models[modelName]
	if !exists {
		return 0
	}

	if instance.command.Process == nil {
		return 0
	}

	if instance.command.ProcessState != nil && instance.command.ProcessState.Exited() {
		return 0
	}

	if !process.IsRunning(instance.command.Process) {
		return 0
	}

	return instance.command.Process.Pid
}

func FetchModels() map[string]engine.Model {
	return map[string]engine.Model{
		"libritts": {
			ID:     "libritts",
			Name:   "LibriTTS",
			Engine: "piper",
			Download: engine.ModelDownload{
				Metadata: "",
				Model:    "https://mechanic.ink/narrator-studio/models/en/en_GB/vctk/medium/en_GB-vctk-medium.onnx",
				Phonemes: "https://mechanic.ink/narrator-studio/models/en/en_GB/vctk/medium/en_GB-vctk-medium.onnx.json",
			},
		},
		"vctk": {
			ID:     "vctk",
			Name:   "VCTK",
			Engine: "piper",
			Download: engine.ModelDownload{
				Metadata: "",
				Model:    "https://mechanic.ink/narrator-studio/models/en/en_GB/vctk/medium/en_GB-vctk-medium.onnx",
				Phonemes: "https://mechanic.ink/narrator-studio/models/en/en_GB/vctk/medium/en_GB-vctk-medium.onnx.json",
			},
		},
	}
}

// </editor-fold>
