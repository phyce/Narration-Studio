<script setup lang="ts">

import InputText from "primevue/inputtext";
import InputGroup from "primevue/inputgroup";
import InputGroupAddon from "primevue/inputgroupaddon";
import Button from "primevue/button";
import Dropdown from "primevue/dropdown";
import {onMounted, ref} from "vue";
import { GetSettings, SaveSettings } from "../../../wailsjs/go/main/App"
import { UserSettings } from "../interfaces/settings"
import Toast from 'primevue/toast';
import { useToast } from "primevue/usetoast";
const toast = useToast();

interface OutputType {
	value: number;
	name: string;
	label: string;
}

const outputTypes: OutputType[] = [
	{
		value: 0,
		name: 'Combined File',
		label: 'Combined File'
	},
	{
		value: 1,
		name: 'Split Files',
		label: 'Split Files'
	}
];

const settings = ref<UserSettings>({} as UserSettings);

function handleSaveSettings() {
	SaveSettings(JSON.stringify(settings.value));
}

onMounted(async () => {
	settings.value = JSON.parse(await GetSettings()) as UserSettings;
	settings.value.outputType = JSON.parse(settings.value.outputType as string);
});
</script>

<template>
	<div class="flex flex-col w-full h-full">
		<Toast position="bottom-center" />
		<div class="w-full px-2 mb-2 flex">
			<Button
				@click="handleSaveSettings()"
				class="mt-2 mr-2"
				icon="pi pi-save"
				title="Save Settings"
				label="Save Settings"
				aria-label="Save Settings"
			/>
		</div>
		<div class="flex-grow background-secondary flex flex-col p-2">
			<InputGroup class="mb-2 flex">
				<InputGroupAddon class="w-1/6">Piper path</InputGroupAddon>
				<InputText
					:value="settings.piperPath"
					placeholder="Select a directory"
					class="flex-grow disabled:bg-neutral-800"
					disabled
				/>
				<Button icon="pi pi-folder-open w-1/6" title="Browse" aria-label="Browse" />
			</InputGroup>
			<InputGroup class="mb-2 flex">
				<InputGroupAddon class="w-1/6">Models directory</InputGroupAddon>
				<InputText
					:value="settings.piperModelsDirectory"
					placeholder="Output Path"
					class="flex-grow disabled:bg-neutral-800"
					disabled />
				<Button icon="pi pi-folder-open w-1/6" title="Browse" aria-label="Generate" />
			</InputGroup>
			<InputGroup class="mb-2 flex">
				<InputGroupAddon class="w-1/6">Output Type</InputGroupAddon>
				<Dropdown
					v-model="settings.outputType"
					:options="outputTypes"
					inputId="outputType"
					optionLabel="label"
					placeholder="select type"
					class="flex-grow"
				/>
			</InputGroup>
		</div>
	</div>
</template>

<style scoped>

</style>