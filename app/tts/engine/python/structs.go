package python

import (
	"nstudio/app/tts/engine"
)

type Manifest struct {
	SchemaVersion       int             `json:"schemaVersion"`
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	Version             string          `json:"version"`
	Description         string          `json:"description"`
	Type                string          `json:"type"`
	Tags                []string        `json:"tags"`
	OS                  string          `json:"os"`
	Arch                string          `json:"arch"`
	ApproxInstallSizeMB int             `json:"approxInstallSizeMB"`
	Runtime             Runtime         `json:"runtime"`
	Models              []ManifestModel `json:"models"`
}

type Runtime struct {
	Python         string            `json:"python"`
	Entrypoint     string            `json:"entrypoint"`
	BootTimeoutSec int               `json:"bootTimeoutSec"`
	Env            map[string]string `json:"env"`
}

type ManifestModel struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Voices []ManifestVoice `json:"voices"`
}

type ManifestVoice struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Gender string `json:"gender"`
}

func (manifestModel *ManifestModel) ToVoices() []engine.Voice {
	voices := make([]engine.Voice, 0, len(manifestModel.Voices))
	for _, manifestVoice := range manifestModel.Voices {
		voices = append(voices, engine.Voice{
			ID:     manifestVoice.ID,
			Name:   manifestVoice.Name,
			Gender: manifestVoice.Gender,
		})
	}
	return voices
}

type rpcRequest struct {
	ID     string                 `json:"id"`
	Op     string                 `json:"op"`
	Model  string                 `json:"model,omitempty"`
	Voice  string                 `json:"voice,omitempty"`
	Text   string                 `json:"text,omitempty"`
	Out    string                 `json:"out,omitempty"`
	Params map[string]interface{} `json:"params,omitempty"`
	Voices []string               `json:"voices,omitempty"`
}

type rpcResponse struct {
	ID         string `json:"id,omitempty"`
	Event      string `json:"event,omitempty"`
	Runner     string `json:"runner,omitempty"`
	Version    string `json:"version,omitempty"`
	Ok         bool   `json:"ok"`
	Error      string `json:"error,omitempty"`
	Code       string `json:"code,omitempty"`
	SampleRate int    `json:"sampleRate,omitempty"`
	Channels   int    `json:"channels,omitempty"`
	Bits       int    `json:"bits,omitempty"`
	Frames     int    `json:"frames,omitempty"`
}

type SynthPayload struct {
	Voice  string                 `json:"voice"`
	Text   string                 `json:"text"`
	Params map[string]interface{} `json:"params,omitempty"`
}
