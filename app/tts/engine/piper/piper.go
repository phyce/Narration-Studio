package piper

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/gopxl/beep"
	"github.com/gopxl/beep/speaker"
	"io"
	"nstudio/app/common/response"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
	"nstudio/app/tts/util"
	process "nstudio/app/tts/util/process"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

type VoiceSynthesizer struct {
	//modelsDirectory string
	//piperPath       string
	command   *exec.Cmd
	stdin     io.WriteCloser
	stderr    io.ReadCloser
	stdout    io.ReadCloser
	audioData *AudioBuffer
	Voices    []engine.Voice
}

type PiperInput struct {
	Text       string `json:"text"`
	SpeakerID  int    `json:"speaker_id"`
	OutputFile string `json:"output_file"`
}

type PiperInputLite struct {
	Text      string `json:"text"`
	SpeakerID int    `json:"speaker_id"`
}

type Piper struct {
	models    map[string]VoiceSynthesizer
	piperPath string
	modelPath string
	initOnce  sync.Once
}

// <editor-fold desc="Engine Interface">
func (piper *Piper) Initialize() error {
	var err error

	piperPathValue := config.GetInstance().GetSetting("piperPath")
	if piperPathValue.String == nil {
		return util.TraceError(fmt.Errorf("Piper:Initialize:piperPathValue: is nil"))
	}

	err, piper.piperPath = util.ExpandPath(*piperPathValue.String)
	if err != nil {
		return util.TraceError(err)
	}

	modelPathValue := config.GetInstance().GetSetting("piperModelsDirectory")
	if modelPathValue.String == nil {
		return util.TraceError(fmt.Errorf("Piper:Initialize:modelPathValue: is nil"))
	}

	if runtime.GOOS == "darwin" {
		err, piper.modelPath = util.ExpandPath(*modelPathValue.String)
		if err != nil {
			return util.TraceError(err)
		}
	}

	piper.models = make(map[string]VoiceSynthesizer)

	return err
}

func (piper *Piper) Start(modelName string) error {
	var err error
	modelProcessID := piper.GetProcessID(modelName)
	if modelProcessID > 0 {
		return util.TraceError(fmt.Errorf("Piper:Start:modelName[%s] already exists. PID: ", modelName, modelProcessID))
	}

	metadataPath := filepath.Join(piper.modelPath, modelName, fmt.Sprintf("%s.metadata.json", modelName))
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return util.TraceError(err)
	}

	var voices []engine.Voice
	if err := json.Unmarshal(data, &voices); err != nil {
		return util.TraceError(err)
	}

	onnxPath := filepath.Join(piper.modelPath, modelName, fmt.Sprintf("%s.onnx", modelName))

	cmdArgs := []string{"--model", onnxPath, "--json-input", "--output-raw"}

	command := exec.Command(piper.piperPath, cmdArgs...)
	response.Debug(response.Data{
		Summary: fmt.Sprintf("Preparing command: %s %s",
			command.Path,
			strings.Join(command.Args[1:], " "),
		),
	})

	instance := VoiceSynthesizer{
		command:   command,
		audioData: &AudioBuffer{},
		Voices:    voices,
	}

	instance.stdin, err = instance.command.StdinPipe()
	if err != nil {
		return util.TraceError(err)
	}

	instance.stderr, err = instance.command.StderrPipe()
	if err != nil {
		return util.TraceError(err)
	}

	instance.stdout, err = instance.command.StdoutPipe()
	if err != nil {
		return util.TraceError(err)
	}

	err = instance.command.Start()
	if err != nil {
		return util.TraceError(err)
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

	// Send interrupt signal to the process
	if err := instance.command.Process.Signal(os.Interrupt); err != nil {
		// If sending interrupt fails, attempt to kill the process
		if killErr := instance.command.Process.Kill(); killErr != nil {
			return util.TraceError(fmt.Errorf("failed to kill process for model %s: %v, original error: %v", modelName, killErr, err))
		}
	}

	// Wait for the process to exit
	err := instance.command.Wait()

	// Handle the error returned by Wait()
	if err != nil {
		// Check if the error is an ExitError
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Check if the process was terminated by a signal
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				if status.Signaled() && status.Signal() == os.Interrupt {
					// Process exited due to os.Interrupt, which is expected
					// Do not treat as an error
				} else {
					// Process was terminated by a different signal or exited with a non-zero status
					return util.TraceError(fmt.Errorf("process for model %s exited with signal: %v", modelName, status.Signal()))
				}
			} else {
				// Unable to determine the exit status
				return util.TraceError(fmt.Errorf("process for model %s exited with error: %v", modelName, err))
			}
		} else {
			// Some other error occurred
			return util.TraceError(fmt.Errorf("process for model %s exited with error: %v", modelName, err))
		}
	}

	// Close stdin, stdout, and stderr
	if instance.stdin != nil {
		if err := instance.stdin.Close(); err != nil {
			return util.TraceError(fmt.Errorf("failed to close stdin for model %s: %v", modelName, err))
		}
	}
	if instance.stdout != nil {
		if err := instance.stdout.Close(); err != nil {
			return util.TraceError(fmt.Errorf("failed to close stdout for model %s: %v", modelName, err))
		}
	}
	if instance.stderr != nil {
		if err := instance.stderr.Close(); err != nil {
			return util.TraceError(fmt.Errorf("failed to close stderr for model %s: %v", modelName, err))
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
		return util.TraceError(err)
	}
	jsonBytes = append(jsonBytes, '\n')

	audioClip, err := piper.Generate(message.Voice.Model, jsonBytes)
	if err != nil {
		return util.TraceError(err)
	}

	playRawAudioBytes(audioClip)
	return nil
}

