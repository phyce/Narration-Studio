package python

import (
	"nstudio/app/common/audio"
	"nstudio/app/tts/engine"
)

type Manifest struct {
	SchemaVersion       int             `json:"schemaVersion"`
	Protocol            int             `json:"protocol"`
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	Version             string          `json:"version"`
	EngineVersion       string          `json:"engineVersion,omitempty"`
	Variant             string          `json:"variant"`
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
	Executable     string            `json:"executable,omitempty"`
	BootTimeoutSec int               `json:"bootTimeoutSec"`
	Env            map[string]string `json:"env"`
	Prepare        *Prepare          `json:"prepare,omitempty"`
}

func (packRuntime Runtime) RunsPython() bool {
	return packRuntime.Executable == ""
}

func (packRuntime Runtime) Program() string {
	if packRuntime.RunsPython() {
		return packRuntime.Python
	}
	return packRuntime.Executable
}

func (packRuntime Runtime) RunnerFiles() []string {
	if packRuntime.RunsPython() {
		return []string{packRuntime.Python, packRuntime.Entrypoint}
	}
	return []string{packRuntime.Executable}
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
	ID         string `json:"id"`
	Name       string `json:"name"`
	Gender     string `json:"gender"`
	SampleRate int    `json:"sampleRate,omitempty"`
}

func (manifest *Manifest) findModel(modelID string) (*ManifestModel, bool) {
	for index := range manifest.Models {
		if manifest.Models[index].ID == modelID {
			return &manifest.Models[index], true
		}
	}
	return nil, false
}

func (manifestModel *ManifestModel) VoiceFormat(voice string, modelFormat audio.StreamFormat) audio.StreamFormat {
	for _, manifestVoice := range manifestModel.Voices {
		if (manifestVoice.ID == voice || manifestVoice.Name == voice) && manifestVoice.SampleRate > 0 {
			modelFormat.SampleRate = manifestVoice.SampleRate
			return modelFormat
		}
	}
	return modelFormat
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
