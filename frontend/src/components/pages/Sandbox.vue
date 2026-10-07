<script setup lang="ts">
import '../../css/pages/sandbox.css';

import Editor from '../common/Editor.vue';
import ProfileSelector from '../common/ProfileSelector.vue';
import Button from 'primevue/button';
import Checkbox from 'primevue/checkbox';
import {computed, onMounted, ref} from "vue"
import {Engine, Model, Voice} from '../interfaces/engine';
import {useLocalStorage} from '@vueuse/core';
import {GetEngines, GetModelVoices, Play, SetPlaybackPaused, StopPlayback} from '../../../bindings/nstudio/app';
import {formatToTreeSelectData} from "../../util/util";
import TreeSelect from "primevue/treeselect";
import Dropdown from "primevue/dropdown";

const modelVoiceTree = ref<any[]>([]);
const engines = ref<Engine[]>([]);
const voices = ref<Voice[]>([]);
const selectedModel = ref<Model>();
const selectedVoice = ref<Voice>();
const selectedProfile = useLocalStorage<string>('sandboxProfile', 'default');
const text = useLocalStorage<string>('sandboxText', 'user: hello world');
const overrideVoices = ref<boolean>(false);
const isPlaying = ref<boolean>(false);
const isPaused = ref<boolean>(false);

const regexes = [
	{regex: /^[^\S\r\n]*([^:\r\n]+):\s*(.*?)(?=\r?\n|$)/gm, className: 'matching-sentence'},
	{regex: /^[^\S\r\n]*([^:\r\n]+?):\s?(?=\S)/gm, className: 'matching-character'}
];

const isDisabled = computed(() => {
	return (overrideVoices.value && selectedVoice.value === undefined);
});

async function generateSpeech() {
	let voiceID = "";
	if (overrideVoices.value) {
		if (selectedModel.value === undefined || selectedVoice.value === undefined) return;

		voiceID = "::" + Object.keys(selectedModel.value)[0] + ":" + selectedVoice.value.voiceID;
	}

	isPlaying.value = true;
	isPaused.value = false;
	try {
		await Play(text.value, false, voiceID, selectedProfile.value);
	} finally {
		isPlaying.value = false;
		isPaused.value = false;
	}
}

function togglePause() {
	isPaused.value = !isPaused.value;
	SetPlaybackPaused(isPaused.value);
}

function stopPlayback() {
	StopPlayback();
}

async function getEngines() {
	const result = await GetEngines();
	const engines: Engine[] = JSON.parse(result);

	return engines;
}

async function getModelVoices(engine: string, model: string) {
	const result = await GetModelVoices(engine, model);
	const voices: Voice[] = JSON.parse(result);

	return voices;
}

async function onModelSelect(node: any) {
	voices.value = await getModelVoices(node.engine, node.id) ?? [];
}

onMounted(async () => {
	engines.value = await getEngines() ?? [];

	modelVoiceTree.value = formatToTreeSelectData(engines.value);
});
</script>

<template>
	<div class="page">
		<aside class="page-sidebar">
			<div class="field">
				<label class="field__label">Profile</label>
				<ProfileSelector v-model="selectedProfile"/>
			</div>

			<Button v-if="!isPlaying"
					icon="pi pi-play"
					:label="isDisabled ? 'Select a voice first' : 'Play'"
					:disabled="isDisabled"
					@click="generateSpeech"
			/>
			<div v-else class="sandbox__playback">
				<Button :icon="isPaused ? 'pi pi-play' : 'pi pi-pause'"
						:label="isPaused ? 'Resume' : 'Pause'"
						severity="secondary"
						@click="togglePause"
				/>
				<Button icon="pi pi-stop"
						label="Stop"
						severity="danger"
						@click="stopPlayback"
				/>
			</div>

			<div class="sandbox__override">
				<div class="check-row">
					<Checkbox v-model="overrideVoices" inputId="overrideVoices" :binary="true"/>
					<label for="overrideVoices">Override voices</label>
				</div>
				<template v-if="overrideVoices">
					<TreeSelect :options="modelVoiceTree"
								v-model="selectedModel"
								@node-select="onModelSelect"
								placeholder="Select a model"
								class="w-full"
					/>
					<Dropdown v-model="selectedVoice"
							  :options="voices"
							  filter
							  optionLabel="name"
							  placeholder="Select a voice"
							  class="w-full"
					/>
				</template>
			</div>
		</aside>
		<div class="sandbox__editor">
			<Editor v-model:text="text" :regexes="regexes"/>
		</div>
	</div>
</template>
