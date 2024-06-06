export interface Engine {
	id: string;
	name: string;
	models: Record<string, Model>;
}

export interface Model {
	id: string;
	name: string;
}

export interface Voice {
	piperVoiceID: number;
	name: string;
	gender: string;
}