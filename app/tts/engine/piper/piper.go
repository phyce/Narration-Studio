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
	"io/ioutil"
	"nstudio/app/common/response"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
	"nstudio/app/tts/util"
	"nstudio/app/tts/voiceManager"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
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

var Format = beep.Format{
	SampleRate:  beep.SampleRate(22050),
	NumChannels: 1,
	Precision:   2,
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
func (piper *Piper) Initialize(models []string) error {
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

	err, piper.modelPath = util.ExpandPath(*modelPathValue.String)
	if err != nil {
		return util.TraceError(err)
	}

	if runtime.GOOS == "darwin" {
		err, piper.modelPath = util.ExpandPath(piper.modelPath)
		if err != nil {
			return util.TraceError(err)
		}
	}

	piper.models = make(map[string]VoiceSynthesizer)
	for _, model := range models {
		err := piper.InitializeModel(model)
		if err != nil {
			return util.TraceError(err)
		}
	}

	return err
}

func (piper *Piper) Prepare() error {
	for modelName, model := range piper.models {
		metadataPath := filepath.Join(piper.modelPath, modelName, fmt.Sprintf("%s.metadata.json", modelName))
		data, err := ioutil.ReadFile(metadataPath)
		if err != nil {
			err = fmt.Errorf("failed to read voice metadata for model %s: %v", modelName, err)
			return util.TraceError(err)
		}

		var voices []engine.Voice
		if err := json.Unmarshal(data, &voices); err != nil {
			err = fmt.Errorf("failed to parse voice metadata for model %s: %v", modelName, err)
			return util.TraceError(err)
		}

		if err = model.command.Start(); err != nil {
			return util.TraceError(err)
		}

		piper.StartAudioCapture(model)

		format := Format
		if err = speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)); err != nil {
			return util.TraceError(err)
		}
	}

	return nil
}

func (piper *Piper) Play(message util.CharacterMessage) error {
	response.Debug(response.Data{
		Summary: "Piper playing:" + message.Character,
		Detail:  message.Text,
	})

	if strings.HasPrefix(message.Character, "::") {

	}

	voice, err := voiceManager.GetInstance().GetVoice(message.Character, false)
	if err != nil {
		return util.TraceError(err)
	}

	speakerID, _ := strconv.Atoi(voice.Voice)

	input := PiperInputLite{
		Text:      strings.ReplaceAll(message.Text, `"`, `\"`),
		SpeakerID: speakerID,
	}

	jsonBytes, err := json.Marshal(input)
	if err != nil {
		return util.TraceError(err)
	}
	jsonBytes = append(jsonBytes, '\n')

	audioClip, err := piper.Generate(voice.Model, jsonBytes)
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

	for index, message := range messages {
		voice, err := voiceManager.GetInstance().GetVoice(message.Character, false)
		if err != nil {
			return util.TraceError(err)
		}

		speakerID, _ := strconv.Atoi(voice.Voice)

		input := PiperInput{
			Text:      strings.ReplaceAll(message.Text, `"`, `\"`),
			SpeakerID: speakerID,
			OutputFile: util.GenerateFilename(
				message,
				index,
				expandedPath,
			),
		}

		jsonBytes, err := json.Marshal(input)
		if err != nil {
			return util.TraceError(err)
		}
		jsonBytes = append(jsonBytes, '\n')

		audioClip, err := piper.Generate(voice.Model, jsonBytes)
		if err != nil {
			return util.TraceError(err)
		}

		if play {
			playRawAudioBytes(audioClip)
		}
	}

	return nil
}

func (piper *Piper) Generate(model string, jsonBytes []byte) ([]byte, error) {
	piper.initOnce.Do(func() {
		err := piper.Prepare()
		if err != nil {
			response.Error(response.Data{
				Summary: "Failed to prepare piper",
				Detail:  err.Error(),
			})
		}
	})

	if utf8.Valid(jsonBytes) == false {
		return nil, util.TraceError(fmt.Errorf("input JSON is not valid UTF-8"))
	}

	if _, err := piper.models[model].stdin.Write(jsonBytes); err != nil {
		return nil, util.TraceError(err)
	}

	endSignal := make(chan bool)
	go func() {
		scanner := bufio.NewScanner(piper.models[model].stderr)
		for scanner.Scan() {
			text := scanner.Text()
			fmt.Println(text)

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
		return nil, util.TraceError(fmt.Errorf("model %s does not exist", model))
	}
	return modelData.Voices, nil
}

//</editor-fold>

func (piper *Piper) InitializeModel(modelName string) error {
	var err error

	metadataPath := filepath.Join(piper.modelPath, modelName, fmt.Sprintf("%s.metadata.json", modelName))
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return util.TraceError(err)
	}

	var voices []engine.Voice
	fmt.Println(string(data))
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

	piper.models[modelName] = instance
	return nil
}

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
