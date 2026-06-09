package audio

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func defaultStreamFormat() StreamFormat {
	return StreamFormat{SampleRate: 24000, Channels: 1, BitDepth: 16}
}

func patternBytes(length int) []byte {
	data := make([]byte, length)
	for i := range data {
		data[i] = byte(i % 251)
	}
	return data
}

func readExactly(t *testing.T, stream *Stream, length int) []byte {
	t.Helper()
	data := make([]byte, length)
	if _, err := io.ReadFull(stream, data); err != nil {
		t.Fatalf("ReadFull(%d): %v", length, err)
	}
	return data
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("sink failed")
}

func TestPipeStream(t *testing.T) {
	t.Run("WriteThenReadInOrder", func(t *testing.T) {
		stream, writer := NewPipeStream(defaultStreamFormat())

		if _, err := writer.Write([]byte("hello ")); err != nil {
			t.Fatalf("Write: %v", err)
		}
		if _, err := writer.Write([]byte("world")); err != nil {
			t.Fatalf("Write: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		data, err := io.ReadAll(stream)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if string(data) != "hello world" {
			t.Fatalf("got %q, want %q", data, "hello world")
		}
	})

	t.Run("ReadBlocksUntilWrite", func(t *testing.T) {
		stream, writer := NewPipeStream(defaultStreamFormat())

		received := make(chan []byte, 1)
		go func() {
			received <- readExactly(t, stream, 4)
		}()

		if _, err := writer.Write([]byte("data")); err != nil {
			t.Fatalf("Write: %v", err)
		}

		select {
		case data := <-received:
			if string(data) != "data" {
				t.Fatalf("got %q, want %q", data, "data")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Read never returned after Write")
		}
	})

	t.Run("DrainBufferedThenEOF", func(t *testing.T) {
		stream, writer := NewPipeStream(defaultStreamFormat())
		writer.Write([]byte("tail"))
		writer.Close()

		data := readExactly(t, stream, 4)
		if string(data) != "tail" {
			t.Fatalf("got %q, want %q", data, "tail")
		}

		if _, err := stream.Read(make([]byte, 1)); err != io.EOF {
			t.Fatalf("got %v, want io.EOF", err)
		}
	})

	t.Run("DrainBufferedThenProducerError", func(t *testing.T) {
		stream, writer := NewPipeStream(defaultStreamFormat())
		producerError := errors.New("synthesis failed")

		writer.Write([]byte("partial"))
		writer.CloseWithError(producerError)

		data := readExactly(t, stream, 7)
		if string(data) != "partial" {
			t.Fatalf("got %q, want %q", data, "partial")
		}

		if _, err := stream.Read(make([]byte, 1)); err != producerError {
			t.Fatalf("got %v, want producer error", err)
		}
		if stream.Err() != producerError {
			t.Fatalf("Err() = %v, want producer error", stream.Err())
		}
		select {
		case <-stream.Done():
		default:
			t.Fatal("Done() not closed after CloseWithError")
		}
	})

	t.Run("WriteAfterWriterClose", func(t *testing.T) {
		_, writer := NewPipeStream(defaultStreamFormat())
		writer.Close()

		if _, err := writer.Write([]byte("late")); err != ErrStreamClosed {
			t.Fatalf("got %v, want ErrStreamClosed", err)
		}
	})

	t.Run("WriteAfterConsumerClose", func(t *testing.T) {
		stream, writer := NewPipeStream(defaultStreamFormat())
		stream.Close()

		if _, err := writer.Write([]byte("late")); err != ErrStreamClosed {
			t.Fatalf("got %v, want ErrStreamClosed", err)
		}
	})

	t.Run("ConsumerCloseUnblocksRead", func(t *testing.T) {
		stream, _ := NewPipeStream(defaultStreamFormat())

		result := make(chan error, 1)
		go func() {
			_, err := stream.Read(make([]byte, 1))
			result <- err
		}()

		time.Sleep(10 * time.Millisecond)
		stream.Close()

		select {
		case err := <-result:
			if err != ErrStreamClosed {
				t.Fatalf("got %v, want ErrStreamClosed", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Read never returned after consumer Close")
		}
	})

	t.Run("QueueLimitFailsWrite", func(t *testing.T) {
		originalLimit := streamQueueLimit
		streamQueueLimit = 8
		t.Cleanup(func() { streamQueueLimit = originalLimit })

		_, writer := NewPipeStream(defaultStreamFormat())

		if _, err := writer.Write(patternBytes(8)); err != nil {
			t.Fatalf("Write within limit: %v", err)
		}
		if _, err := writer.Write([]byte{0}); err != ErrStreamLimitReached {
			t.Fatalf("got %v, want ErrStreamLimitReached", err)
		}
	})

	t.Run("Buffered", func(t *testing.T) {
		stream, writer := NewPipeStream(defaultStreamFormat())

		writer.Write(patternBytes(10))
		if buffered := stream.Buffered(); buffered != 10 {
			t.Fatalf("Buffered() = %d, want 10", buffered)
		}

		readExactly(t, stream, 4)
		if buffered := stream.Buffered(); buffered != 6 {
			t.Fatalf("Buffered() = %d, want 6", buffered)
		}
	})

	t.Run("FirstWriterCloseWins", func(t *testing.T) {
		stream, writer := NewPipeStream(defaultStreamFormat())

		writer.Close()
		writer.CloseWithError(errors.New("too late"))

		if err := stream.Err(); err != nil {
			t.Fatalf("Err() = %v, want nil", err)
		}
	})

	t.Run("EmptyDestinationDoesNotBlock", func(t *testing.T) {
		stream, _ := NewPipeStream(defaultStreamFormat())

		copied, err := stream.Read(nil)
		if copied != 0 || err != nil {
			t.Fatalf("Read(nil) = %d, %v; want 0, nil", copied, err)
		}
	})
}

func TestStreamDone(t *testing.T) {
	t.Run("OpenWhileProducing", func(t *testing.T) {
		stream, _ := NewPipeStream(defaultStreamFormat())

		select {
		case <-stream.Done():
			t.Fatal("Done() closed before producer finished")
		default:
		}
	})

	t.Run("ClosesAtProductionCompleteBeforeConsumption", func(t *testing.T) {
		stream, writer := NewPipeStream(defaultStreamFormat())

		writer.Write(patternBytes(100))
		writer.Close()

		select {
		case <-stream.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("Done() not closed after producer Close")
		}
		if buffered := stream.Buffered(); buffered != 100 {
			t.Fatalf("Buffered() = %d, want 100 (nothing consumed yet)", buffered)
		}
	})
}

func TestBufferedStream(t *testing.T) {
	pcm := patternBytes(64)
	stream := NewBufferedStream(defaultStreamFormat(), pcm)

	select {
	case <-stream.Done():
	default:
		t.Fatal("Done() not closed on a buffered stream")
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("Err() = %v, want nil", err)
	}

	data, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(data, pcm) {
		t.Fatal("drained data does not match input")
	}
}

func TestStreamFromAudio(t *testing.T) {
	pcm := patternBytes(32)
	source := NewAudioFromPCM(pcm, 22050, 2, 16)

	stream, err := NewStreamFromAudio(source)
	if err != nil {
		t.Fatalf("NewStreamFromAudio: %v", err)
	}

	want := StreamFormat{SampleRate: 22050, Channels: 2, BitDepth: 16}
	if stream.Format != want {
		t.Fatalf("Format = %+v, want %+v", stream.Format, want)
	}

	data, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(data, pcm) {
		t.Fatal("drained data does not match source PCM")
	}
}

func TestProducedStream(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		pcm := patternBytes(256)
		stream := NewProducedStream(defaultStreamFormat(), func(writer *StreamWriter) error {
			for offset := 0; offset < len(pcm); offset += 64 {
				if _, err := writer.Write(pcm[offset : offset+64]); err != nil {
					return err
				}
			}
			return nil
		})

		data, err := io.ReadAll(stream)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if !bytes.Equal(data, pcm) {
			t.Fatal("drained data does not match produced PCM")
		}
		if err := stream.Err(); err != nil {
			t.Fatalf("Err() = %v, want nil", err)
		}
	})

	t.Run("ErrorReturnedAfterPartialData", func(t *testing.T) {
		producerError := errors.New("engine died")
		stream := NewProducedStream(defaultStreamFormat(), func(writer *StreamWriter) error {
			writer.Write([]byte("partial"))
			return producerError
		})

		data := readExactly(t, stream, 7)
		if string(data) != "partial" {
			t.Fatalf("got %q, want %q", data, "partial")
		}
		if _, err := stream.Read(make([]byte, 1)); err != producerError {
			t.Fatalf("got %v, want producer error", err)
		}
	})

	t.Run("PanicBecomesStreamError", func(t *testing.T) {
		stream := NewProducedStream(defaultStreamFormat(), func(writer *StreamWriter) error {
			panic("boom")
		})

		select {
		case <-stream.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("Done() not closed after producer panic")
		}

		err := stream.Err()
		if err == nil {
			t.Fatal("Err() = nil, want panic error")
		}
		if !strings.Contains(err.Error(), "boom") {
			t.Fatalf("Err() = %v, want it to mention the panic value", err)
		}
	})
}

func TestTeeStream(t *testing.T) {
	t.Run("MirrorsConsumedBytes", func(t *testing.T) {
		pcm := patternBytes(128)
		var sink bytes.Buffer
		stream := NewTeeStream(NewBufferedStream(defaultStreamFormat(), pcm), &sink)

		data, err := io.ReadAll(stream)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if !bytes.Equal(data, pcm) {
			t.Fatal("drained data does not match input")
		}
		if !bytes.Equal(sink.Bytes(), pcm) {
			t.Fatal("sink does not match consumed bytes")
		}
		if err := stream.TeeErr(); err != nil {
			t.Fatalf("TeeErr() = %v, want nil", err)
		}
	})

	t.Run("SinkErrorDoesNotFailReads", func(t *testing.T) {
		pcm := patternBytes(128)
		stream := NewTeeStream(NewBufferedStream(defaultStreamFormat(), pcm), failingWriter{})

		data, err := io.ReadAll(stream)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if !bytes.Equal(data, pcm) {
			t.Fatal("drained data does not match input")
		}
		if stream.TeeErr() == nil {
			t.Fatal("TeeErr() = nil, want sink error")
		}
	})
}

func TestReadAll(t *testing.T) {
	t.Run("ReturnsAudioWithStreamFormat", func(t *testing.T) {
		pcm := patternBytes(96)
		format := StreamFormat{SampleRate: 48000, Channels: 2, BitDepth: 16}
		stream, writer := NewPipeStream(format)

		go func() {
			writer.Write(pcm)
			writer.Close()
		}()

		result, err := stream.ReadAll()
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if !bytes.Equal(result.Data, pcm) {
			t.Fatal("audio data does not match produced PCM")
		}
		wantMetadata := AudioMetadata{SampleRate: 48000, Channels: 2, BitDepth: 16, Format: FormatPCM}
		if result.Metadata != wantMetadata {
			t.Fatalf("Metadata = %+v, want %+v", result.Metadata, wantMetadata)
		}
	})

	t.Run("ReturnsProducerError", func(t *testing.T) {
		stream, writer := NewPipeStream(defaultStreamFormat())
		producerError := errors.New("synthesis failed")

		go func() {
			writer.Write([]byte("partial"))
			writer.CloseWithError(producerError)
		}()

		if _, err := stream.ReadAll(); err != producerError {
			t.Fatalf("got %v, want producer error", err)
		}
	})
}

func TestConcurrentProducerConsumer(t *testing.T) {
	const chunkCount = 1000
	const chunkSize = 331

	stream, writer := NewPipeStream(defaultStreamFormat())

	go func() {
		chunk := patternBytes(chunkSize)
		for i := 0; i < chunkCount; i++ {
			if _, err := writer.Write(chunk); err != nil {
				writer.CloseWithError(err)
				return
			}
		}
		writer.Close()
	}()

	data, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(data) != chunkCount*chunkSize {
		t.Fatalf("got %d bytes, want %d", len(data), chunkCount*chunkSize)
	}

	chunk := patternBytes(chunkSize)
	for i := 0; i < chunkCount; i++ {
		if !bytes.Equal(data[i*chunkSize:(i+1)*chunkSize], chunk) {
			t.Fatalf("chunk %d corrupted", i)
		}
	}
}
