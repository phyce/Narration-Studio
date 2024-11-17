package audio

import (
	"fmt"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"nstudio/app/common/issue"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func CombineWavFiles(dirPath, outputFilename string, pauseDuration time.Duration, sampleRate, numChannels, bitDepth int) error {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	var wavFiles []string
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".wav" {
			wavFiles = append(wavFiles, filepath.Join(dirPath, file.Name()))
		}
	}

	if len(wavFiles) == 0 {
		return issue.Trace(fmt.Errorf("no WAV files found in directory"))
	}

	sort.Strings(wavFiles)

	var combinedBuffer *audio.IntBuffer

	silenceSamples := int(float64(pauseDuration.Seconds()) * float64(sampleRate))
	silenceData := make([]int, silenceSamples*numChannels)
	silenceBuffer := &audio.IntBuffer{
		Data: silenceData,
		Format: &audio.Format{
			NumChannels: numChannels,
			SampleRate:  sampleRate,
		},
		SourceBitDepth: bitDepth,
	}

	for idx, wavPath := range wavFiles {
		file, err := os.Open(wavPath)
		if err != nil {
			return err
		}

		decoder := wav.NewDecoder(file)
		if !decoder.IsValidFile() {
			file.Close()
			return issue.Trace(fmt.Errorf("invalid WAV file: " + wavPath))
		}

		buf, err := decoder.FullPCMBuffer()
		if err != nil {
			file.Close()
			return err
		}
		file.Close()

		if buf.Format.SampleRate != sampleRate {
			buf, err = ResampleBuffer(buf, sampleRate)
			if err != nil {
				return err
			}
		}

		if buf.Format.NumChannels != numChannels {
			buf, err = ChangeNumChannels(buf, numChannels)
			if err != nil {
				return err
			}
		}

		if buf.SourceBitDepth != bitDepth {
			buf, err = ChangeBitDepth(buf, bitDepth)
			if err != nil {
				return err
			}
		}

		if idx == 0 {
			combinedBuffer = &audio.IntBuffer{
				Data:           []int{},
				Format:         buf.Format,
				SourceBitDepth: buf.SourceBitDepth,
			}
		}

		combinedBuffer.Data = append(combinedBuffer.Data, buf.Data...)

		if idx < len(wavFiles)-1 {
			combinedBuffer.Data = append(combinedBuffer.Data, silenceBuffer.Data...)
		}
	}

	outputPath := filepath.Join(dirPath, outputFilename)
	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	encoder := wav.NewEncoder(outFile, sampleRate, bitDepth, numChannels, 1)
	err = encoder.Write(combinedBuffer)
	if err != nil {
		return err
	}

	err = encoder.Close()
	if err != nil {
		return err
	}

	return nil
}

func ResampleBuffer(buf *audio.IntBuffer, targetSampleRate int) (*audio.IntBuffer, error) {
	srcSampleRate := buf.Format.SampleRate
	if srcSampleRate == targetSampleRate {
		return buf, nil
	}

	resampleRatio := float64(targetSampleRate) / float64(srcSampleRate)
	srcData := buf.Data
	srcLength := len(srcData)
	dstLength := int(float64(srcLength) * resampleRatio)
	dstData := make([]int, dstLength)

	for i := 0; i < dstLength; i++ {
		srcPos := float64(i) / resampleRatio
		srcIndex := int(srcPos)
		if srcIndex >= srcLength-1 {
			srcIndex = srcLength - 2
		}
		frac := srcPos - float64(srcIndex)
		sample := int(float64(srcData[srcIndex])*(1-frac) + float64(srcData[srcIndex+1])*frac)
		dstData[i] = sample
	}

	resampledBuf := &audio.IntBuffer{
		Data:           dstData,
		Format:         &audio.Format{SampleRate: targetSampleRate, NumChannels: buf.Format.NumChannels},
		SourceBitDepth: buf.SourceBitDepth,
	}
	return resampledBuf, nil
}

func ChangeNumChannels(buf *audio.IntBuffer, targetNumChannels int) (*audio.IntBuffer, error) {
	srcNumChannels := buf.Format.NumChannels
	if srcNumChannels == targetNumChannels {
		return buf, nil
	}
	srcData := buf.Data
	numSamples := len(srcData) / srcNumChannels
	dstData := make([]int, numSamples*targetNumChannels)

	if targetNumChannels == 1 && srcNumChannels > 1 {
		for i := 0; i < numSamples; i++ {
			sum := 0
			for c := 0; c < srcNumChannels; c++ {
				sum += srcData[i*srcNumChannels+c]
			}
			avg := sum / srcNumChannels
			dstData[i] = avg
		}
	} else if targetNumChannels > 1 && srcNumChannels == 1 {
		for i := 0; i < numSamples; i++ {
			sample := srcData[i]
			for c := 0; c < targetNumChannels; c++ {
				dstData[i*targetNumChannels+c] = sample
			}
		}
	} else {
		return nil, issue.Trace(fmt.Errorf("unsupported channel conversion"))
	}

	convertedBuf := &audio.IntBuffer{
		Data:           dstData,
		Format:         &audio.Format{SampleRate: buf.Format.SampleRate, NumChannels: targetNumChannels},
		SourceBitDepth: buf.SourceBitDepth,
	}
	return convertedBuf, nil
}

func ChangeBitDepth(buf *audio.IntBuffer, targetBitDepth int) (*audio.IntBuffer, error) {
	srcBitDepth := buf.SourceBitDepth
	if srcBitDepth == targetBitDepth {
		return buf, nil
	}

	srcData := buf.Data
	dstData := make([]int, len(srcData))

	maxSrcValue := 1 << (srcBitDepth - 1)
	maxDstValue := 1 << (targetBitDepth - 1)

	for i, sample := range srcData {
		scaledSample := sample * maxDstValue / maxSrcValue
		dstData[i] = scaledSample
	}

	convertedBuf := &audio.IntBuffer{
		Data:           dstData,
		Format:         buf.Format,
		SourceBitDepth: targetBitDepth,
	}
	return convertedBuf, nil
}
