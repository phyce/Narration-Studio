package python

import (
	"nstudio/app/tts/engine"
)

func modelsFromManifest(manifest Manifest) map[string]engine.Model {
	models := make(map[string]engine.Model, len(manifest.Models))
	for _, manifestModel := range manifest.Models {
		models[manifestModel.ID] = engine.Model{
			ID:     manifestModel.ID,
			Name:   manifestModel.Name,
			Engine: manifest.ID,
		}
	}
	return models
}

func ToEngine(manifest Manifest, bundlePath string) engine.Engine {
	engineType := engine.Local
	if manifest.Type == "api" {
		engineType = engine.Api
	}

	tags := manifest.Tags
	if tags == nil {
		tags = []string{"local", "python"}
	}

	return engine.Engine{
		ID:     manifest.ID,
		Name:   manifest.Name,
		Type:   engineType,
		Tags:   tags,
		Models: modelsFromManifest(manifest),
		Engine: New(manifest, bundlePath),
		NewInstance: func() engine.Base {
			return New(manifest, bundlePath)
		},
	}
}
