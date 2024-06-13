<script setup lang="ts">
import Button from 'primevue/button'
import InputGroup from 'primevue/inputgroup'
import InputText from 'primevue/inputtext'
import Editor from "../common/Editor.vue";
import {Play, SelectDirectory, GetSettings, SaveSettings, ProcessScript} from '../../../wailsjs/go/main/App'
import {useLocalStorage} from "@vueuse/core";
import {UserSettings} from "../interfaces/settings";
import {useToast} from "primevue/usetoast";
import {onMounted, ref} from "vue";
const toast = useToast();

const text = useLocalStorage<string>('scriptText', 'user: hello world');
const settings = ref<UserSettings>({} as UserSettings);

const regexes = [
	{ regex: /^[^\S\r\n]*([^:\r\n]+):\s*(.*?)(?=\r?\n|$)/gm, className: 'matching-sentence' },
	{ regex: /^([^\s:]+):\s?(?=\S)/gm, className: 'matching-character' },
];

const handleBrowseClick = async () => {
	try {
		settings.value.scriptOutputPath = await SelectDirectory(settings.value.scriptOutputPath as string);

		await SaveSettings(JSON.stringify(settings.value)).then(() => {
			toast.add({ severity: 'success', summary: 'Success', detail: 'Output path updated.', life: 3000 });
		});

	} catch (error) {
		toast.add({ severity: 'error', summary: 'Failed to read settings', detail: error, life: 3000});
	}
}

const processScript = async () => {
	const result = await ProcessScript(text.value)
}

onMounted(async () => {
	console.log("OnMounted is running");
	try {
		const settingsString = await GetSettings();
		settings.value = JSON.parse(settingsString) as UserSettings;
		console.log(settings.value);
	} catch (error) {
		toast.add({ severity: 'error', summary: 'Failed to read settings', detail: error, life: 5000});
	}
});
</script>

<template>
	<div class="flex w-full h-full">
		<div class="w-1/5 p-2">
			<InputGroup class="" :title="settings.scriptOutputPath">
				<InputText
					:value="settings.scriptOutputPath"
					placeholder="Output Path"
					class="disabled:bg-neutral-800"
					disabled
				/>
				<Button
					@click="handleBrowseClick"
					icon="pi pi-folder-open"
					title="Browse"
					aria-label="Browse"
				/>
			</InputGroup>
			<Button
				@click="processScript"
				class="w-full mt-2"
				icon="pi pi-upload"
				title="Generate"
				aria-label="Generate"
			/>
		</div>
		<div class="w-4/5">
			<Editor v-model:text="text" :regexes="regexes" model-value=""/>
		</div>
	</div>
</template>