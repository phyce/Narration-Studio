package http

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nstudio/app/common/audio"

	"github.com/labstack/echo/v4"
)

var errClientGone = errors.New("client disconnected")

type truncatingResponseWriter struct {
	headers   http.Header
	written   bytes.Buffer
	failAfter int
	flushes   int
	status    int
}

func newTruncatingResponseWriter(failAfter int) *truncatingResponseWriter {
	return &truncatingResponseWriter{headers: http.Header{}, failAfter: failAfter}
}

func (writer *truncatingResponseWriter) Header() http.Header { return writer.headers }

func (writer *truncatingResponseWriter) Write(data []byte) (int, error) {
	if writer.written.Len() >= writer.failAfter {
		return 0, errClientGone
	}
	return writer.written.Write(data)
}

func (writer *truncatingResponseWriter) WriteHeader(status int) { writer.status = status }

func (writer *truncatingResponseWriter) Flush() { writer.flushes++ }

func newStreamContext(responseWriter http.ResponseWriter) echo.Context {
	request := httptest.NewRequest(http.MethodPost, "/tts", strings.NewReader(""))
	return echo.New().NewContext(request, responseWriter)
}

func filledPipeStream(byteCount int) *audio.Stream {
	stream, writer := audio.NewPipeStream(audio.StreamFormat{SampleRate: 22050, Channels: 1, BitDepth: 16})
	_, _ = writer.Write(bytes.Repeat([]byte{0x11, 0x22}, byteCount/2))
	_ = writer.Close()
	return stream
}

func TestStreamTTSResponseCompletion(t *testing.T) {
	const payloadSize = 128 * 1024

	t.Run("DrainedStreamReportsCompleted", func(t *testing.T) {
		responseWriter := newTruncatingResponseWriter(payloadSize * 2)
		stream := filledPipeStream(payloadSize)

		if !streamTTSResponse(newStreamContext(responseWriter), stream, "wav", "test.wav") {
			t.Fatal("streamTTSResponse reported incomplete for a fully drained stream")
		}
		if got := responseWriter.written.Len(); got != payloadSize+44 {
			t.Errorf("wrote %d bytes, want %d (44-byte header + payload)", got, payloadSize+44)
		}
		if responseWriter.flushes == 0 {
			t.Error("response was never flushed")
		}
	})

	t.Run("PCMSkipsWAVHeader", func(t *testing.T) {
		responseWriter := newTruncatingResponseWriter(payloadSize * 2)
		stream := filledPipeStream(payloadSize)

		if !streamTTSResponse(newStreamContext(responseWriter), stream, "pcm", "test.pcm") {
			t.Fatal("streamTTSResponse reported incomplete for a fully drained stream")
		}
		if got := responseWriter.written.Len(); got != payloadSize {
			t.Errorf("wrote %d bytes, want %d (no header)", got, payloadSize)
		}
	})

	t.Run("ClientDisconnectReportsIncomplete", func(t *testing.T) {
		responseWriter := newTruncatingResponseWriter(1024)
		stream := filledPipeStream(payloadSize)

		if streamTTSResponse(newStreamContext(responseWriter), stream, "wav", "test.wav") {
			t.Fatal("streamTTSResponse reported completed after the client hung up")
		}
		if responseWriter.written.Len() >= payloadSize {
			t.Errorf("wrote %d bytes, expected a truncated body", responseWriter.written.Len())
		}
		if _, err := stream.Read(make([]byte, 16)); !errors.Is(err, audio.ErrStreamClosed) {
			t.Errorf("stream error after disconnect = %v, want ErrStreamClosed", err)
		}
	})

	t.Run("ProducerFailureReportsIncomplete", func(t *testing.T) {
		responseWriter := newTruncatingResponseWriter(payloadSize * 2)
		stream, writer := audio.NewPipeStream(audio.StreamFormat{SampleRate: 22050, Channels: 1, BitDepth: 16})
		_, _ = writer.Write(bytes.Repeat([]byte{0x33, 0x44}, 512))
		_ = writer.CloseWithError(errors.New("engine exploded"))

		if streamTTSResponse(newStreamContext(responseWriter), stream, "wav", "test.wav") {
			t.Fatal("streamTTSResponse reported completed after the producer failed")
		}
	})
}

func TestAudioFileExtension(t *testing.T) {
	cases := map[string]string{
		"wav":       "wav",
		"flac":      "flac",
		"ogg":       "ogg",
		"mp3":       "mp3",
		"pcm":       "pcm",
		"pcm_s16le": "pcm",
	}

	for outputFormat, want := range cases {
		if got := audioFileExtension(outputFormat); got != want {
			t.Errorf("audioFileExtension(%q) = %q, want %q", outputFormat, got, want)
		}
	}
}

func TestCanStream(t *testing.T) {
	cases := []struct {
		name         string
		outputFormat string
		audioOptions *AudioOptions
		want         bool
	}{
		{"WAVNoOptions", "wav", nil, true},
		{"PCMNoOptions", "pcm", nil, true},
		{"PCMS16LENoOptions", "pcm_s16le", nil, true},
		{"MP3", "mp3", nil, false},
		{"FLAC", "flac", nil, false},
		{"OGG", "ogg", nil, false},
		{"WAVUntouchedOptions", "wav", &AudioOptions{Format: "wav"}, true},
		{"WAVResampleRequested", "wav", &AudioOptions{Format: "wav", SampleRate: 44100}, false},
		{"WAVChannelChangeRequested", "wav", &AudioOptions{Format: "wav", Channels: 2}, false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := canStream(testCase.outputFormat, testCase.audioOptions); got != testCase.want {
				t.Errorf("canStream(%q, %+v) = %v, want %v",
					testCase.outputFormat, testCase.audioOptions, got, testCase.want)
			}
		})
	}
}
