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
	"nstudio/app/config"
	"nstudio/app/tts/util"
	"nstudio/app/tts/voiceManager"
	"os/exec"
	"strings"
	"sync"
	"time"
)

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

type SpeechSynthesizer struct {
	//modelsDirectory string
	//piperPath       string
	command   *exec.Cmd
	stdin     io.WriteCloser
	stderr    io.ReadCloser
	stdout    io.ReadCloser
	audioData *AudioBuffer
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
	models    map[string]SpeechSynthesizer
	piperPath string
	modelPath string
	initOnce  sync.Once
}

func (piper *Piper) Initialize() error {
	var err error

	fmt.Println("Piper engine initializing")

	// Retrieve piperPath setting
	piperPathValue := config.GetInstance().GetSetting("piperPath")
	if piperPathValue.String == nil {
		err = errors.New("piperPath is nil")
		return err
	}
	fmt.Println("Piper path:", *piperPathValue.String)
	piper.piperPath = *piperPathValue.String

	// Retrieve modelPath setting
	modelPathValue := config.GetInstance().GetSetting("piperModelsDirectory")
	if modelPathValue.String == nil {
		err = errors.New("modelPath is nil")
		return err
	}
	fmt.Println("Model path:", *modelPathValue.String)
	piper.modelPath = *modelPathValue.String

	piper.models = make(map[string]SpeechSynthesizer)

	return err
}

func (piper *Piper) Prepare() error {
	fmt.Println("Piper prepare launched")

	// Command preparation
	//TODO: get appropriate model name
	cmdArgs := []string{"--model", piper.modelPath + "\\libritts\\libritts.onnx", "--json-input", "--output-raw"}
	command := exec.Command(piper.piperPath, cmdArgs...)
	fmt.Printf("Executing command: %s %s\n", command.Path, strings.Join(command.Args[1:], " "))

	instance := SpeechSynthesizer{
		command:   command,
		audioData: &AudioBuffer{},
	}

	// TODO this will need to load all the models based on user's choices
	var err error
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

	piper.models["libritts"] = instance

	err = instance.command.Start()
	if err != nil {
		return err
	}

	// Start capturing audio
	piper.StartAudioCapture(instance)

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
		err := piper.Initialize()
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

	input := PiperInputLite{
		Text:      strings.ReplaceAll(message.Text, `"`, `\"`),
		SpeakerID: voice.Voice,
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

func (piper *Piper) StartAudioCapture(instance SpeechSynthesizer) {
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

//func (synth *SpeechSynthesizer) Prepare() {
//	synth.audioData = &AudioBuffer{}
//
//	go func() {
//		_, err := io.Copy(synth.audioData, synth.stdout)
//		if err != nil {
//			fmt.Printf("Error during audio capture: %v\n", err)
//		}
//	}()
//}

//func (ss *SpeechSynthesizer) Stop() error {
//	if err := ss.stdin.Close(); err != nil {
//		return err
//	}
//	return ss.command.Wait()
//}
