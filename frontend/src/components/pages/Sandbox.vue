<script setup lang="ts">
import Editor from '../common/Editor.vue'
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox';
import {onMounted, ref} from "vue"
import { /*Engine, Model, Voice,*/} from '../common/voiceData';
import {Engine, Model, Voice } from '../interfaces/engine';
import { useLocalStorage } from '@vueuse/core';
import {/*GetEngineVoiceData,*/ GetEngines,  Play} from '../../../wailsjs/go/main/App'
import Toast from 'primevue/toast';
import { useToast } from "primevue/usetoast";
import {/*findById,*/ formatToTreeSelectData} from "../../util/util";
import TreeSelect from "primevue/treeselect";
import Dropdown from "primevue/dropdown";
const toast = useToast();

const selectedModel = ref<Model>();
const selectedVoice = ref<Voice>();
const voices = ref<Voice[]>([]);
const text = useLocalStorage<string>('sandboxText', 'user: hello world');
const overrideVoices = ref<boolean>(false);
const saveNewCharacters = ref<boolean>(false);

const regexes = [
	{ regex: /^[^\S\r\n]*([^:\r\n]+):\s*(.*?)(?=\r?\n|$)/gm, className: 'matching-sentence' },
	{ regex: /^([^\s:]+):\s?(?=\S)/gm, className: 'matching-character' }
];

async function generateSpeech() {
	console.log(text.value);
	const result = await Play(text.value);
	console.log(result);
	if (result === '') toast.add({ severity: 'success', summary: 'Success', detail: 'Generation completed', life: 3000 });
}

async function getEngines() {
	// console.log("engines");
	const result = await GetEngines();
	// console.log(result);
	try {
		// Parse the result string into JSON
		const engines: Engine[] = JSON.parse(result);

		// Log the parsed JSON
		console.log(engines);

		// Optionally, handle success notification
		// if (engines.length > 0) {
			// Assuming toast is defined and configured somewhere in your code
			// toast.add({ severity: 'success', summary: 'Success', detail: 'Execution completed', life: 3000 });
		// }
		return engines;
	} catch (error) {
		// Handle JSON parsing error
		console.error("Error parsing JSON:", error);
	}
}

function onModelSelect(node: any) {

	return;
	//When a node gets selected this gets triggered
	//Reload voices list
	//get from backend
	//fill the searchable dropdown with data
	// const selected = findById(node.id, engines.value);
	// console.log(selected);
	// if (selected && 'voices' in selected && selected.voices) {
	// 	voices.value = selected.voices as unknown as Voice[];
	// }
}

// const engines = await getEngines() as unknown as Engine[];
//
// const nodes = engines.map(engine => ({
// 	key: engine.id,
// 	label: engine.name,
// 	selectable: false,
// 	children: engine.models?.map(model => ({
// 		selectable: true,
// 		key: model.id,
// 		label: model.name,
// 		data: model
// 	}))
// }));
const nodes = ref<any[]>([]);
const treeNodes = ref<any[]>([]);
const engines = ref<Engine[]>([]);

onMounted(async () => {
	engines.value = await getEngines() as unknown as Engine[];
	nodes.value = engines.value.map(engine => ({
		key: engine.id,
		label: engine.name,
		selectable: false,
		children: engine.models?.map(model => ({
			selectable: true,
			key: model.id,
			label: model.name,
			data: model
		}))
	}));

	treeNodes.value = formatToTreeSelectData(engines.value);
});


</script>

<template>
	<div class="flex w-full h-full">
		<div class="w-1/5 p-2">
			<Toast position="bottom-center" />
			<Button @click="generateSpeech" class="w-full" icon="pi pi-play" title="Play All" aria-label="Play" />
			<TreeSelect :options="treeNodes" v-model="selectedModel" @node-select="onModelSelect" placeholder="Select a model" class="w-full mt-2" />
			<Dropdown v-model="selectedVoice" :options="voices" filter optionLabel="name" placeholder="Select a voice" class="w-full mt-2" />
			<div class="flex items-center justify-start w-full pt-1">
				<Checkbox v-model="overrideVoices" inputId="overrideVoices" name="overrideVoices" value="1" />
				<label for="overrideVoices" class="ml-2 cursor-pointer select-none"> Override Voices </label>
			</div>
			<Button @click="getEngines" class="w-full" icon="pi pi-send" title="Geg Engine Voices" aria-label="Get Engine Voices" />
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