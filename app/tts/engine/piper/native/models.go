package native

import (
	"nstudio/app/common/util"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
	"os"
	"path/filepath"
)

func FetchModels() map[string]engine.Model {
	result := make(map[string]engine.Model)

	dir := config.GetEngine().Local.Piper.ModelsDirectory
	if dir == "" {
		return result
	}
	err, modelsDir := util.ExpandPath(dir)
	if err != nil || modelsDir == "" {
		return result
	}

	entries, err := os.ReadDir(modelsDir)
	if err != nil {
		return result
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()

		complete := true
		for _, suffix := range []string{".onnx", ".onnx.json", ".metadata.json"} {
			if _, err := os.Stat(filepath.Join(modelsDir, id, id+suffix)); err != nil {
				complete = false
				break
			}
		}
		if !complete {
			continue
		}

		result[id] = engine.Model{
			ID:     id,
			Name:   id,
			Engine: "piper",
		}
	}

	return result
}
