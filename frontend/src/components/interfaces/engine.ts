export interface Engine {
	id: string;
	name: string;
	type: 'local' | 'api';
	tags: string[];
	models: Record<string, Model>;
}

export interface ApiAvailability {
	available: boolean;
	missingApiKey: boolean;
	reason?: string;
}

export interface Model {
	id: string;
	name: string;
	engine: string;
	key: string;
}

export interface Voice {
	voiceID: string;
	name: string;
	gender: string;
	key: string;
}

export interface CharacterVoice {
	key: string;
	name: string;
	engine: string;
	model: string;
	voice: string;
}

export interface ProfileSettings {
	modelToggles?: Record<string, boolean>;
	cacheEnabled?: boolean;
}