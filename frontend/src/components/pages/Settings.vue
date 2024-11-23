<script setup lang="ts">
import '../../css/pages/settings.css';

import InputText from "primevue/inputtext";
import InputGroup from "primevue/inputgroup";
import InputSwitch from 'primevue/inputswitch';
import InputGroupAddon from "primevue/inputgroupaddon";
import Button from "primevue/button";
import Dropdown from "primevue/dropdown";
import {onBeforeMount, onMounted, reactive, ref} from "vue";
import {GetSettings, SaveSettings, SelectDirectory, SelectFile} from "../../../wailsjs/go/main/App";
import { OutputTypeOptions } from "../enums/outputType";
import {config as configuration} from "../../../wailsjs/go/models";
import configBase = configuration.Base;

const config = reactive<configBase>({} as configBase);
const loading = ref<boolean>(true);

function handleSaveSettings() {
	SaveSettings(config);
}

const handlePiperEngineLocationSelect = async () => {
	const result = await SelectFile(config.engine.local.piper.directory as string);
	if (result.length > 0 && config.engine.local.piper.directory != result) {
		config.engine.local.piper.directory =  result;
	}
};

const handlePiperModelLocationSelect = async () => {
	const result = await SelectDirectory(config.engine.local.piper.modelsDirectory as string);
	if (result.length > 0 && config.engine.local.piper.modelsDirectory != result) {
		config.engine.local.piper.modelsDirectory =  result;
	}
}

onBeforeMount( async () => {
	Object.assign(config, await GetSettings());
	loading.value = false;
})
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
			<div class="settings_actions_debug">
				<InputSwitch
					v-model="config.settings.debug"
					:true-value="true"
					:false-value="false"
				/>
				<label class="checkbox-label" for="overrideVoices">
					Debug Mode
				</label>
			</div>
		</div>
		<div class="settings__container" v-if="!loading">
			<InputGroup class="input-group">
				<InputGroupAddon class="input-group__addon">Piper path</InputGroupAddon>
				<InputText class="input-group__input"
						   v-model="config.engine.local.piper.directory"
						   placeholder="Select a directory"
						   disabled
				/>
				<Button class="input-group__button"
						title="Browse"
						aria-label="Browse"
						@click="handlePiperEngineLocationSelect"
				>
					<i class="pi pi-folder-open"/>
				</Button>
			</InputGroup>
			<InputGroup class="input-group">
				<InputGroupAddon class="input-group__addon">Models directory</InputGroupAddon>
				<InputText class="input-group__input"
						   v-model="config.engine.local.piper.modelsDirectory"
						   placeholder="Output Path"
						   disabled
				/>
				<Button class="input-group__button"
						title="Browse"
						aria-label="Browse"
						@click="handlePiperModelLocationSelect"
				>
					<i class="pi pi-folder-open"/>
				</Button>
			</InputGroup>
			<InputGroup class="input-group">
				<InputGroupAddon class="input-group__addon">Output Type</InputGroupAddon>
				<Dropdown class="input-group__dropdown"
						  v-model="config.settings.outputType"
						  :options="OutputTypeOptions"
						  inputId="outputType"
						  optionLabel="label"
						  optionValue="value"
						  placeholder="select type"
				/>
			</InputGroup>
			<InputGroup class="input-group">
				<InputGroupAddon class="input-group__addon">OpenAI API Key</InputGroupAddon>
				<InputText class="input-group__input" v-model="config.engine.api.openAI.apiKey" />
			</InputGroup>
			<InputGroup class="input-group">
				<InputGroupAddon class="input-group__addon">Elevenlabs API Key</InputGroupAddon>
				<InputText class="input-group__input" v-model="config.engine.api.elevenLabs.apiKey" />
			</InputGroup>
		</div>
	</div>
</template>