package piper

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gopxl/beep"
	"github.com/gopxl/beep/speaker"
	"io"
	"io/ioutil"
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
		err = errors.New("Piper:Initialize:piperPathValue: is nil")
		return err
	}
	err, piper.piperPath = util.ExpandPath(*piperPathValue.String)
	if err != nil {
		err = errors.New("Piper:Initialize:piperPath: " + err.Error())
		return err
	}

	modelPathValue := config.GetInstance().GetSetting("piperModelsDirectory")
	if modelPathValue.String == nil {
		err = errors.New("Piper:Initialize:modelPathValue: is nil")
		return err
	}

	err, piper.modelPath = util.ExpandPath(*modelPathValue.String)
	if err != nil {
		err = errors.New("Piper:Initialize:modelPath: " + err.Error())
		return err
	}

	piper.models = make(map[string]VoiceSynthesizer)
	for _, model := range models {
		err := piper.InitializeModel(model)
		if err != nil {
			return err
		}
	}

	return err
}

func (piper *Piper) Prepare() error {
	//metadataPath := piper.modelPath + "\\libritts\\libritts.metadata.json"
	//data, err := ioutil.ReadFile(metadataPath)
	//if err != nil {
	//	return fmt.Errorf("failed to read voice metadata: %v", err)
	//}
	//
	//var voices []engine.Voice
	//if err := json.Unmarshal(data, &voices); err != nil {
	//	return fmt.Errorf("failed to parse voice metadata: %v", err)
	//}
	//
	////TODO: Loop through models and start them as needed. start all for now
	//err = piper.models["libritts"].command.Start()
	//if err != nil {
	//	return err
	//}
	//
	//piper.StartAudioCapture(piper.models["libritts"])
	//
	//format := Format
	//if err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)); err != nil {
	//	return err
	//}
	//return nil
	for modelName, model := range piper.models {
		metadataPath := filepath.Join(piper.modelPath, modelName, fmt.Sprintf("%s.metadata.json", modelName))
		data, err := ioutil.ReadFile(metadataPath)
		if err != nil {
			err = fmt.Errorf("failed to read voice metadata for model %s: %v", modelName, err)
			return err
		}

		var voices []engine.Voice
		if err := json.Unmarshal(data, &voices); err != nil {
			err = fmt.Errorf("failed to parse voice metadata for model %s: %v", modelName, err)
			return err
		}

		if err = model.command.Start(); err != nil {
			return err
		}

		piper.StartAudioCapture(model)

		format := Format
		if err = speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)); err != nil {
			return err
		}
	}

	return nil
}

func (piper *Piper) Play(message util.CharacterMessage) error {
	fmt.Printf("Piper playing: Character=%s, Message=%s\n", message.Character, message.Text)

	if strings.HasPrefix(message.Character, "::") {

	}

	voice := voiceManager.GetInstance().GetVoice(message.Character, false)

	speakerID, _ := strconv.Atoi(voice.Voice)

	input := PiperInputLite{
		Text:      strings.ReplaceAll(message.Text, `"`, `\"`),
		SpeakerID: speakerID,
	}

	jsonBytes, err := json.Marshal(input)
	if err != nil {
		return util.GenerateError(err, "Failed to marshal input")
	}
	jsonBytes = append(jsonBytes, '\n')

	audioClip, err := piper.Generate(voice.Model, jsonBytes)
	if err != nil {
		return err
	}

	if err := playRawAudioBytes(audioClip); err != nil {
		return err
	}

	return nil
}

func (piper *Piper) Save(messages []util.CharacterMessage, play bool) error {

	for index, message := range messages {

		voice := voiceManager.GetInstance().GetVoice(message.Character, false)

		speakerID, _ := strconv.Atoi(voice.Voice)

		input := PiperInput{
			Text:       strings.ReplaceAll(message.Text, `"`, `\"`),
			SpeakerID:  speakerID,
			OutputFile: util.GenerateFilename(message, index),
		}
		fmt.Println("Piper Input")
		fmt.Println(input)

		jsonBytes, err := json.Marshal(input)
		if err != nil {
			return util.GenerateError(err, "Failed to marshal input")
		}
		jsonBytes = append(jsonBytes, '\n')

		audioClip, err := piper.Generate(voice.Model, jsonBytes)
		if err != nil {
			return err
		}

		if play {
			if err := playRawAudioBytes(audioClip); err != nil {
				return err
			}
		}
	}

	return nil
}

