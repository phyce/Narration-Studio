package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"nstudio/app/common/response"
	"strings"

	"github.com/go-audio/audio"
	beepMp3 "github.com/gopxl/beep/mp3"
	"github.com/mewkiz/flac"
)

type AudioFormat string

const (
	FormatPCM  AudioFormat = "pcm"
	FormatFLAC AudioFormat = "flac"
	FormatMP3  AudioFormat = "mp3"
	FormatWAV  AudioFormat = "wav"
	FormatOGG  AudioFormat = "ogg"
)

type AudioMetadata struct {
	SampleRate int // e.g., 22050, 24000, 44100
	Channels   int
	BitDepth   int // 16, 24, 32 bits per sample
	Format     AudioFormat
}

type Audio struct {
	Data     []byte
	Metadata AudioMetadata
}

func NewAudioFromPCM(pcmData []byte, sampleRate, channels, bitDepth int) *Audio {
	return &Audio{
		Data: pcmData,
		Metadata: AudioMetadata{
			SampleRate: sampleRate,
			Channels:   channels,
			BitDepth:   bitDepth,
			Format:     FormatPCM,
		},
	}
}

func NewAudioFromFLAC(flacData []byte) *Audio {
	return &Audio{
		Data: flacData,
		Metadata: AudioMetadata{
			Format: FormatFLAC,
		},
	}
}

func NewAudioFromMP3(mp3Data []byte) *Audio {
	return &Audio{
		Data: mp3Data,
		Metadata: AudioMetadata{
			Format: FormatMP3,
		},
	}
}

func NewAudioFromWAV(wavData []byte) (*Audio, error) {
	if len(wavData) < 44 {
		return nil, response.Err(fmt.Errorf("invalid WAV file: too small"))
	}

	if string(wavData[0:4]) != "RIFF" || string(wavData[8:12]) != "WAVE" {
		return nil, response.Err(fmt.Errorf("invalid WAV file: missing RIFF/WAVE header"))
	}

	channels := int(binary.LittleEndian.Uint16(wavData[22:24]))
	sampleRate := int(binary.LittleEndian.Uint32(wavData[24:28]))
	bitDepth := int(binary.LittleEndian.Uint16(wavData[34:36]))

	return &Audio{
		Data: wavData,
		Metadata: AudioMetadata{
			SampleRate: sampleRate,
			Channels:   channels,
			BitDepth:   bitDepth,
			Format:     FormatWAV,
		},
	}, nil
}

func (a *Audio) ToPCM() ([]byte, error) {
	switch a.Metadata.Format {
	case FormatPCM:
		return a.Data, nil

	case FormatFLAC:
		pcmData, sampleRate, channels, bitDepth, err := decodeFLACToPCM(a.Data)
		if err != nil {
			return nil, err
		}

		a.Metadata.SampleRate = sampleRate
		a.Metadata.Channels = channels
		a.Metadata.BitDepth = bitDepth
		return pcmData, nil

	case FormatMP3:
		pcmData, sampleRate, channels, bitDepth, err := decodeMP3ToPCM(a.Data)
		if err != nil {
			return nil, err
		}

		a.Metadata.SampleRate = sampleRate
		a.Metadata.Channels = channels
		a.Metadata.BitDepth = bitDepth
		return pcmData, nil

	case FormatWAV:
		return extractPCMFromWAV(a.Data)

	default:
		return nil, response.Err(fmt.Errorf("unsupported format conversion: %s to PCM", a.Metadata.Format))
	}
}

func (a *Audio) ToWAV() ([]byte, error) {
	pcmData, err := a.ToPCM()
	if err != nil {
		return nil, err
	}

	return buildWAVFile(pcmData, a.Metadata.SampleRate, a.Metadata.Channels, a.Metadata.BitDepth)
}

func (a *Audio) ToFLAC() ([]byte, error) {
	if a.Metadata.Format == FormatFLAC {
		return a.Data, nil
	}

	return a.ToWAV()
}

func (a *Audio) ToOGG() ([]byte, error) {
	return a.ToWAV()
}

func (a *Audio) ToMP3() ([]byte, error) {
	return a.ToWAV()
}

func (a *Audio) ToFormat(format string) ([]byte, error) {
	normalizedFormat := strings.ToLower(format)

	switch normalizedFormat {
	case "pcm", "pcm_s16le":
		return a.ToPCM()
	case "wav":
		return a.ToWAV()
	case "flac":
		return a.ToFLAC()
	case "ogg":
		return a.ToOGG()
	case "mp3":
		return a.ToMP3()
	default:
		return nil, response.Err(fmt.Errorf("unsupported output format: %s", format))
	}
}

func (a *Audio) Resample(targetSampleRate int) error {
	if a.Metadata.SampleRate == targetSampleRate {
		return nil
	}

	pcmData, err := a.ToPCM()
	if err != nil {
		return err
	}

	buffer, err := pcmBytesToIntBuffer(pcmData, a.Metadata)
	if err != nil {
		return err
	}

	resampled, err := ResampleBuffer(buffer, targetSampleRate)
	if err != nil {
		return err
	}

	a.Data = intBufferToPCMBytes(resampled)
	a.Metadata.SampleRate = targetSampleRate
	a.Metadata.Format = FormatPCM

	return nil
}

