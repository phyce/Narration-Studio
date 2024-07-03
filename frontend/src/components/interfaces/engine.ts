export interface Engine {
	id:     string;
	name:   string;
	models: Record<string, Model>;
}

export interface Model {
	id:     string;
	name:   string;
	engine: string;
	key:    string;
}

export interface Voice {
	voiceID:    number;
	name:       string;
	gender:     string;
	key:        string;
}

export interface CharacterVoice {
	key:        string;
	name:       string;
	engine:     string;
	model:      string;
	voice:      string;
}