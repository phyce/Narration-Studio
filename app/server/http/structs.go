package http

import "fmt"

type ProfileTTSRequest struct {
	Profile   string                 `json:"profile" validate:"required"`
	Character string                 `json:"character" validate:"required"`
	Text      string                 `json:"text" validate:"required,min=1,max=10000"`
	Options   map[string]interface{} `json:"options,omitempty"`
}

func (r *ProfileTTSRequest) GetAudioOptions() (*AudioOptions, error) {
	if r.Options == nil {
		return nil, nil
	}

	audioOpts, exists := r.Options["audio"]
	if !exists {
		return nil, nil
	}

	audioMap, ok := audioOpts.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid audio options format")
	}

	opts := &AudioOptions{
		Format:     "wav", // default
		SampleRate: 0,     // 0 means use engine default
		Channels:   0,     // 0 means use engine default
		BitDepth:   0,     // 0 means use engine default
	}

	if format, ok := audioMap["format"].(string); ok {
		opts.Format = format
	}
	if sampleRate, ok := audioMap["sample_rate"].(float64); ok {
		opts.SampleRate = int(sampleRate)
	}
	if channels, ok := audioMap["channels"].(float64); ok {
		opts.Channels = int(channels)
	}
	if bitDepth, ok := audioMap["bit_depth"].(float64); ok {
		opts.BitDepth = int(bitDepth)
	}

	return opts, nil
}

type AudioOptions struct {
	Format     string `json:"format"`      // "pcm_s16le", "wav", "flac", etc.
	SampleRate int    `json:"sample_rate"` // 22050, 24000, 44100, etc.
	Channels   int    `json:"channels"`
	BitDepth   int    `json:"bit_depth"` // 16, 24, 32
}

type SimpleTTSRequest struct {
	Text    string                 `json:"text" validate:"required,min=1,max=10000"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type ProfileCreateRequest struct {
	ID          string `json:"id" validate:"required"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
