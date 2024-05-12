<script setup lang="ts">
import InputText from 'primevue/inputtext';
import Button from "primevue/button";
import Dropdown from "primevue/dropdown";
import TreeSelect from "primevue/treeselect";
import {computed, ref, watch} from "vue";

interface Engine {
	id: number;
	name: string;
	models: Model[];
}
interface Model {
	id: number;
	name: string;
	voices: Voice[];
}
interface Voice {
	id: number;
	name: string;
	gender: string;
}

const engines = ref<Engine[]>([
	{
		id: 1,
		name: 'Piper',
		models: [
			{
				id: 2,
				name: 'LibriTTS',
				voices: [
					{
						id: 23,
						name: 'Piper Test voice 1',
						gender: 'male',
					},
					{
						id: 24,
						name: 'Piper Test voice 2',
						gender: 'Female',
					},
					{
						id: 25,
						name: 'Piper Test voice 3',
						gender: 'Female',
					},
					{
						id: 26,
						name: 'Piper Test voice 4',
						gender: 'Female',
					},
					{
						id: 27,
						name: 'Piper Test voice 5',
						gender: 'Female',
					},
					{
						id: 28,
						name: 'Piper Test voice 6',
						gender: 'Female',
					},
					{
						id: 29,
						name: 'Piper Test voice 8',
						gender: 'Female',
					},
					{
						id: 210,
						name: 'Piper Test voice 2',
						gender: 'Female',
					},
					{
						id: 211,
						name: 'Piper Test voice 2',
						gender: 'Female',
					},
					{
						id: 212,
						name: 'Piper Test voice 2',
						gender: 'Female',
					},
					{
						id: 213,
						name: 'Piper Test voice 2',
						gender: 'Female',
					},
				]
			},
		]
	},
	{
		id: 5,
		name: 'Suno Bark',
		models: [
			{
				id: 6,
				name: 'Default',
				voices: [
					{
						id: 7,
						name: 'Suno Test voice 1',
						gender: 'male',
					},
					{
						id: 8,
						name: 'Suno Test voice 2',
						gender: 'Female',
					},
				]
			},
		]
	},
	{
		id: 9,
		name: 'Microsoft',
		models: [
			{
				id: 10,
				name: 'SAPI 4',
				voices: [
					{
						id: 11,
						name: 'MS Test voice 1',
						gender: 'male',
					},
					{
						id: 12,
						name: 'MS Test voice 2',
						gender: 'Female',
					},
				]
			},
			{
				id: 13,
				name: 'SAPI 5',
				voices: [
					{
						id: 14,
						name: 'MS Test voice 3',
						gender: 'male',
					},
					{
						id: 15,
						name: 'MS Test voice 4',
						gender: 'Female',
					},
				]
			},
		]
	},
])

function findById(id: number, engines: Engine[]): Engine | Model | Voice | undefined {
	for (const engine of engines) {
		if (engine.id === id) {
			return engine;
		}
		for (const model of engine.models) {
			if (model.id === id) {
				return model;
			}
			for (const voice of model.voices) {
				if (voice.id === id) {
					return voice;
				}
			}
		}
	}
	return undefined;  // Return undefined if the id is not found
}

function formatToTreeSelectData(engines: Engine[]) {
	return engines.map(engine => ({
		key: `engine-${engine.id}`,
		id: engine.id,
		label: engine.name,
		data: engine.name,
		selectable:false,
		icon: 'pi pi-fw pi-folder',
		children: engine.models.map(model => ({
			key: `model-${model.id}`,
			id: model.id,
			label: model.name,
			data: model.name,
			selectable:true,
			icon: 'pi pi-fw pi-cog'
		}))
	}));
}

const treeNodes = formatToTreeSelectData(engines.value);

const selectedModel = ref<Model>();
const selectedVoice = ref<Voice>();
const voices = ref<Voice[]>([]);

const nodes = engines.value.map(engine => ({
	key: engine.id,
	label: engine.name,
	selectable: false,
	children: engine.models.map(model => ({
		selectable: true,
		key: model.id,
		label: model.name,
		data: model
	}))
}));

//Get selected node,
function onModelSelect(node: any) {
	console.log('updating voices');
	console.log(node);

	const selected = findById(node.id, engines.value);
	console.log(selected);
	if (selected && 'voices' in selected) {  // Type guard to check if it is a Model
		console.log('Selected model:', selected);
		voices.value = selected.voices;  // Update the voices ref
	}
}

</script>

<template>
	<div class="flex flex-col w-full h-full">
		<div class="w-full px-2 mb-2 flex">
			<Button class="mt-2 mr-2" icon="pi pi-save" title="Save All" label="Save All" aria-label="Save All" />
			<Button class="mt-2 button-start" icon="pi pi-power-off" title="Start Preview" label="Start Preview" aria-label="Start Preview" />
		</div>
		<div class="flex-grow background-secondary flex">
			<div class="w-3/12 p-2">
				<InputText class="w-full" type="text"  placeholder="Character" />
			</div>
			<div class="w-3/12">
				<TreeSelect :options="treeNodes" v-model="selectedModel" @node-select="onModelSelect" placeholder="Select a model" class="w-full mt-2" />
			</div>
			<div class="w-4/12">
				<Dropdown v-model="selectedVoice" :options="voices" filter optionLabel="name" placeholder="Select a voice" class="w-full ml-2 mt-2" />
			</div>
			<div class="w-2/12 pl-2 flex flex-col">
				<div>
					<Button class="mt-2 mr-2 inline-block button-start" icon="pi pi-volume-up" title="Preview" aria-label="Preview" />
					<Button class="mt-2 inline-block button-stop" icon="pi pi-trash" title="Remove" aria-label="Remove" />
				</div>
			</div>
		</div>
	</div>
</template>

<style scoped>

</style>