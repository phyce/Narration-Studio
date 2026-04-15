//go:build !clib

package audio

import (
	"bytes"
	"encoding/binary"
	"io"
	"nstudio/app/common/response"
	"nstudio/app/common/util"
	"os"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/gopxl/beep"
	beepFlac "github.com/gopxl/beep/flac"
	beepMp3 "github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
	"github.com/mewkiz/flac"
)

func PlayPCMAudioBytes(audioClip []byte) error {
	audioDataReader := bytes.NewReader(audioClip)

	//TODO add ability to change format details
	originalFormat := beep.Format{
		SampleRate:  24000,
		NumChannels: 1,
		Precision:   2,
	}

	streamer := beep.StreamerFunc(func(samples [][2]float64) (processedSamples int, ok bool) {
		for sampleIndex := range samples {
			if audioDataReader.Len() < 2 {
				return sampleIndex, false
			}

			var sample int16
			err := binary.Read(audioDataReader, binary.LittleEndian, &sample)
			if err != nil {
				response.Error(util.MessageData{
					Summary: "Error reading PCM data",
					Detail:  err.Error(),
				})
				return sampleIndex, false
			}

			floatSample := float64(sample) / (1 << 15)

			samples[sampleIndex][0] = floatSample
			samples[sampleIndex][1] = floatSample

			processedSamples++
		}
		return len(samples), true
	})

	resampler := beep.Resample(4, originalFormat.SampleRate, beep.SampleRate(48000), streamer)

	done := make(chan bool)

	speaker.Play(beep.Seq(resampler, beep.Callback(func() {
		done <- true
	})))

	<-done

	return nil
}

func PlayRawAudioBytes(audioClip []byte) {
	done := make(chan struct{})
	audioDataReader := bytes.NewReader(audioClip)

	streamer := beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		for i := range samples {
			if audioDataReader.Len() < 2 {
				return i, false
			}
			var sample int16

			err := binary.Read(audioDataReader, binary.LittleEndian, &sample)
			if err != nil {
				return i, false
			}
			flSample := float64(sample) / (1 << 15)
			samples[i][0] = flSample
			samples[i][1] = flSample
		}
		return len(samples), true
	})

	resampledStreamer := beep.Resample(4, 22050, 48000, streamer)

	speaker.Play(beep.Seq(resampledStreamer, beep.Callback(func() {
		close(done)
	})))

	<-done
}

func PlayFLACAudioBytes(audioClip []byte) error {
	audioReader := io.NopCloser(bytes.NewReader(audioClip))

	streamer, format, err := beepFlac.Decode(audioReader)
	if err != nil {
		return err
	}
	defer streamer.Close()

	sampleRate := beep.SampleRate(48000)

	resampled := beep.Resample(4, format.SampleRate, sampleRate, streamer)

	done := make(chan bool)
	speaker.Play(beep.Seq(resampled, beep.Callback(func() {
		done <- true
	})))

	<-done

	return nil
}

func PlayMP3AudioBytes(audioClip []byte) error {
	audioReader := io.NopCloser(bytes.NewReader(audioClip))

	streamer, format, err := beepMp3.Decode(audioReader)
	if err != nil {
		return err
	}
	defer streamer.Close()

	sampleRate := beep.SampleRate(48000)

	resampled := beep.Resample(4, format.SampleRate, sampleRate, streamer)

	done := make(chan bool)
	speaker.Play(beep.Seq(resampled, beep.Callback(func() {
		done <- true
	})))

	<-done

	return nil
}

func SaveFLACAsWAV(flacAudioClip []byte, filename string) error {
	reader := bytes.NewReader(flacAudioClip)

	stream, err := flac.New(reader)
	if err != nil {
		return response.Err(err)
	}

	var buffer audio.IntBuffer
	buffer.Format = &audio.Format{
		NumChannels: int(stream.Info.NChannels),
		SampleRate:  int(stream.Info.SampleRate),
	}

	for {
		frame, err := stream.ParseNext()
		if err == io.EOF {
			break
		}
		if err != nil {
			return response.Err(err)
		}
		for _, subframe := range frame.Subframes {
			for _, sample := range subframe.Samples {
				buffer.Data = append(buffer.Data, int(sample))
			}
		}
	}

	outputFile, err := os.Create(filename)
	if err != nil {
		return response.Err(err)
	}
	defer outputFile.Close()

	encoder := wav.NewEncoder(outputFile, buffer.Format.SampleRate, int(stream.Info.BitsPerSample), buffer.Format.NumChannels, 1)
	if err := encoder.Write(&buffer); err != nil {
		return response.Err(err)
	}
	if err := encoder.Close(); err != nil {
		return response.Err(err)
	}

	return nil
}
