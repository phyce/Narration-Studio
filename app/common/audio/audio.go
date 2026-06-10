package audio

import (
	"nstudio/app/common/response"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

func CombineWAVFiles(dirPath, outputFilename string, pauseDuration time.Duration, sampleRate, channelCount, bitDepth int) error {
	wavFiles, err := filepath.Glob(filepath.Join(dirPath, "*.wav"))
	if err != nil {
		return response.Err("Failed to list WAV files: %v", err)
	}
	if len(wavFiles) == 0 {
		return response.Err("No WAV files found in the directory")
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
			return response.Err("%w", err)
		}

		decoder := wav.NewDecoder(file)
		if !decoder.IsValidFile() {
			return response.Err("Invalid WAV file: %s", wavPath)
		}

		pcmBuffer, err := decoder.FullPCMBuffer()
		if err != nil {
			return response.Err("%w", err)
		}

		if pcmBuffer.Format.SampleRate != sampleRate {
			pcmBuffer, err = ResampleBuffer(pcmBuffer, sampleRate)
			if err != nil {
				response.Err("%w", err)
			}
		}

		if pcmBuffer.Format.NumChannels != channelCount {
			pcmBuffer, err = ChangeChannelCount(pcmBuffer, channelCount)
			if err != nil {
				response.Err("%w", err)
			}
		}

		if pcmBuffer.SourceBitDepth != bitDepth {
			pcmBuffer, err = ChangeBitDepth(pcmBuffer, bitDepth)
			if err != nil {
				response.Err("%w", err)
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
		response.Err("%w", err)
	}
	defer combinedFile.Close()

	encoder := wav.NewEncoder(combinedFile, sampleRate, bitDepth, channelCount, 1)
	err = encoder.Write(combinedBuffer)
	if err != nil {
		response.Err("%w", err)
	}

	err = encoder.Close()
	if err != nil {
		response.Err("%w", err)
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
		return nil, response.Err("Unsupported channel conversion")
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
