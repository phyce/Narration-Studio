package config

import "nstudio/app/enums/Engines"

func Debug() bool {
	return GetSettings().Debug
}

func GetServerInstanceCount(engineID, modelID string) int {
	settings := GetSettings()

	switch engineID {
	case string(Engines.Piper):
		if settings.Server.Engines.Piper != nil {
			if modelConfig, exists := settings.Server.Engines.Piper[modelID]; exists {
				return modelConfig.Instances
			}
		}
	case string(Engines.OpenAI):
		if settings.Server.Engines.OpenAI != nil {
			if modelConfig, exists := settings.Server.Engines.OpenAI[modelID]; exists {
				return modelConfig.Instances
			}
		}
	case string(Engines.ElevenLabs):
		if settings.Server.Engines.ElevenLabs != nil {
			if modelConfig, exists := settings.Server.Engines.ElevenLabs[modelID]; exists {
				return modelConfig.Instances
			}
		}
	case string(Engines.MsSapi4):
		if settings.Server.Engines.MsSapi4 != nil {
			if modelConfig, exists := settings.Server.Engines.MsSapi4[modelID]; exists {
				return modelConfig.Instances
			}
		}
	}

	return 0
}
