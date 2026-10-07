package python

import (
	"nstudio/app/tts/engine"
)

type Manifest struct {
	SchemaVersion       int             `json:"schemaVersion"`
	Protocol            int             `json:"protocol"`
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	Version             string          `json:"version"`
	Variant             string          `json:"variant"`
	VariantDescription  string          `json:"variantDescription,omitempty"`
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
	Prepare        *Prepare          `json:"prepare,omitempty"`
}

type Prepare struct {
	DownloadSizeMB int    `json:"downloadSizeMB"`
	Source         string `json:"source"`
}

type ManifestModel struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Voices    []ManifestVoice `json:"voices"`
	TestVoice string          `json:"testVoice,omitempty"`
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

type runnerRequest struct {
	ID     string                 `json:"id"`
	Op     string                 `json:"op"`
	Model  string                 `json:"model,omitempty"`
	Voice  string                 `json:"voice,omitempty"`
	Text   string                 `json:"text,omitempty"`
	Params map[string]interface{} `json:"params,omitempty"`
	Voices []string               `json:"voices,omitempty"`
}
