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
	voiceID: number;
	name: string;
	gender: string;
}