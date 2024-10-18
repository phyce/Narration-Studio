<script setup lang="ts">
import '../../css/pages/settings.css';

import InputText from "primevue/inputtext";
import InputGroup from "primevue/inputgroup";
import InputGroupAddon from "primevue/inputgroupaddon";
import Button from "primevue/button";
import Dropdown from "primevue/dropdown";
import {onMounted, ref} from "vue";
import { GetSettings, SaveSettings } from "../../../wailsjs/go/main/App";
import { UserSettings } from "../interfaces/settings";
import { OutputTypeOptions } from "../enums/outputType";

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
	<div class="settings">
		<div class="settings__actions">
			<Button class="settings__actions__save"
					@click="handleSaveSettings()"
					title="Save Settings"
					aria-label="Save Settings"
			>
				<i class="pi pi-save"/>&nbsp;
				Save Settings
			</Button>
		</div>
		<div class="settings__container">
			<InputGroup class="input-group">
				<InputGroupAddon class="input-group__addon">Piper path</InputGroupAddon>
				<InputText class="input-group__input"
						   :value="settings.piperPath"
						   placeholder="Select a directory"
						   disabled
				/>
				<Button class="input-group__button" title="Browse" aria-label="Browse" >
					<i class="pi pi-folder-open"/>
				</Button>
			</InputGroup>
			<InputGroup class="input-group">
				<InputGroupAddon class="input-group__addon">Models directory</InputGroupAddon>
				<InputText class="input-group__input"
						   :value="settings.piperModelsDirectory"
						   placeholder="Output Path"
						   disabled
				/>
				<Button class="input-group__button" title="Browse" aria-label="Browse">
					<i class="pi pi-folder-open"/>
				</Button>
			</InputGroup>
			<InputGroup class="input-group">
				<InputGroupAddon class="input-group__addon">Output Type</InputGroupAddon>
				<Dropdown class="input-group__dropdown"
						  v-model="settings.outputType"
						  :options="OutputTypeOptions"
						  inputId="outputType"
						  optionLabel="label"
						  placeholder="select type"
				/>
			</InputGroup>
			<InputGroup class="input-group">
				<InputGroupAddon class="input-group__addon">OpenAI API Key</InputGroupAddon>
				<InputText class="input-group__input" v-model="settings.openAiApiKey" />
			</InputGroup>
			<InputGroup class="input-group">
				<InputGroupAddon class="input-group__addon">Elevenlabs API Key</InputGroupAddon>
				<InputText class="input-group__input" v-model="settings.elevenlabsApiKey" />
			</InputGroup>
		</div>
	</div>
</template>