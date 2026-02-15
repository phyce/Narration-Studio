package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"nstudio/app/common/response"
	"nstudio/app/common/util"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/gopxl/beep"
	beepFlac "github.com/gopxl/beep/flac"
	beepMp3 "github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
	"github.com/mewkiz/flac"
)

func CombineWAVFiles(dirPath, outputFilename string, pauseDuration time.Duration, sampleRate, channelCount, bitDepth int) error {
	wavFiles, err := filepath.Glob(filepath.Join(dirPath, "*.wav"))
	if err != nil {
		return response.Err(fmt.Errorf("Failed to list WAV files: %v", err))
	}
	if len(wavFiles) == 0 {
		return response.Err(fmt.Errorf("No WAV files found in the directory"))
	}

	sort.Strings(wavFiles)

	var combinedBuffer *audio.IntBuffer

	silenceSamples := int(float64(pauseDuration.Seconds()) * float64(sampleRate))
	silenceData := make([]int, silenceSamples*channelCount)
	silenceBuffer := &audio.IntBuffer{
		Data: silenceData,
		Format: &audio.Format{
			NumChannels: channelCount,
			SampleRate:  sampleRate,
		},
		SourceBitDepth: bitDepth,
	}

	for index, wavPath := range wavFiles {
		file, err := os.Open(wavPath)
		defer file.Close()
		if err != nil {
			return err
		}

		decoder := wav.NewDecoder(file)
		if !decoder.IsValidFile() {
			return response.Err(fmt.Errorf("Invalid WAV file: " + wavPath))
		}

		pcmBuffer, err := decoder.FullPCMBuffer()
		if err != nil {
			return response.Err(err)
		}

		if pcmBuffer.Format.SampleRate != sampleRate {
			pcmBuffer, err = ResampleBuffer(pcmBuffer, sampleRate)
			if err != nil {
				response.Err(err)
			}
		}

		if pcmBuffer.Format.NumChannels != channelCount {
			pcmBuffer, err = ChangeChannelCount(pcmBuffer, channelCount)
			if err != nil {
				response.Err(err)
			}
		}

		if pcmBuffer.SourceBitDepth != bitDepth {
			pcmBuffer, err = ChangeBitDepth(pcmBuffer, bitDepth)
			if err != nil {
				response.Err(err)
			}
		}

		if index == 0 {
			combinedBuffer = &audio.IntBuffer{
				Data:           []int{},
				Format:         pcmBuffer.Format,
				SourceBitDepth: pcmBuffer.SourceBitDepth,
			}
		}

		combinedBuffer.Data = append(combinedBuffer.Data, pcmBuffer.Data...)

		if index < len(wavFiles)-1 {
			combinedBuffer.Data = append(combinedBuffer.Data, silenceBuffer.Data...)
		}
	}

	outputPath := filepath.Join(dirPath, outputFilename)
	combinedFile, err := os.Create(outputPath)
	if err != nil {
		response.Err(err)
	}
	defer combinedFile.Close()

	encoder := wav.NewEncoder(combinedFile, sampleRate, bitDepth, channelCount, 1)
	err = encoder.Write(combinedBuffer)
	if err != nil {
		response.Err(err)
	}

	err = encoder.Close()
	if err != nil {
		response.Err(err)
	}

	return nil
}

func ResampleBuffer(buffer *audio.IntBuffer, targetSampleRate int) (*audio.IntBuffer, error) {
	sourceSampleRate := buffer.Format.SampleRate
	if sourceSampleRate == targetSampleRate {
		return buffer, nil
	}

	resampleRatio := float64(targetSampleRate) / float64(sourceSampleRate)
	sourceLength := len(buffer.Data)
	outputLength := int(float64(sourceLength) * resampleRatio)
	outputData := make([]int, outputLength)

	for i := 0; i < outputLength; i++ {
		sourcePosition := float64(i) / resampleRatio
		sourceIndex := int(sourcePosition)
		if sourceIndex >= sourceLength-1 {
			sourceIndex = sourceLength - 2
		}
		interpolationFraction := sourcePosition - float64(sourceIndex)
		sample := int(float64(buffer.Data[sourceIndex])*(1-interpolationFraction) + float64(buffer.Data[sourceIndex+1])*interpolationFraction)
		outputData[i] = sample
	}

	resampledBuffer := &audio.IntBuffer{
		Data:           outputData,
		Format:         &audio.Format{SampleRate: targetSampleRate, NumChannels: buffer.Format.NumChannels},
		SourceBitDepth: buffer.SourceBitDepth,
	}
	return resampledBuffer, nil
}