func (piper *Piper) Save(messages []util.CharacterMessage, play bool) error {
	response.Debug(response.Data{
		Summary: "Piper saving messages",
	})

	err, expandedPath := util.ExpandPath(*config.GetInstance().GetSetting("scriptOutputPath").String)
	if err != nil {
		return util.TraceError(err)
	}

	for _, message := range messages {
		speakerID, _ := strconv.Atoi(message.Voice.Voice)

		input := PiperInput{
			Text:      strings.ReplaceAll(message.Text, `"`, `\"`),
			SpeakerID: speakerID,
			OutputFile: util.GenerateFilename(
				message,
				util.FileIndexGet(),
				expandedPath,
			),
		}

		jsonBytes, err := json.Marshal(input)
		if err != nil {
			return util.TraceError(err)
		}
		jsonBytes = append(jsonBytes, '\n')

		audioClip, err := piper.Generate(message.Voice.Model, jsonBytes)
		if err != nil {
			return util.TraceError(err)
		}

		if play {
			playRawAudioBytes(audioClip)
		}
	}

	return nil
}

func (piper *Piper) Generate(model string, payload []byte) ([]byte, error) {
	if piper.GetProcessID(model) == 0 {
		if !config.GetInstance().GetModelToggles()["piper"][model] {
			return make([]byte, 0), util.TraceError(fmt.Errorf("Model is not running and not enabled:" + model))
		}
		util.TraceError(fmt.Errorf("Model is not running:" + model))

		err := piper.Start(model)
		if err != nil {
			util.TraceError(fmt.Errorf("Failed to start model %s: %v", model, err))
		}
	}

	if utf8.Valid(payload) == false {
		return nil, util.TraceError(fmt.Errorf("input JSON is not valid UTF-8"))
	}
	if _, err := piper.models[model].stdin.Write(payload); err != nil {
		return nil, util.TraceError(err)
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
		return nil, util.TraceError(fmt.Errorf("model %s is not initialized", model))
	}
	return modelData.Voices, nil
}

func (piper *Piper) FetchModels() map[string]engine.Model {
	return FetchModels()
}

// </editor-fold>

// <editor-fold desc="Other">
func (piper *Piper) StartAudioCapture(instance VoiceSynthesizer) {
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

func playRawAudioBytes(audioClip []byte) {
	done := make(chan struct{})
	audioDataReader := bytes.NewReader(audioClip)

	streamer := beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		for i := range samples {
			if audioDataReader.Len() < 2 { // 2 bytes needed for one sample
				close(done)
				return i, false
			}
			var sample int16

			err := binary.Read(audioDataReader, binary.LittleEndian, &sample)
			if err != nil {
				close(done)
				return i, false
			}
			flSample := float64(sample) / (1 << 15)
			samples[i][0] = flSample
			samples[i][1] = flSample
		}
		return len(samples), true
	})

	speaker.Play(streamer)
	<-done
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
