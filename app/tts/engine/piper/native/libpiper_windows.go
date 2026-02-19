package native

import (
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"syscall"
	"unsafe"
)

const (
	piperOK   = 0
	piperDone = 1
)

var (
	libpiper         *syscall.LazyDLL
	libpiperDirectML *syscall.LazyDLL
	libpiperOnce     sync.Once

	procCreate                   *syscall.LazyProc
	procCreateEx                 *syscall.LazyProc
	procFree                     *syscall.LazyProc
	procDefaultSynthesizeOptions *syscall.LazyProc
	procSynthesizeStart          *syscall.LazyProc
	procSynthesizeNext           *syscall.LazyProc

	gpuAvailable    bool
	nativeAvailable bool
)

func loadDLL() {
	// Probe for DirectML DLL
	libpiperDirectML = syscall.NewLazyDLL("libpiper_directml.dll")
	if err := libpiperDirectML.Load(); err == nil {
		gpuAvailable = true
	} else {
		libpiperDirectML = nil
	}

	// Always load CPU DLL references
	libpiper = syscall.NewLazyDLL("libpiper.dll")
	if err := libpiper.Load(); err == nil {
		nativeAvailable = true
	}

	// Set up procs from CPU DLL (default)
	procCreate = libpiper.NewProc("piper_create")
	procCreateEx = libpiper.NewProc("piper_create_ex")
	procFree = libpiper.NewProc("piper_free")
	procDefaultSynthesizeOptions = libpiper.NewProc("piper_default_synthesize_options")
	procSynthesizeStart = libpiper.NewProc("piper_synthesize_start")
	procSynthesizeNext = libpiper.NewProc("piper_synthesize_next")
}

// IsGPUAvailable returns true if the DirectML-enabled DLL is present.
func IsGPUAvailable() bool {
	libpiperOnce.Do(loadDLL)
	return gpuAvailable
}

// IsNativeAvailable returns true if libpiper.dll loaded successfully.
func IsNativeAvailable() bool {
	libpiperOnce.Do(loadDLL)
	return nativeAvailable
}

// cSynthesizeOptions mirrors the C struct piper_synthesize_options.
type cSynthesizeOptions struct {
	SpeakerID   int32
	LengthScale float32
	NoiseScale  float32
	NoiseWScale float32
}

// cAudioChunk mirrors the C struct piper_audio_chunk (x86_64 layout).
type cAudioChunk struct {
	Samples       uintptr // const float *
	NumSamples    uintptr // size_t
	SampleRate    int32
	_isLastPad    int32 // bool is_last (1 byte) + 3 bytes padding
	Phonemes      uintptr
	NumPhonemes   uintptr
	PhonemeIds    uintptr
	NumPhonemeIds uintptr
	Alignments    uintptr
	NumAlignments uintptr
}

type Synthesizer struct {
	synth  uintptr // opaque pointer to piper_synthesizer
	usedDL *syscall.LazyDLL
}

type SynthesizeOptions struct {
	SpeakerID   int
	LengthScale float32
	NoiseScale  float32
	NoiseWScale float32
}

func NewSynthesizer(modelPath, configPath, espeakDataPath string, useGPU bool) (*Synthesizer, error) {
	libpiperOnce.Do(loadDLL)

	cModel, _ := syscall.BytePtrFromString(modelPath)
	cConfig, _ := syscall.BytePtrFromString(configPath)
	cEspeak, _ := syscall.BytePtrFromString(espeakDataPath)

	// Try DirectML if requested and available
	if useGPU && gpuAvailable {
		synth, err := tryCreateWithDLL(libpiperDirectML, cModel, cConfig, cEspeak, "directml")
		if err == nil {
			return synth, nil
		}
		// DirectML failed, fall back to CPU
	}

	// CPU path
	if err := procCreate.Find(); err != nil {
		return nil, fmt.Errorf("libpiper.dll not available: %w", err)
	}

	r1, _, _ := procCreate.Call(
		uintptr(unsafe.Pointer(cModel)),
		uintptr(unsafe.Pointer(cConfig)),
		uintptr(unsafe.Pointer(cEspeak)),
	)

	if r1 == 0 {
		return nil, fmt.Errorf("piper_create failed for model: %s", modelPath)
	}

	return &Synthesizer{synth: r1, usedDL: libpiper}, nil
}