func ChangeChannelCount(buffer *audio.IntBuffer, targetChannelCount int) (*audio.IntBuffer, error) {
	sourceChannelCount := buffer.Format.NumChannels
	if sourceChannelCount == targetChannelCount {
		return buffer, nil
	}
	sourceData := buffer.Data
	sampleCount := len(sourceData) / sourceChannelCount
	resultData := make([]int, sampleCount*targetChannelCount)

	if targetChannelCount == 1 && sourceChannelCount > 1 {
		for sampleIndex := 0; sampleIndex < sampleCount; sampleIndex++ {
			sum := 0
			for channelIndex := 0; channelIndex < sourceChannelCount; channelIndex++ {
				sum += sourceData[sampleIndex*sourceChannelCount+channelIndex]
			}
			avg := sum / sourceChannelCount
			resultData[sampleIndex] = avg
		}
	} else if targetChannelCount > 1 && sourceChannelCount == 1 {
		for sampleIndex := 0; sampleIndex < sampleCount; sampleIndex++ {
			sample := sourceData[sampleIndex]
			for channelIndex := 0; channelIndex < targetChannelCount; channelIndex++ {
				resultData[sampleIndex*targetChannelCount+channelIndex] = sample
			}
		}
	} else {
		return nil, response.Err(fmt.Errorf("Unsupported channel conversion"))
	}

	convertedBuf := &audio.IntBuffer{
		Data:           resultData,
		Format:         &audio.Format{SampleRate: buffer.Format.SampleRate, NumChannels: targetChannelCount},
		SourceBitDepth: buffer.SourceBitDepth,
	}
	return convertedBuf, nil
}

func ChangeBitDepth(buffer *audio.IntBuffer, targetBitDepth int) (*audio.IntBuffer, error) {
	sourceBitDepth := buffer.SourceBitDepth
	if sourceBitDepth == targetBitDepth {
		return buffer, nil
	}

	sourceData := buffer.Data
	resultData := make([]int, len(sourceData))

	maxSourceValue := 1 << (sourceBitDepth - 1)
	maxResultValue := 1 << (targetBitDepth - 1)

	for index, sample := range sourceData {
		scaledSample := sample * maxResultValue / maxSourceValue
		resultData[index] = scaledSample
	}

	convertedBuffer := &audio.IntBuffer{
		Data:           resultData,
		Format:         buffer.Format,
		SourceBitDepth: targetBitDepth,
	}
	return convertedBuffer, nil
}

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

func SaveWAVFile(audioClip []byte, filename string) error {
	sampleRate := 24000
	channelCount := 1
	bitDepth := 16
	bytesPerSample := bitDepth / 8

	sampleCount := len(audioClip) / bytesPerSample

	buffer := &audio.IntBuffer{
		Format: &audio.Format{
			SampleRate:  sampleRate,
			NumChannels: channelCount,
		},
		Data:           make([]int, sampleCount),
		SourceBitDepth: bitDepth,
	}

	reader := bytes.NewReader(audioClip)
	for i := 0; i < sampleCount; i++ {
		var sample int16
		if err := binary.Read(reader, binary.LittleEndian, &sample); err != nil {
			return err
		}
		buffer.Data[i] = int(sample)
	}

	outputFile, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	encoder := wav.NewEncoder(outputFile, buffer.Format.SampleRate, buffer.SourceBitDepth, buffer.Format.NumChannels, 1)
	defer encoder.Close()

	if err := encoder.Write(buffer); err != nil {
		return err
	}

	if err := encoder.Close(); err != nil {
		return err
	}

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

func SaveMP3(audioClip []byte, filename string) error {
	return os.WriteFile(filename, audioClip, 0644)
}