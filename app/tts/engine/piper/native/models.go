package native

import "nstudio/app/tts/engine"

func FetchModels() map[string]engine.Model {
	return map[string]engine.Model{
		"libritts": {
			ID:     "libritts",
			Name:   "LibriTTS",
			Engine: "piper",
			Download: engine.ModelDownload{
				Metadata: "",
				Model:    "https://mechanic.ink/narrator-studio/models/en/en_GB/vctk/medium/en_GB-vctk-medium.onnx",
				Phonemes: "https://mechanic.ink/narrator-studio/models/en/en_GB/vctk/medium/en_GB-vctk-medium.onnx.json",
			},
		},
		"vctk": {
			ID:     "vctk",
			Name:   "VCTK",
			Engine: "piper",
			Download: engine.ModelDownload{
				Metadata: "",
				Model:    "https://mechanic.ink/narrator-studio/models/en/en_GB/vctk/medium/en_GB-vctk-medium.onnx",
				Phonemes: "https://mechanic.ink/narrator-studio/models/en/en_GB/vctk/medium/en_GB-vctk-medium.onnx.json",
			},
		},
	}
}
