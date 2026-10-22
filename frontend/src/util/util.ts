import {Engine} from '../components/interfaces/engine';

export function formatToTreeSelectData(engines: Engine[]) {
	return engines
		.filter(engine => Object.keys(engine.models ?? {}).length > 0)
		.map(engine => ({
			key: `engine:${engine.id}`,
			id: engine.id,
			label: engine.name,
			data: engine.name,
			selectable: false,
			icon: 'pi pi-fw pi-folder',
			children: Object.values(engine.models ?? {}).map(model => ({
				key: `${engine.id}:${model.id}`,
				id: model.id,
				label: model.name,
				data: model.name,
				engine: engine.id,
				selectable: true,
				icon: 'pi pi-fw pi-volume-up'
			}))
		}));
}
