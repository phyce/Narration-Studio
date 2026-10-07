package python

import (
	"context"
	"encoding/binary"
	"io"
	"nstudio/app/common/audio"
	"nstudio/app/common/response"
	"path/filepath"
	"time"
)

const verificationText = "Narration Studio is ready to speak."

var (
	minimumVerificationDuration = 300 * time.Millisecond
	minimumVerificationPeak     = 300
)

func VerifyBundle(ctx context.Context, manifest Manifest, bundlePath string) error {
	verifyingRunner, err := newRunner(manifest.ID, bundlePath, &manifest)
	if err != nil {
		return response.Err("start runner: %w", err)
	}
	defer verifyingRunner.close(time.Duration(defaultShutdownGraceSec) * time.Second)
	stopKillingOnCancel := context.AfterFunc(ctx, func() { _ = verifyingRunner.kill() })
	defer stopKillingOnCancel()

	for _, model := range manifest.Models {
		if err := verifyModel(ctx, verifyingRunner, model, bundlePath); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return response.Err("model %s: %w", model.ID, err)
		}
	}
	return nil
}

func verifyModel(ctx context.Context, verifyingRunner *runner, model ManifestModel, bundlePath string) error {
	if err := verifyingRunner.load(model.ID, nil); err != nil {
		return response.Err("load: %w", err)
	}

	voice := testVoice(model, bundlePath)
	if voice == "" {
		return nil
	}

	speech := audio.NewProducedStream(verifyingRunner.format, func(writer *audio.StreamWriter) error {
		return verifyingRunner.streamSynth(ctx, runnerRequest{Op: "synth", Model: model.ID, Voice: voice, Text: verificationText}, writer)
	})
	pcm, err := io.ReadAll(speech)
	if err != nil {
		return response.Err("speak a test sentence: %w", err)
	}
	if !soundsLikeSpeech(pcm, verifyingRunner.format) {
		return response.Err("the test sentence produced no audible audio (%d bytes)", len(pcm))
	}
	return nil
}

func testVoice(model ManifestModel, bundlePath string) string {
	if model.TestVoice != "" {
		return filepath.Join(bundlePath, filepath.FromSlash(model.TestVoice))
	}
	if len(model.Voices) > 0 {
		return model.Voices[0].ID
	}
	return ""
}

func soundsLikeSpeech(pcm []byte, format audio.StreamFormat) bool {
	if format.BitDepth != 16 || format.SampleRate <= 0 || format.Channels <= 0 {
		return len(pcm) > 0
	}
	bytesPerSecond := format.SampleRate * format.Channels * 2
	if time.Duration(len(pcm))*time.Second/time.Duration(bytesPerSecond) < minimumVerificationDuration {
		return false
	}
	peak := 0
	for offset := 0; offset+1 < len(pcm); offset += 2 {
		sample := int(int16(binary.LittleEndian.Uint16(pcm[offset:])))
		if sample < 0 {
			sample = -sample
		}
		if sample > peak {
			peak = sample
		}
	}
	return peak >= minimumVerificationPeak
}