// tryCreateWithDLL attempts to create a synthesizer using a specific DLL with piper_create_ex.
func tryCreateWithDLL(dll *syscall.LazyDLL, cModel, cConfig, cEspeak *byte, provider string) (*Synthesizer, error) {
	createEx := dll.NewProc("piper_create_ex")
	if err := createEx.Find(); err != nil {
		return nil, fmt.Errorf("piper_create_ex not found: %w", err)
	}

	cProvider, _ := syscall.BytePtrFromString(provider)

	r1, _, _ := createEx.Call(
		uintptr(unsafe.Pointer(cModel)),
		uintptr(unsafe.Pointer(cConfig)),
		uintptr(unsafe.Pointer(cEspeak)),
		uintptr(unsafe.Pointer(cProvider)),
	)

	if r1 == 0 {
		return nil, fmt.Errorf("piper_create_ex failed with provider %s", provider)
	}

	return &Synthesizer{synth: r1, usedDL: dll}, nil
}

func (s *Synthesizer) Free() {
	if s.synth != 0 {
		freeProc := s.usedDL.NewProc("piper_free")
		freeProc.Call(s.synth)
		s.synth = 0
	}
}

// DefaultOptions returns the default synthesis options from the voice model config.
// On Windows x64 ABI, structs >8 bytes are returned via a hidden first pointer parameter.
func (s *Synthesizer) DefaultOptions() SynthesizeOptions {
	var cOpts cSynthesizeOptions
	defOptsProc := s.usedDL.NewProc("piper_default_synthesize_options")
	defOptsProc.Call(
		uintptr(unsafe.Pointer(&cOpts)),
		s.synth,
	)
	return SynthesizeOptions{
		SpeakerID:   int(cOpts.SpeakerID),
		LengthScale: cOpts.LengthScale,
		NoiseScale:  cOpts.NoiseScale,
		NoiseWScale: cOpts.NoiseWScale,
	}
}

// Synthesize runs text-to-speech and returns int16 PCM bytes and the sample rate.
func (s *Synthesizer) Synthesize(text string, opts *SynthesizeOptions) ([]byte, int, error) {
	cText, _ := syscall.BytePtrFromString(text)

	var optsPtr uintptr
	if opts != nil {
		cOpts := cSynthesizeOptions{
			SpeakerID:   int32(opts.SpeakerID),
			LengthScale: opts.LengthScale,
			NoiseScale:  opts.NoiseScale,
			NoiseWScale: opts.NoiseWScale,
		}
		optsPtr = uintptr(unsafe.Pointer(&cOpts))
	}

	startProc := s.usedDL.NewProc("piper_synthesize_start")
	rc, _, _ := startProc.Call(
		s.synth,
		uintptr(unsafe.Pointer(cText)),
		optsPtr,
	)
	if int32(rc) != piperOK {
		return nil, 0, fmt.Errorf("piper_synthesize_start failed with code %d", int32(rc))
	}

	var pcmBuf []byte
	sampleRate := 0

	nextProc := s.usedDL.NewProc("piper_synthesize_next")
	for {
		var chunk cAudioChunk
		rc, _, _ = nextProc.Call(
			s.synth,
			uintptr(unsafe.Pointer(&chunk)),
		)

		if int32(rc) < 0 {
			return nil, 0, fmt.Errorf("piper_synthesize_next failed with code %d", int32(rc))
		}

		if chunk.NumSamples > 0 {
			sampleRate = int(chunk.SampleRate)

			floats := unsafe.Slice((*float32)(unsafe.Pointer(chunk.Samples)), int(chunk.NumSamples))

			for _, f := range floats {
				if f > 1.0 {
					f = 1.0
				} else if f < -1.0 {
					f = -1.0
				}

				sample := int16(math.Round(float64(f) * 32767.0))
				var b [2]byte
				binary.LittleEndian.PutUint16(b[:], uint16(sample))
				pcmBuf = append(pcmBuf, b[:]...)
			}
		}

		if int32(rc) == piperDone {
			break
		}
	}

	return pcmBuf, sampleRate, nil
}
