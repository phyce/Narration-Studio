import {Engine, Model, Voice } from '../components/interfaces/engine';
// import {GetEngines} from "../../wailsjs/go/main/App";
// import {useToast} from "primevue/usetoast";
// const toast = useToast();

export function formatToTreeSelectData(engines: Engine[]) {
	return engines.map(engine => ({
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

// export async function getEngines() {
// 	const result = await GetEngines();
// 	try {
// 		const engines: Engine[] = JSON.parse(result);
//
// 		return engines;
// 	} catch (error) {
// 		toast.add({ severity: 'error', summary: 'Error getting engines:', detail: error, life: 5000});
// 	}
// }