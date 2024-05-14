package tts

var Engines = []Engine{
	{
		ID:   1,
		Name: "Piper",
		Models: []Model{
			{
				ID:   2,
				Name: "LibriTTS",
				Voices: []Voice{
					{ID: 23, Name: "Piper Test voice 1", Gender: 1},
					{ID: 24, Name: "Piper Test voice 2", Gender: 0},
				},
			},
		},
	},
	{
		ID:   5,
		Name: "Suno Bark",
		Models: []Model{
			{
				ID:   6,
				Name: "Default",
				Voices: []Voice{
					{ID: 7, Name: "Suno Test voice 1", Gender: 1},
					{ID: 8, Name: "Suno Test voice 2", Gender: 0},
				},
			},
		},
	},
	{
		ID:   9,
		Name: "Microsoft",
		Models: []Model{
			{
				ID:   10,
				Name: "SAPI 4",
				Voices: []Voice{
					{ID: 11, Name: "MS Test voice 1", Gender: 1},
					{ID: 12, Name: "MS Test voice 2", Gender: 0},
				},
			},
			{
				ID:   13,
				Name: "SAPI 5",
				Voices: []Voice{
					{ID: 14, Name: "MS Test voice 3", Gender: 1},
					{ID: 15, Name: "MS Test voice 4", Gender: 0},
				},
			},
		},
	},
}

func getAllVoices() []Voice {
	result := []Voice{}
	for _, engine := range Engines {
		for _, model := range engine.Models {
			for _, voice := range model.Voices {
				result = append(result, voice)
			}
		}
	}
	return result
}
