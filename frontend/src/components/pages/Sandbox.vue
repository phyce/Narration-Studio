<script setup lang="ts">
import '../../css/pages/sandbox.css';

import Editor from '../common/Editor.vue';
import Button from 'primevue/button';
import Checkbox from 'primevue/checkbox';
import { computed, onMounted, ref } from "vue"
import { Engine, Model, Voice } from '../interfaces/engine';
import { useLocalStorage } from '@vueuse/core';
import { GetVoices, GetEngines, Play } from '../../../wailsjs/go/main/App';
import { formatToTreeSelectData } from "../../util/util";
import TreeSelect from "primevue/treeselect";
import Dropdown from "primevue/dropdown";

const modelVoiceTree = ref<any[]>([]);
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

const isDisabled = computed(() => {
	return (overrideVoices.value && selectedVoice.value === undefined);
});

async function generateSpeech() {
	let voiceID = "";
	if(overrideVoices.value) {
		if (selectedModel.value === undefined || selectedVoice.value === undefined) return;

		voiceID = "::" + Object.keys(selectedModel.value)[0] + ":" + selectedVoice.value.voiceID;
	}

	await Play(text.value, (!!saveNewCharacters.value), voiceID);
}

async function getEngines() {
	const result = await GetEngines();
	const engines: Engine[] = JSON.parse(result);

	return engines;
}

async function getVoices(engine: string, model: string) {
	const result = await GetVoices(engine, model);
	const voices: Voice[] = JSON.parse(result);

	return voices;
}

async function onModelSelect(node: any) {
	voices.value = await getVoices(node.engine, node.id) ?? [];
}

onMounted(async () => {
	engines.value = await getEngines() ?? [];

	modelVoiceTree.value = formatToTreeSelectData(engines.value);
});
</script>

<template>
	<div class="sandbox">
		<div class="sandbox__panel">
			<Button class="sandbox__panel__generate"
					title="Play All"
					aria-label="Play"
					:disabled="isDisabled"
					@click="generateSpeech"
			>
				<i class="pi pi-play"/>
			</Button>
			<TreeSelect class="sandbox__panel__model-tree"
						:options="modelVoiceTree"
						v-model="selectedModel"
						@node-select="onModelSelect"
						placeholder="Select a model"
			/>
			<Dropdown class="sandbox__panel__voice-dropdown"
					  v-model="selectedVoice"
					  :options="voices"
					  filter
					  optionLabel="name"
					  placeholder="Select a voice"
			/>

			<div class="sandbox__panel__checkbox">
				<Checkbox class="checkbox"
						  v-model="overrideVoices"
						  inputId="overrideVoices"
						  name="overrideVoices"
						  :binary="true"
				/>
				<label class="checkbox-label" for="overrideVoices">
					Override Voices
				</label>
			</div>
			<div class="sandbox__panel__checkbox">
				<Checkbox class="checkbox"
						  v-model="saveNewCharacters"
						  inputId="saveNewCharacters"
						  name="saveNewCharacters"
						  value="1"
				/>
				<label class="checkbox-label" for="saveNewCharacters">
					Save new characters
				</label>
			</div>
		</div>
		<div class="sandbox__editor">
			<Editor v-model:text="text" :regexes="regexes"/>
		</div>
	</div>
</template>