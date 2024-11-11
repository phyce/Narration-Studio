package openai

import (
	"bytes"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/gopxl/beep"
	beepFlac "github.com/gopxl/beep/flac"
	"github.com/gopxl/beep/speaker"
	"github.com/mewkiz/flac"
	"io"
	"nstudio/app/common/response"
	"nstudio/app/common/util"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
	"os"
)

type OpenAI struct {
	Models     map[string]Model
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

// <editor-fold desc="Engine Interface">
func (openAI *OpenAI) Initialize() error {
	//openAI.outputType = *config.GetSetting("openAiOutputType").String
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

	input := OpenAIRequest{
		Voice:          message.Voice.Voice,
		Input:          message.Text,
		Model:          message.Voice.Model,
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
	response.Debug(response.Data{
		Summary: "Openai saving messages",
	})

	err, expandedPath := util.ExpandPath(*config.GetSetting("scriptOutputPath").String)
	if err != nil {
		return util.TraceError(err)
	}

	for _, message := range messages {
		input := OpenAIRequest{
			Voice:          message.Voice.Voice,
			Input:          message.Text,
			Model:          message.Voice.Model,
			ResponseFormat: openAI.outputType,
			Speed:          1,
		}

		audioClip, err := openAI.sendRequest(input)
		if err != nil {
			return util.TraceError(err)
		}

		filename := util.GenerateFilename(
			message,
			util.FileIndexGet(),
			expandedPath,
		)

		err = saveWavFile(audioClip, filename)
		if err != nil {
			return util.TraceError(err)
		}

		if play {
			err = playFLACAudioBytes(audioClip)
			if err != nil {
				return util.TraceError(err)
			}
		}
	}

	return nil
}

func (openAI *OpenAI) Generate(model string, payload []byte) ([]byte, error) {
	return make([]byte, 0), nil
}

func (openAI *OpenAI) GetVoices(model string) ([]engine.Voice, error) {
	return voices, nil
}

func (openAI *OpenAI) FetchModels() map[string]engine.Model {
	if getApiKey() == "" {
		return make(map[string]engine.Model)
	}

	return FetchModels()
}

// </editor-fold>

// <editor-fold desc="Other">
func playFLACAudioBytes(audioClip []byte) error {
	audioReader := io.NopCloser(bytes.NewReader(audioClip))

	streamer, format, err := beepFlac.Decode(audioReader)
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

func saveWavFile(flacData []byte, filename string) error {
	// Create a bytes.Reader from the FLAC data
	reader := bytes.NewReader(flacData)

	// Decode the FLAC data
	stream, err := flac.New(reader)
	if err != nil {
		return util.TraceError(err)
	}

	// Prepare an audio buffer
	var buf audio.IntBuffer
	buf.Format = &audio.Format{
		NumChannels: int(stream.Info.NChannels),
		SampleRate:  int(stream.Info.SampleRate),
	}

	for {
		frame, err := stream.ParseNext()
		if err == io.EOF {
			break
		}
		if err != nil {
			return util.TraceError(err)
		}
		// Append the samples from each subframe to the buffer
		for _, subframe := range frame.Subframes {
			for _, sample := range subframe.Samples {
				buf.Data = append(buf.Data, int(sample))
			}
		}
	}

	// Create the output WAV file
	outFile, err := os.Create(filename)
	if err != nil {
		return util.TraceError(err)
	}
	defer outFile.Close()

	// Encode the buffer as WAV
	encoder := wav.NewEncoder(outFile, buf.Format.SampleRate, int(stream.Info.BitsPerSample), buf.Format.NumChannels, 1)
	if err := encoder.Write(&buf); err != nil {
		return util.TraceError(err)
	}
	if err := encoder.Close(); err != nil {
		return util.TraceError(err)
	}

	return nil
}

func FetchModels() map[string]engine.Model {
	return map[string]engine.Model{
		"tts-1": {
			ID:     "tts-1",
			Name:   "TTS-1",
			Engine: "openai",
		},
		"tts-1-hd": {
			ID:     "tts-1-hd",
			Name:   "TTS-1 HD",
			Engine: "openai",
		},
	}
}

// </editor-fold>
