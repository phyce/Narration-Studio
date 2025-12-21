<script setup lang="ts">
import '../../css/pages/settings.css';

import Button from "primevue/button";
import {onBeforeMount, ref, watch} from "vue";
import {GetConfigSchema, GetSettings, SaveSettings} from "../../../wailsjs/go/main/App";
import {config as configuration} from "../../../wailsjs/go/models";
import type {ConfigField, ConfigSchema} from "../../interfaces/config";
import RecursiveField from "../config/RecursiveField.vue";
import configBase = configuration.Base;

const config = ref<configBase>({} as configBase);
const schema = ref<ConfigSchema | null>(null);
const loading = ref<boolean>(true);
const hasUnsavedChanges = ref<boolean>(false);
const originalConfig = ref<string>('');

watch(config, () => {
	const currentConfigStr = JSON.stringify(config.value);
	if (originalConfig.value && currentConfigStr !== originalConfig.value) {
		hasUnsavedChanges.value = true;
	}
}, {deep: true});

async function loadData() {
	loading.value = true;

	// Load config
	config.value = await GetSettings();
	originalConfig.value = JSON.stringify(config.value);
	console.log('we got settings');
	console.log(config.value);

	// Load schema
	try {
		const schemaResult = await GetConfigSchema();
		schema.value = JSON.parse(schemaResult);
		console.log('Schema loaded:', schema.value);
		console.log('Server fields:', schema.value?.fields.filter(f => f.path.startsWith('settings.server')));
	} catch (err) {
		console.error('Failed to load config schema:', err);
	}

	loading.value = false;
}

async function handleSaveSettings() {
	await SaveSettings(config.value);
	originalConfig.value = JSON.stringify(config.value);
	hasUnsavedChanges.value = false;
}

function getValueByPath(path: string): any {
	const parts = path.split('.');
	let value: any = config.value;

	for (const part of parts) {
		if (value && typeof value === 'object' && part in value) {
			value = value[part];
		} else {
			return undefined;
		}
	}

	return value;
}

function setValueByPath(path: string, value: any) {
	const parts = path.split('.');
	let obj: any = config.value;

	for (let i = 0; i < parts.length - 1; i++) {
		const part = parts[i];
		if (!(part in obj)) {
			obj[part] = {};
		}
		obj = obj[part];
	}

	obj[parts[parts.length - 1]] = value;
}

// Confirm before leaving if there are unsaved changes
onBeforeMount(async () => {
	await loadData();

	window.addEventListener('beforeunload', (e) => {
		if (hasUnsavedChanges.value) {
			e.preventDefault();
			e.returnValue = '';
		}
	});
});





function getRootFields(): ConfigField[] {
	if (!schema.value?.fields) return [];

	return schema.value.fields.filter(field => {
		const parts = field.path.split('.');
		return parts.length === 1;
	});
}

</script>

<template>
	<div class="settings">
		<div class="settings__actions">
			<Button
				class="settings__actions__save"
				@click="handleSaveSettings()"
				:disabled="!hasUnsavedChanges"
				title="Save Settings"
				aria-label="Save Settings"
			>
				<i class="pi pi-save"/>&nbsp;
				Save Settings
				<span v-if="hasUnsavedChanges" class="unsaved-indicator">*</span>
			</Button>
		</div>

		<div class="settings__container" v-if="!loading && schema">
			<RecursiveField
				v-for="field in getRootFields()"
				:key="field.path"
				:field="field"
				:schema="schema"
				:get-value-by-path="getValueByPath"
				:set-value-by-path="setValueByPath"
			/>
		</div>

		<div v-else-if="loading" class="settings__loading">
			<i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
			<p>Loading settings...</p>
		</div>
	</div>
</template>

<style scoped>
.settings {
	display: flex;
	flex-direction: column;
	height: 100%;
	overflow: hidden;
}

.settings__actions {
	display: flex;
	align-items: center;
	gap: 1rem;
	padding: 1rem;
	border-bottom: 1px solid var(--surface-border);
}

.settings__actions__save {
	position: relative;
}

.unsaved-indicator {
	color: var(--yellow-500);
	font-weight: bold;
	margin-left: 0.25rem;
}

.settings__container {
	flex: 1;
	overflow-y: auto;
	padding: 1.5rem 1.5rem 1.5rem 1.5rem;
	margin-left: 0;
}

.settings__loading {
	display: flex;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	height: 100%;
	gap: 1rem;
	color: var(--text-color-secondary);
}
</style>
