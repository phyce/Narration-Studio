//go:build !clib

package modelManager

import (
	"nstudio/app/common/response"
	"time"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/speaker"
)

func initSpeaker() {
	format := beep.Format{
		SampleRate:  48000,
		NumChannels: 1,
		Precision:   2,
	}

	if err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)); err != nil {
		response.Err("failed to initialize speaker: %w", err)
	}
}
