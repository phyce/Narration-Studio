//go:build linux

package native

/*
#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdlib.h>
#include <stddef.h>

// --- Piper C types ---

typedef void piper_synthesizer_t;

typedef struct {
	int   speaker_id;
	float length_scale;
	float noise_scale;
	float noise_w_scale;
} piper_synthesize_options_t;

typedef struct {
	const char *provider;
	int         cuda_device_id;
} piper_create_options_t;

// Mirrors piper_audio_chunk on x86-64 (matches Windows cAudioChunk layout):
//   const float *samples         (ptr, 8)
//   size_t       num_samples     (8)
//   int          sample_rate     (4)
//   int          _is_last_pad    (bool is_last + 3 bytes pad, 4)
//   const char  *phonemes        (ptr, 8)
//   size_t       num_phonemes    (8)
//   void        *phoneme_ids     (ptr, 8)
//   size_t       num_phoneme_ids (8)
//   void        *alignments      (ptr, 8)
//   size_t       num_alignments  (8)
typedef struct {
	const float *samples;
	size_t       num_samples;
	int          sample_rate;
	int          _is_last_pad;
	const char  *phonemes;
	size_t       num_phonemes;
	void        *phoneme_ids;
	size_t       num_phoneme_ids;
	void        *alignments;
	size_t       num_alignments;
} piper_audio_chunk_t;

// --- Function pointer types ---

typedef piper_synthesizer_t *(*fn_piper_create)(const char *, const char *, const char *);
typedef piper_synthesizer_t *(*fn_piper_create_ex)(const char *, const char *, const char *, const piper_create_options_t *);
typedef void                 (*fn_piper_free)(piper_synthesizer_t *);
typedef piper_synthesize_options_t (*fn_piper_default_opts)(piper_synthesizer_t *);
typedef int (*fn_piper_synth_start)(piper_synthesizer_t *, const char *, const piper_synthesize_options_t *);
typedef int (*fn_piper_synth_next)(piper_synthesizer_t *, piper_audio_chunk_t *);

// --- Globals ---

static void               *_handle            = NULL;
static fn_piper_create     _piper_create       = NULL;
static fn_piper_create_ex  _piper_create_ex    = NULL;
static fn_piper_free       _piper_free         = NULL;
static fn_piper_default_opts _piper_default_opts = NULL;
static fn_piper_synth_start  _piper_synth_start  = NULL;
static fn_piper_synth_next   _piper_synth_next   = NULL;

// preload_lib opens a library globally so its symbols are available to
// subsequent dlopen calls (best-effort, errors are ignored).
static void preload_lib(const char *path) {
	dlopen(path, RTLD_LAZY | RTLD_GLOBAL);
}

// load_libpiper opens libpiper.so at 'path' and resolves all required symbols.
// Returns 0 on success, -1 if dlopen failed, -2 if a symbol is missing.
static int load_libpiper(const char *path) {
	_handle = dlopen(path, RTLD_NOW | RTLD_LOCAL);
	if (!_handle) return -1;

	_piper_create       = (fn_piper_create)      dlsym(_handle, "piper_create");
	_piper_free         = (fn_piper_free)         dlsym(_handle, "piper_free");
	_piper_default_opts = (fn_piper_default_opts) dlsym(_handle, "piper_default_synthesize_options");
	_piper_synth_start  = (fn_piper_synth_start)  dlsym(_handle, "piper_synthesize_start");
	_piper_synth_next   = (fn_piper_synth_next)   dlsym(_handle, "piper_synthesize_next");

	if (!_piper_create || !_piper_free || !_piper_default_opts ||
	    !_piper_synth_start || !_piper_synth_next) {
		dlclose(_handle);
		_handle = NULL;
		return -2;
	}

	// Optional — missing in CPU-only builds.
	_piper_create_ex = (fn_piper_create_ex) dlsym(_handle, "piper_create_ex");

	return 0;
}

// --- C wrappers (called from Go) ---
// Accept void* for the synthesizer so Go can pass unsafe.Pointer directly.

static void *call_create(const char *m, const char *c, const char *e) {
	return _piper_create(m, c, e);
}

static int has_create_ex(void) {
	return _piper_create_ex != NULL;
}

static void *call_create_ex(const char *m, const char *c, const char *e, const char *provider) {
	if (!_piper_create_ex) return NULL;
	piper_create_options_t opts = { provider, 0 };
	return _piper_create_ex(m, c, e, &opts);
}

static void call_free(void *s) {
	_piper_free((piper_synthesizer_t *)s);
}

static piper_synthesize_options_t call_default_opts(void *s) {
	return _piper_default_opts((piper_synthesizer_t *)s);
}

static int call_synth_start(void *s, const char *text,
                             const piper_synthesize_options_t *opts) {
	return _piper_synth_start((piper_synthesizer_t *)s, text, opts);
}

static int call_synth_next(void *s, piper_audio_chunk_t *chunk) {
	return _piper_synth_next((piper_synthesizer_t *)s, chunk);
}
*/
import "C"

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"unsafe"
)

