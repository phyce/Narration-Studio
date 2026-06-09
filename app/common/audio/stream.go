package audio

import (
	"errors"
	"io"
	"nstudio/app/common/response"
	"sync"
)

// Producers never block: the queue is elastic so generation can run ahead of
// playback. The limit only guards against a consumer that never reads.
var streamQueueLimit = 64 * 1024 * 1024

var (
	ErrStreamClosed       = errors.New("audio stream is closed")
	ErrStreamLimitReached = errors.New("audio stream queue limit exceeded")
)

type StreamFormat struct {
	SampleRate int
	Channels   int
	BitDepth   int
}

type Stream struct {
	Format StreamFormat

	mutex          sync.Mutex
	condition      *sync.Cond
	queue          []byte
	producerClosed bool
	producerError  error
	consumerClosed bool
	teeSink        io.Writer
	teeError       error
	done           chan struct{}
}

type StreamWriter struct {
	stream *Stream
}

func NewPipeStream(format StreamFormat) (*Stream, *StreamWriter) {
	stream := newStream(format)
	return stream, &StreamWriter{stream: stream}
}

func NewBufferedStream(format StreamFormat, pcm []byte) *Stream {
	stream := newStream(format)
	stream.queue = pcm
	stream.producerClosed = true
	close(stream.done)
	return stream
}

func NewStreamFromAudio(source *Audio) (*Stream, error) {
	pcm, err := source.ToPCM()
	if err != nil {
		return nil, err
	}

	return NewBufferedStream(StreamFormat{
		SampleRate: source.Metadata.SampleRate,
		Channels:   source.Metadata.Channels,
		BitDepth:   source.Metadata.BitDepth,
	}, pcm), nil
}

func NewProducedStream(format StreamFormat, produce func(writer *StreamWriter) error) *Stream {
	stream, writer := NewPipeStream(format)

	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				writer.CloseWithError(response.Err("stream producer panicked: %v", recovered))
			}
		}()

		if err := produce(writer); err != nil {
			writer.CloseWithError(err)
		} else {
			writer.Close()
		}
	}()

	return stream
}

func NewTeeStream(stream *Stream, sink io.Writer) *Stream {
	stream.mutex.Lock()
	defer stream.mutex.Unlock()
	stream.teeSink = sink
	stream.teeError = nil
	return stream
}

func newStream(format StreamFormat) *Stream {
	stream := &Stream{
		Format: format,
		done:   make(chan struct{}),
	}
	stream.condition = sync.NewCond(&stream.mutex)
	return stream
}

func (stream *Stream) Read(destination []byte) (int, error) {
	stream.mutex.Lock()
	defer stream.mutex.Unlock()

	for {
		if stream.consumerClosed {
			return 0, ErrStreamClosed
		}
		if len(stream.queue) > 0 {
			break
		}
		if stream.producerClosed {
			if stream.producerError != nil {
				return 0, stream.producerError
			}
			return 0, io.EOF
		}
		if len(destination) == 0 {
			return 0, nil
		}
		stream.condition.Wait()
	}

	copied := copy(destination, stream.queue)
	stream.queue = stream.queue[copied:]

	if stream.teeSink != nil && copied > 0 {
		if _, err := stream.teeSink.Write(destination[:copied]); err != nil {
			stream.teeError = err
			stream.teeSink = nil
		}
	}

	return copied, nil
}

func (stream *Stream) Close() error {
	stream.mutex.Lock()
	defer stream.mutex.Unlock()

	if stream.consumerClosed {
		return nil
	}
	stream.consumerClosed = true
	stream.queue = nil
	stream.condition.Broadcast()
	return nil
}

// Done closes when the producer finished (success or error), not when playback
// finished — it is the model-pool release hook.
func (stream *Stream) Done() <-chan struct{} {
	return stream.done
}

func (stream *Stream) Err() error {
	stream.mutex.Lock()
	defer stream.mutex.Unlock()
	return stream.producerError
}

func (stream *Stream) TeeErr() error {
	stream.mutex.Lock()
	defer stream.mutex.Unlock()
	return stream.teeError
}

func (stream *Stream) Buffered() int {
	stream.mutex.Lock()
	defer stream.mutex.Unlock()
	return len(stream.queue)
}

func (stream *Stream) ReadAll() (*Audio, error) {
	data, err := io.ReadAll(stream)
	if err != nil {
		return nil, err
	}
	return NewAudioFromPCM(data, stream.Format.SampleRate, stream.Format.Channels, stream.Format.BitDepth), nil
}

func (writer *StreamWriter) Write(data []byte) (int, error) {
	stream := writer.stream
	stream.mutex.Lock()
	defer stream.mutex.Unlock()

	if stream.consumerClosed || stream.producerClosed {
		return 0, ErrStreamClosed
	}
	if len(stream.queue)+len(data) > streamQueueLimit {
		return 0, ErrStreamLimitReached
	}

	stream.queue = append(stream.queue, data...)
	stream.condition.Broadcast()
	return len(data), nil
}

func (writer *StreamWriter) Close() error {
	return writer.CloseWithError(nil)
}

func (writer *StreamWriter) CloseWithError(err error) error {
	stream := writer.stream
	stream.mutex.Lock()
	defer stream.mutex.Unlock()

	if stream.producerClosed {
		return nil
	}
	stream.producerClosed = true
	stream.producerError = err
	close(stream.done)
	stream.condition.Broadcast()
	return nil
}
