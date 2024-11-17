<script setup lang="ts">
import '../../css/pages/script-editor.css';

import Button from 'primevue/button';
import InputGroup from 'primevue/inputgroup';
import InputText from 'primevue/inputtext';
import Editor from "../common/Editor.vue";
import { SelectDirectory, GetSettings, SaveSettings, ProcessScript } from '../../../wailsjs/go/main/App';
import { useLocalStorage } from "@vueuse/core";
import { onMounted, ref } from "vue";
import {config as configuration} from "../../../wailsjs/go/models";
import configBase = configuration.Base;

const text = useLocalStorage<string>('scriptText', 'user: hello world');
const config = ref<configBase>({} as configBase);
const loading = ref<boolean>(true);

const regexes = [
	{ regex: /^[^\S\r\n]*([^:\r\n]+):\s*(.*?)(?=\r?\n|$)/gm, className: 'matching-sentence' },
	{ regex: /^([^\s:]+):\s?(?=\S)/gm, className: 'matching-character' },
];

const handleBrowseClick = async () => {
	const result = await SelectDirectory(config.value.settings.outputPath as string);
	if (result.length > 0 && config.value.settings.outputPath != result) {
		config.value.settings.outputPath =  result;

		await SaveSettings(JSON.stringify(config.value));
	}
}

const processScript = () => {
	ProcessScript(text.value)
}

onMounted(async () => {
	config.value = await GetSettings();
	loading.value = false;
});
</script>

<template>
	<div class="script" v-if="!loading">
		<div class="script__panel">
			<InputGroup v-if="!loading" :title="config.settings.outputPath">
				<InputText class="script__panel__input"
						   :value="config.settings.outputPath"
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