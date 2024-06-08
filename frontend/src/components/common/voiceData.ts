import {ref} from "vue";

export interface Engine {
	id: string;
	name: string;
	models?: Model[];
}
export interface Model {
	id: string;
	name: string;
	engine?: string;
	voices?: Voice[];
}
export interface Voice {
	id: string;
	name: string;
	gender: string;
}

export function getEngines() : Engine[] {
	return engines.value.map(engine => {
		return {
			id: engine.id,
			name: engine.name,
		}
	});
}

export function getModels() : Model[] {
	return engines.value.flatMap(engine => {
		return engine.models?.map(model => {
			return {
				id: model.id,
				name: model.name,
				engine: engine.name,
			}
		}) ?? [];
	});
}

export const engines = ref<Engine[]>([
	{
		id: "1",
		name: 'Piper',
		models: [
			{
				id: "2",
				name: 'LibriTTS',
				voices: [
					{
						id: "23",
						name: 'Piper Test voice 1',
						gender: 'male',
					},
					{
						id: "24",
						name: 'Piper Test voice 2',
						gender: 'Female',
					},
					{
						id: "25",
						name: 'Piper Test voice 3',
						gender: 'Female',
					},
					{
						id: "26",
						name: 'Piper Test voice 4',
						gender: 'Female',
					},
					{
						id: "27",
						name: 'Piper Test voice 5',
						gender: 'Female',
					},
					{
						id: "28",
						name: 'Piper Test voice 6',
						gender: 'Female',
					},
					{
						id: "29",
						name: 'Piper Test voice 8',
						gender: 'Female',
					},
					{
						id: "210",
						name: 'Piper Test voice 2',
						gender: 'Female',
					},
					{
						id: "211",
						name: 'Piper Test voice 2',
						gender: 'Female',
					},
					{
						id: "212",
						name: 'Piper Test voice 2',
						gender: 'Female',
					},
					{
						id: "213",
						name: 'Piper Test voice 2',
						gender: 'Female',
					},
				]
			},
		]
	},
	{
		id: "5",
		name: 'Suno Bark',
		models: [
			{
				id: "6",
				name: 'Default',
				voices: [
					{
						id: "7",
						name: 'Suno Test voice 1',
						gender: 'male',
					},
					{
						id: "8",
						name: 'Suno Test voice 2',
						gender: 'Female',
					},
				]
			},
		]
	},
	{
		id: '9',
		name: 'Microsoft',
		models: [
			{
				id: '10',
				name: 'SAPI 4',
				voices: [
					{
						id: '11',
						name: 'MS Test voice 1',
						gender: 'male',
					},
					{
						id: '12',
						name: 'MS Test voice 2',
						gender: 'Female',
					},
				]
			},
			{
				id: '13',
				name: 'SAPI 5',
				voices: [
					{
						id: '14',
						name: 'MS Test voice 3',
						gender: 'male',
					},
					{
						id: '15',
						name: 'MS Test voice 4',
						gender: 'Female',
					},
				]
			},
		]
	},
])