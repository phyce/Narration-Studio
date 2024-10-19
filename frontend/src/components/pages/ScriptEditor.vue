<script setup lang="ts">
import '../../css/pages/script-editor.css';

import Button from 'primevue/button';
import InputGroup from 'primevue/inputgroup';
import InputText from 'primevue/inputtext';
import Editor from "../common/Editor.vue";
import { SelectDirectory, GetSettings, SaveSettings, ProcessScript } from '../../../wailsjs/go/main/App';
import { useLocalStorage } from "@vueuse/core";
import { UserSettings } from "../interfaces/settings";
import { onMounted, ref } from "vue";

const text = useLocalStorage<string>('scriptText', 'user: hello world');
const settings = ref<UserSettings>({} as UserSettings);
const regexes = [
	{ regex: /^[^\S\r\n]*([^:\r\n]+):\s*(.*?)(?=\r?\n|$)/gm, className: 'matching-sentence' },
	{ regex: /^([^\s:]+):\s?(?=\S)/gm, className: 'matching-character' },
];

const handleBrowseClick = async () => {
	const result = await SelectDirectory(settings.value.scriptOutputPath as string);
	if (result.length > 0 && settings.value.scriptOutputPath != result) {
		settings.value.scriptOutputPath =  result;

		await SaveSettings(JSON.stringify(settings.value));
	}
}

const processScript = () => {
	ProcessScript(text.value)
}

onMounted(async () => {
	const settingsString = await GetSettings();
	settings.value = JSON.parse(settingsString) as UserSettings;
});
</script>

<template>
	<div class="script">
		<div class="script__panel">
			<InputGroup :title="settings.scriptOutputPath">
				<InputText class="script__panel__input"
						   :value="settings.scriptOutputPath"
						   placeholder="Output Path"
						   disabled
				/>
				<Button class="script__panel__browse"
						@click="handleBrowseClick"
						title="Browse"
						aria-label="Browse"
				>
					<i class="pi pi-folder-open"/>
				</Button>
			</InputGroup>
			<Button class="script__panel__generate"
					@click="processScript"
					title="Generate"
					aria-label="Generate"
			>
				<i class="pi pi-upload"/>
			</Button>
		</div>
		<div class="script__editor">
			<Editor v-model:text="text" :regexes="regexes" model-value=""/>
		</div>
	</div>
</template>