<script setup lang="ts">
import Editor from '../common/Editor.vue'
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox';
import {computed, onMounted, ref} from "vue"
import {Engine, Model, Voice } from '../interfaces/engine';
import { useLocalStorage } from '@vueuse/core';
import { GetVoices, GetEngines, Play } from '../../../wailsjs/go/main/App'
import Toast from 'primevue/toast';
import { useToast } from "primevue/usetoast";
import { formatToTreeSelectData } from "../../util/util";
import TreeSelect from "primevue/treeselect";
import Dropdown from "primevue/dropdown";
const toast = useToast();

const nodes = ref<any[]>([]);
const treeNodes = ref<any[]>([]);
const engines = ref<Engine[]>([]);
const voices = ref<Voice[]>([]);
const selectedModel = ref<Model>();
const selectedVoice = ref<Voice>();
const text = useLocalStorage<string>('sandboxText', 'user: hello world');
const overrideVoices = ref<boolean>(false);
const saveNewCharacters = ref<boolean>(false);

const regexes = [
	{ regex: /^[^\S\r\n]*([^:\r\n]+):\s*(.*?)(?=\r?\n|$)/gm, className: 'matching-sentence' },
	{ regex: /^([^\s:]+):\s?(?=\S)/gm, className: 'matching-character' }
];

async function generateSpeech() {
	let voiceID = "";
	if(overrideVoices.value) {
		if (selectedModel.value === undefined || selectedVoice.value === undefined) return;
		voiceID = selectedModel.value.id + ":" + selectedVoice.value.voiceID ;
	}

	const result = await Play(text.value, (saveNewCharacters.value? true: false), voiceID);

	if (result === '') toast.add({ severity: 'success', summary: 'Success', detail: 'Generation completed', life: 3000 });
	else toast.add({ severity: 'error', summary: 'Failed to generate audio', detail: result, life: 3000});
}


//TODO: Move this and the copy in CharacterVoices into util.ts (need to have toast in here)
async function getEngines() {
	const result = await GetEngines();
	try {
		const engines: Engine[] = JSON.parse(result);

		return engines;
	} catch (error) {
		toast.add({ severity: 'error', summary: 'Error getting engines:', detail: error, life: 5000});
	}
}

async function getVoices(engine: string, model: string) {
	console.log([engine, model]);
	const result = await GetVoices(engine, model);
	console.log(result);
	try {
		const voices: Voice[] = JSON.parse(result);

		return voices;
	} catch (error) {
		toast.add({ severity: 'error', summary: 'Error getting Voices:', detail: error, life: 5000});
	}
}

async function onModelSelect(node: any) {
	voices.value = await getVoices(node.engine, node.id) ?? [];
}

onMounted(async () => {
	engines.value = await getEngines() ?? [];
	nodes.value = engines.value.map(engine => ({
		key: engine.id,
		label: engine.name,
		selectable: false,
		children: Object.entries(engine.models ?? {}).map(([modelId, modelData]) => ({
			selectable: true,
			key: modelId,
			label: modelData.name,
			data: modelData
		}))
	}));

	treeNodes.value = formatToTreeSelectData(engines.value);
});

const isDisabled = computed(() => {
	return (overrideVoices.value && selectedVoice.value === undefined);
});
</script>

<template>
	<div class="flex w-full h-full">
		<div class="w-1/5 p-2">
			<Toast position="bottom-center" />
			<Button
				@click="generateSpeech"
				class="w-full"
				icon="pi pi-play"
				title="Play All"
				aria-label="Play"
				:disabled="isDisabled"
			/>
			<TreeSelect :options="treeNodes" v-model="selectedModel" @node-select="onModelSelect" placeholder="Select a model" class="w-full mt-2" />
			<Dropdown v-model="selectedVoice" :options="voices" filter optionLabel="name" placeholder="Select a voice" class="w-full mt-2 text-left" />
			<div class="flex items-center justify-start w-full pt-1">
				<Checkbox v-model="overrideVoices" inputId="overrideVoices" name="overrideVoices" value="1" />
				<label for="overrideVoices" class="ml-2 cursor-pointer select-none"> Override Voices </label>
			</div>
			<div class="flex items-center justify-start w-full pt-1">
				<Checkbox v-model="saveNewCharacters" inputId="saveNewCharacters" name="saveNewCharacters" value="1" />
				<label for="saveNewCharacters" class="ml-2 cursor-pointer select-none"> Save new characters </label>
			</div>
		</div>
		<div class="w-4/5">
			<Editor v-model:text="text" :regexes="regexes" model-value=""/>
		</div>
	</div>
</template>