func (a *Audio) ChangeChannels(targetChannels int) error {
	if a.Metadata.Channels == targetChannels {
		return nil
	}

	pcmData, err := a.ToPCM()
	if err != nil {
		return err
	}

	buffer, err := pcmBytesToIntBuffer(pcmData, a.Metadata)
	if err != nil {
		return err
	}

	converted, err := ChangeChannelCount(buffer, targetChannels)
	if err != nil {
		return err
	}

	a.Data = intBufferToPCMBytes(converted)
	a.Metadata.Channels = targetChannels
	a.Metadata.Format = FormatPCM

	return nil
}

func buildWAVFile(pcmData []byte, sampleRate, channels, bitDepth int) ([]byte, error) {
	byteRate := uint32(sampleRate * channels * bitDepth / 8)
	blockAlign := uint16(channels * bitDepth / 8)
	dataSize := uint32(len(pcmData))
	fileSize := 36 + dataSize

	var buf bytes.Buffer

	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, fileSize)
	buf.WriteString("WAVE")

	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16)) // fmt chunk size
	binary.Write(&buf, binary.LittleEndian, uint16(1))  // PCM format
	binary.Write(&buf, binary.LittleEndian, uint16(channels))
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(&buf, binary.LittleEndian, byteRate)
	binary.Write(&buf, binary.LittleEndian, blockAlign)
	binary.Write(&buf, binary.LittleEndian, uint16(bitDepth))

	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, dataSize)
	buf.Write(pcmData)

	return buf.Bytes(), nil
}

func decodeFLACToPCM(flacData []byte) ([]byte, int, int, int, error) {
	reader := bytes.NewReader(flacData)
	stream, err := flac.New(reader)
	if err != nil {
		return nil, 0, 0, 0, response.Err(err)
	}

	var pcmBuffer bytes.Buffer
	sampleRate := int(stream.Info.SampleRate)
	channels := int(stream.Info.NChannels)
	bitDepth := int(stream.Info.BitsPerSample)

	for {
		frame, err := stream.ParseNext()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, 0, 0, response.Err(err)
		}

		// FLAC stores samples interleaved by subframe (channel), we need to interleave them
		// For stereo: subframes[0].Samples[i] is left channel, subframes[1].Samples[i] is right channel
		numSamples := len(frame.Subframes[0].Samples)

		for i := 0; i < numSamples; i++ {
			for _, subframe := range frame.Subframes {
				sample := int16(subframe.Samples[i])
				binary.Write(&pcmBuffer, binary.LittleEndian, sample)
			}
		}
	}

	return pcmBuffer.Bytes(), sampleRate, channels, bitDepth, nil
}

func decodeMP3ToPCM(mp3Data []byte) ([]byte, int, int, int, error) {
	reader := io.NopCloser(bytes.NewReader(mp3Data))
	streamer, format, err := beepMp3.Decode(reader)
	if err != nil {
		return nil, 0, 0, 0, response.Err(err)
	}
	defer streamer.Close()

	var pcmBuffer bytes.Buffer
    
    buf := make([][2]float64, 1024)
    for {
        n, ok := streamer.Stream(buf)
        
        for i := 0; i < n; i++ {
             sampleL := int16(buf[i][0] * 32767)
             binary.Write(&pcmBuffer, binary.LittleEndian, sampleL)
             
             if format.NumChannels == 2 {
                 sampleR := int16(buf[i][1] * 32767)
                 binary.Write(&pcmBuffer, binary.LittleEndian, sampleR)
             }
        }
        if !ok {
            break
        }
    }

	return pcmBuffer.Bytes(), int(format.SampleRate), format.NumChannels, 16, nil
}

func extractPCMFromWAV(wavData []byte) ([]byte, error) {
	if len(wavData) < 44 {
		return nil, response.Err(fmt.Errorf("invalid WAV file: too small"))
	}
	return wavData[44:], nil
}

func pcmBytesToIntBuffer(pcmData []byte, metadata AudioMetadata) (*audio.IntBuffer, error) {
	bytesPerSample := metadata.BitDepth / 8
	if bytesPerSample == 0 {
		bytesPerSample = 2
	}

	sampleCount := len(pcmData) / bytesPerSample

	buffer := &audio.IntBuffer{
		Format: &audio.Format{
			SampleRate:  metadata.SampleRate,
			NumChannels: metadata.Channels,
		},
		Data:           make([]int, sampleCount),
		SourceBitDepth: metadata.BitDepth,
	}

	reader := bytes.NewReader(pcmData)
	for i := 0; i < sampleCount; i++ {
		var sample int16
		if err := binary.Read(reader, binary.LittleEndian, &sample); err != nil {
			if err == io.EOF {
				break
			}
			return nil, response.Err(err)
		}
		buffer.Data[i] = int(sample)
	}

	return buffer, nil
}

func intBufferToPCMBytes(buffer *audio.IntBuffer) []byte {
	var buf bytes.Buffer
	for _, sample := range buffer.Data {
		if sample > 32767 {
			sample = 32767
		} else if sample < -32768 {
			sample = -32768
		}
		binary.Write(&buf, binary.LittleEndian, int16(sample))
	}
	return buf.Bytes()
}