func (piper *Piper) Generate(model string, jsonBytes []byte) ([]byte, error) {
	piper.initOnce.Do(func() {
		err := piper.Prepare()
		if err != nil {
			return
		}
	})

	if utf8.Valid(jsonBytes) == false {
		return nil, errors.New("input JSON is not valid UTF-8")
	}

	fmt.Println("About to send to engine's stdin")
	//fmt.Println(model)
	//fmt.Println(piper.models[model])
	if _, err := piper.models[model].stdin.Write(jsonBytes); err != nil {
		return nil, err
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
		return nil, fmt.Errorf("model %s does not exist", model)
	}
	return modelData.Voices, nil
}

//</editor-fold>

func (piper *Piper) InitializeModel(modelName string) error {
	var err error
	modelPath := piper.modelPath
	fmt.Println("Should start initializing")

	if runtime.GOOS == "darwin" {
		err, modelPath = util.ExpandPath(modelPath)
		if err != nil {
			return err
		}
	}

	metadataPath := filepath.Join(modelPath, modelName, fmt.Sprintf("%s.metadata.json", modelName))
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to read voice metadata: %v", err)
	}

	fmt.Println("Continuing initialization")

	var voices []engine.Voice
	fmt.Println(string(data))
	if err := json.Unmarshal(data, &voices); err != nil {
		return fmt.Errorf("failed to parse voice metadata: %v", err)
	}

	onnxPath := filepath.Join(modelPath, modelName, fmt.Sprintf("%s.onnx", modelName))

	cmdArgs := []string{"--model", onnxPath, "--json-input", "--output-raw"}
	command := exec.Command(piper.piperPath, cmdArgs...)
	fmt.Println("Preparing command: %s %s\n", command.Path, strings.Join(command.Args[1:], " "))

	instance := VoiceSynthesizer{
		command:   command,
		audioData: &AudioBuffer{},
		Voices:    voices,
	}

	instance.stdin, err = instance.command.StdinPipe()
	if err != nil {
		return err
	}

	instance.stderr, err = instance.command.StderrPipe()
	if err != nil {
		return err
	}

	instance.stdout, err = instance.command.StdoutPipe()
	if err != nil {
		return err
	}

	piper.models[modelName] = instance
	return nil
}

func (piper *Piper) StartAudioCapture(instance VoiceSynthesizer) {
	go func() {
		_, err := io.Copy(instance.audioData, instance.stdout)
		if err != nil {
			panic("Error during audio capture: " + err.Error())
		}
	}()
}

func playRawAudioBytes(audioClip []byte) error {
	done := make(chan struct{})
	audioDataReader := bytes.NewReader(audioClip)

	streamer := beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		for i := range samples {
			if audioDataReader.Len() < 2 { // 2 bytes needed for one sample
				close(done) // Signal that streaming is done
				return i, false
			}
			var sample int16
			// Correctly handle little endian for the sample
			err := binary.Read(audioDataReader, binary.LittleEndian, &sample)
			if err != nil {
				close(done) // In case of read error, signal to stop
				return i, false
			}
			flSample := float64(sample) / (1 << 15)
			samples[i][0] = flSample // Mono to left channel
			samples[i][1] = flSample // Mono to right channel
		}
		return len(samples), true
	})

	speaker.Play(streamer)

	// Wait for the audio to finish playing
	<-done

	// Optionally, ensure that all audio has been played out
	time.Sleep(100 * time.Millisecond)
	return nil
}

//func (ss *VoiceSynthesizer) Stop() error {
//	if err := ss.stdin.Close(); err != nil {
//		return err
//	}
//	return ss.command.Wait()
//}
