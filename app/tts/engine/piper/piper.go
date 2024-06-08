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
	"strconv"
	"strings"
	"sync"
	"time"
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
	voices    []engine.Voice
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
	fmt.Println("Piper engine initializing")

	piperPathValue := config.GetInstance().GetSetting("piperPath")
	if piperPathValue.String == nil {
		err = errors.New("piperPath is nil")
		return err
	}
	fmt.Println("Piper path:", *piperPathValue.String)
	piper.piperPath = *piperPathValue.String

	modelPathValue := config.GetInstance().GetSetting("piperModelsDirectory")
	if modelPathValue.String == nil {
		err = errors.New("modelPath is nil")
		return err
	}
	fmt.Println("Model path:", *modelPathValue.String)
	piper.modelPath = *modelPathValue.String

	piper.models = make(map[string]VoiceSynthesizer)
	fmt.Println("About to initialize models")
	for _, model := range models {
		fmt.Println(model)
		err := piper.InitializeModel(model)
		if err != nil {
			return err
		}
	}

	return err
}

func (piper *Piper) Prepare() error {
	fmt.Println("Piper prepare launched")

	metadataPath := piper.modelPath + "\\libritts\\libritts.metadata.json"
	data, err := ioutil.ReadFile(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to read voice metadata: %v", err)
	}
	fmt.Println("read from metadata")
	fmt.Println(string(data))
	var voices []engine.Voice
	if err := json.Unmarshal(data, &voices); err != nil {
		fmt.Println("didn't get to unmarshal voices")
		fmt.Println(err)
		return fmt.Errorf("failed to parse voice metadata: %v", err)
	}
	fmt.Println("unmarshaled voices")
	// Command preparation
	//TODO: Loop through models and start them as needed. start all for now
	err = piper.models["libritts"].command.Start()
	if err != nil {
		return err
	}

	// Start capturing audio
	piper.StartAudioCapture(piper.models["libritts"])

	// Initialize speaker
	format := Format
	if err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)); err != nil {
		return err
	}
	fmt.Println("Done preparing piper")
	return nil
}

func (piper *Piper) Play(message util.CharacterMessage) error {
	piper.initOnce.Do(func() {
		err := piper.Initialize([]string{"libritts"})
		if err != nil {
			return
		}
		err = piper.Prepare()
		if err != nil {
			return
		}
	})
	fmt.Printf("Piper playing: Character=%s, Message=%s\n", message.Character, message.Text)

	voice := voiceManager.GetInstance().GetVoice(message.Character)
	fmt.Println("Voice")
	fmt.Println(voice)

	speakerID, _ := strconv.Atoi(voice.Voice)

	input := PiperInputLite{
		Text:      strings.ReplaceAll(message.Text, `"`, `\"`),
		SpeakerID: speakerID,
	}

	jsonBytes, err := json.Marshal(input)
	if err != nil {
		return err
	}
	jsonBytes = append(jsonBytes, '\n')
	fmt.Println(string(jsonBytes))
	if _, err := piper.models["libritts"].stdin.Write(jsonBytes); err != nil {
		return err
	}

	endSignal := make(chan bool)
	go func() {
		scanner := bufio.NewScanner(piper.models["libritts"].stderr)
		fmt.Println("About to start scanning")
		for scanner.Scan() {
			text := scanner.Text()
			fmt.Println("sdterr text")
			fmt.Println(text)
			if strings.HasSuffix(text, " sec)") {
				endSignal <- true
				return
			}
		}
	}()
	<-endSignal

	audioBytes := piper.models["libritts"].audioData.buffer.Bytes()
	audioClip := make([]byte, len(audioBytes))
	copy(audioClip, audioBytes)
	fmt.Println("AudioBytes")
	fmt.Println(len(audioBytes))

	if err := playRawAudioBytes(audioClip); err != nil {
		return err
	}
	piper.models["libritts"].audioData.Reset()
	return nil
}

func (piper *Piper) GetVoices(model string) ([]engine.Voice, error) {
	fmt.Println("in GetVoices")
	fmt.Println(model)
	fmt.Println(piper.models)
	fmt.Println(piper.models[model])
	modelData, exists := piper.models[model]
	if !exists {
		return nil, fmt.Errorf("model %s does not exist", model)
	}
	return modelData.voices, nil
}

//</editor-fold>

func (piper *Piper) InitializeModel(model string) error {
	metadata := piper.modelPath + "\\" + model + "\\" + model + ".metadata.json"
	data, err := os.ReadFile(metadata)
	if err != nil {
		return fmt.Errorf("failed to read voice metadata: %v", err)
	}
	fmt.Println("read from metadata")
	fmt.Println(string(data))

	var voices []engine.Voice
	if err := json.Unmarshal(data, &voices); err != nil {
		fmt.Println("didn't get to unmarshal voices")
		fmt.Println(err)
		return fmt.Errorf("failed to parse voice metadata: %v", err)
	}
	fmt.Println("unmarshaled voices")

	cmdArgs := []string{"--model", piper.modelPath + "\\" + model + "\\" + model + ".onnx", "--json-input", "--output-raw"}
	command := exec.Command(piper.piperPath, cmdArgs...)
	fmt.Println("Executing command: %s %s\n", command.Path, strings.Join(command.Args[1:], " "))

	instance := VoiceSynthesizer{
		command:   command,
		audioData: &AudioBuffer{},
		voices:    voices,
	}

	instance.stdin, err = instance.command.StdinPipe()
	if err != nil {
		return err
	}

	instance.stderr, err = instance.command.StderrPipe()
	if err != nil {
		return err
	}

	instance.stdout, err = instance.command.StdoutPipe() // Capture stdout
	if err != nil {
		return err
	}

	fmt.Println("Adding model to list:")
	fmt.Println(model)
	fmt.Println(instance)
	piper.models[model] = instance
	return nil
}

func (piper *Piper) StartAudioCapture(instance VoiceSynthesizer) {
	fmt.Println("Capturing audio")
	//instance.audioData = &AudioBuffer{}

	go func() {
		fmt.Println("should be copying stuff from stdout to audioData buffer")
		_, err := io.Copy(instance.audioData, instance.stdout)
		if err != nil {
			// Handle the error. Note: This might happen if the stream ends or an unexpected error occurs.
			fmt.Printf("Error during audio capture: %v\n", err)
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