const (
	piperOK   = 0
	piperDone = 1
)

var (
	libpiperOnce    sync.Once
	nativeAvailable bool
	gpuCapable      bool
)

func loadSO() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	exeDir := filepath.Dir(exePath)

	// Pre-load onnxruntime with RTLD_GLOBAL so libpiper.so can resolve its
	// symbols when loaded with RTLD_NOW below.
	onnxFiles, _ := filepath.Glob(filepath.Join(exeDir, "libonnxruntime*.so*"))
	for _, f := range onnxFiles {
		cPath := C.CString(f)
		C.preload_lib(cPath)
		C.free(unsafe.Pointer(cPath))
	}

	// Try next to the executable first, then fall back to the system search path.
	candidates := []string{
		filepath.Join(exeDir, "libpiper.so"),
		"libpiper.so",
	}
	for _, path := range candidates {
		cPath := C.CString(path)
		rc := C.load_libpiper(cPath)
		C.free(unsafe.Pointer(cPath))
		if rc == 0 {
			nativeAvailable = true
			gpuCapable = C.has_create_ex() != 0
			return
		}
	}
}

// IsGPUAvailable returns true when libpiper.so exposes piper_create_ex (GPU build).
func IsGPUAvailable() bool {
	libpiperOnce.Do(loadSO)
	return gpuCapable
}

// IsNativeAvailable returns true if libpiper.so loaded successfully.
func IsNativeAvailable() bool {
	libpiperOnce.Do(loadSO)
	return nativeAvailable
}

type Synthesizer struct {
	synth unsafe.Pointer
}

type SynthesizeOptions struct {
	SpeakerID   int
	LengthScale float32
	NoiseScale  float32
	NoiseWScale float32
}

func NewSynthesizer(modelPath, configPath, espeakDataPath string, useGPU bool) (*Synthesizer, error) {
	libpiperOnce.Do(loadSO)
	if !nativeAvailable {
		return nil, fmt.Errorf("libpiper.so not available")
	}

	cModel := C.CString(modelPath)
	defer C.free(unsafe.Pointer(cModel))
	cConfig := C.CString(configPath)
	defer C.free(unsafe.Pointer(cConfig))
	cEspeak := C.CString(espeakDataPath)
	defer C.free(unsafe.Pointer(cEspeak))

	var synth unsafe.Pointer
	if useGPU && gpuCapable {
		cProvider := C.CString("cuda")
		defer C.free(unsafe.Pointer(cProvider))
		synth = C.call_create_ex(cModel, cConfig, cEspeak, cProvider)
	} else {
		synth = C.call_create(cModel, cConfig, cEspeak)
	}
	if synth == nil {
		return nil, fmt.Errorf("piper_create failed for model: %s", modelPath)
	}
	return &Synthesizer{synth: synth}, nil
}

func (s *Synthesizer) Free() {
	if s.synth != nil {
		C.call_free(s.synth)
		s.synth = nil
	}
}

func (s *Synthesizer) DefaultOptions() SynthesizeOptions {
	opts := C.call_default_opts(s.synth)
	return SynthesizeOptions{
		SpeakerID:   int(opts.speaker_id),
		LengthScale: float32(opts.length_scale),
		NoiseScale:  float32(opts.noise_scale),
		NoiseWScale: float32(opts.noise_w_scale),
	}
}

func (s *Synthesizer) Synthesize(text string, opts *SynthesizeOptions) ([]byte, int, error) {
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))

	var cOptsPtr *C.piper_synthesize_options_t
	var cOpts C.piper_synthesize_options_t
	if opts != nil {
		cOpts = C.piper_synthesize_options_t{
			speaker_id:    C.int(opts.SpeakerID),
			length_scale:  C.float(opts.LengthScale),
			noise_scale:   C.float(opts.NoiseScale),
			noise_w_scale: C.float(opts.NoiseWScale),
		}
		cOptsPtr = &cOpts
	}

	rc := C.call_synth_start(s.synth, cText, cOptsPtr)
	if int(rc) != piperOK {
		return nil, 0, fmt.Errorf("piper_synthesize_start failed with code %d", int(rc))
	}

	var pcmBuf []byte
	sampleRate := 0

	for {
		var chunk C.piper_audio_chunk_t
		rc = C.call_synth_next(s.synth, &chunk)

		if int(rc) < 0 {
			return nil, 0, fmt.Errorf("piper_synthesize_next failed with code %d", int(rc))
		}

		if chunk.num_samples > 0 {
			sampleRate = int(chunk.sample_rate)
			floats := unsafe.Slice((*float32)(unsafe.Pointer(chunk.samples)), int(chunk.num_samples))

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

		if int(rc) == piperDone {
			break
		}
	}

	return pcmBuf, sampleRate, nil
}
