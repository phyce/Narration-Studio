<script setup lang="ts">
import Button from "primevue/button";
import {computed, onBeforeMount, onBeforeUnmount, ref} from "vue";
import {GetConfigSchema, GetSettings, SaveSettings} from "../../../bindings/nstudio/app";
import * as configuration from "../../../bindings/nstudio/app/config/models";
import type {ConfigField, ConfigSchema} from "../../interfaces/config";
import RecursiveField from "../config/RecursiveField.vue";
import configBase = configuration.Base;

const config = ref<configBase>({} as configBase);
const schema = ref<ConfigSchema | null>(null);
const loading = ref<boolean>(true);
const saving = ref<boolean>(false);
const originalConfig = ref<string>('');

const hasUnsavedChanges = computed(() =>
	!!originalConfig.value && JSON.stringify(config.value) !== originalConfig.value);

const rootFields = computed<ConfigField[]>(() =>
	schema.value?.fields.filter(field => !field.path.includes('.')) ?? []);

async function loadData() {
	loading.value = true;

	config.value = await GetSettings();
	originalConfig.value = JSON.stringify(config.value);

	try {
		schema.value = JSON.parse(await GetConfigSchema());
	} catch (err) {
		console.error('Failed to load config schema:', err);
	}

	loading.value = false;
}

async function handleSaveSettings() {
	saving.value = true;
	try {
		await SaveSettings(config.value);
		originalConfig.value = JSON.stringify(config.value);
	} finally {
		saving.value = false;
	}
}

function revertChanges() {
	config.value = JSON.parse(originalConfig.value);
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

function warnBeforeClosing(event: BeforeUnloadEvent) {
	if (hasUnsavedChanges.value) {
		event.preventDefault();
		event.returnValue = '';
	}
}

onBeforeMount(async () => {
	window.addEventListener('beforeunload', warnBeforeClosing);
	await loadData();
});

onBeforeUnmount(() => {
	window.removeEventListener('beforeunload', warnBeforeClosing);
});
</script>

<template>
	<div class="page page--column">
		<div class="page-toolbar">
			<div class="page-toolbar__spacer"/>
			<span v-if="hasUnsavedChanges" class="unsaved-note">
				<i class="pi pi-circle-fill" style="font-size: 0.45rem"/> Unsaved changes
			</span>
			<Button
				v-if="hasUnsavedChanges"
				label="Revert"
				severity="secondary"
				text
				:disabled="saving"
				@click="revertChanges"
			/>
			<Button
				icon="pi pi-save"
				label="Save"
				:loading="saving"
				:disabled="!hasUnsavedChanges || saving"
				@click="handleSaveSettings"
			/>
		</div>

		<div class="page-body config-fields" v-if="!loading && schema">
			<RecursiveField
				v-for="field in rootFields"
				:key="field.path"
				:field="field"
				:schema="schema"
				:get-value-by-path="getValueByPath"
				:set-value-by-path="setValueByPath"
			/>
		</div>

		<div v-else-if="loading" class="empty-state">
			<i class="pi pi-spin pi-spinner"></i>
			<span>Loading settings…</span>
		</div>
	</div>
</template>
