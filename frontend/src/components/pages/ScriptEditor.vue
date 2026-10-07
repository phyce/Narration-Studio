<script setup lang="ts">
import '../../css/pages/sandbox.css';

import Button from 'primevue/button';
import InputGroup from 'primevue/inputgroup';
import InputText from 'primevue/inputtext';
import Editor from "../common/Editor.vue";
import ProfileSelector from '../common/ProfileSelector.vue';
import {
	GetSettings,
	ProcessScript,
	SaveSettings,
	SelectDirectory,
	SetPlaybackPaused,
	StopPlayback
} from '../../../bindings/nstudio/app';
import {useLocalStorage} from "@vueuse/core";
import {onMounted, ref} from "vue";
import * as configuration from "../../../bindings/nstudio/app/config/models";
import configBase = configuration.Base;

const text = useLocalStorage<string>('scriptText', 'user: hello world');
const selectedProfile = useLocalStorage<string>('scriptProfile', 'default');
const config = ref<configBase>({} as configBase);
const loading = ref<boolean>(true);
const isProcessing = ref<boolean>(false);
const isPaused = ref<boolean>(false);

const regexes = [
	{regex: /^[^\S\r\n]*([^:\r\n]+):\s*(.*?)(?=\r?\n|$)/gm, className: 'matching-sentence'},
	{regex: /^[^\S\r\n]*([^:\r\n]+?):\s?(?=\S)/gm, className: 'matching-character'},
];

const handleBrowseClick = async () => {
	const result = await SelectDirectory(config.value.settings.outputPath as string);
	if (result.length > 0 && config.value.settings.outputPath != result) {
		config.value.settings.outputPath = result;

		await SaveSettings(config.value);
	}
}

async function processScript() {
	isProcessing.value = true;
	isPaused.value = false;
	try {
		await ProcessScript(text.value, selectedProfile.value);
	} finally {
		isProcessing.value = false;
		isPaused.value = false;
	}
}

function togglePause() {
	isPaused.value = !isPaused.value;
	SetPlaybackPaused(isPaused.value);
}

function stopProcessing() {
	StopPlayback();
}

onMounted(async () => {
	config.value = await GetSettings();
	loading.value = false;
});
</script>

<template>
	<div class="page" v-if="!loading">
		<aside class="page-sidebar">
			<div class="field">
				<label class="field__label">Profile</label>
				<ProfileSelector v-model="selectedProfile"/>
			</div>

			<div class="field">
				<label class="field__label" for="script-output-path">Output folder</label>
				<InputGroup :title="config.settings.outputPath">
					<InputText id="script-output-path"
							   :value="config.settings.outputPath"
							   placeholder="Not set"
							   readonly
					/>
					<Button icon="pi pi-folder-open"
							@click="handleBrowseClick"
							title="Choose the output folder"
							aria-label="Choose the output folder"
					/>
				</InputGroup>
			</div>

			<Button v-if="!isProcessing"
					icon="pi pi-upload"
					label="Generate audio"
					@click="processScript"
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
						@click="stopProcessing"
				/>
			</div>
		</aside>
		<div class="sandbox__editor">
			<Editor v-model:text="text" :regexes="regexes"/>
		</div>
	</div>
</template>
