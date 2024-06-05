import {Engine, Model, Voice } from '../components/interfaces/engine';

export function findById(id: string, engines: Engine[]): Engine | Model | Voice | undefined {
	for (const engine of engines) {
		if (engine.id === id) return engine;

		if(engine.models !== undefined) for (const model of engine.models) {
			if (model.id === id) return model;

			// if(model.voices !== undefined) for (const voice of model.voices) {
			// 	if (voice.id === id) return voice;
			// }
		}
	}
	return undefined;
}

export function formatToTreeSelectData(engines: Engine[]) {
	return engines.map(engine => ({
		key: `engine-${engine.id}`,
		id: engine.id,
		label: engine.name,
		data: engine.name,
		selectable:false,
		icon: 'pi pi-fw pi-folder',
		children: engine.models?.map(model => ({
			key: `model-${model.id}`,
			id: model.id,
			label: model.name,
			data: model.name,
			engine: engine.name,
			selectable:true,
			icon: 'pi pi-fw pi-volume-up'
		}))
	}));
}