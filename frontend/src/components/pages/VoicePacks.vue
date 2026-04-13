<script setup lang="ts">
import '../../css/pages/voice-packs.css';

import Button from 'primevue/button';
import InputSwitch from 'primevue/inputswitch';
import Dialog from 'primevue/dialog';
import Tag from 'primevue/tag';
import Badge from 'primevue/badge';
import DataView from 'primevue/dataview';
import SplitButton from 'primevue/splitbutton';
import {computed, onMounted, reactive, ref} from 'vue';
import {Engine, Model} from '../interfaces/engine';
import {
	GetAvailableModels,
	GetConfigSchema,
	GetEngines,
	GetSettings,
	RefreshModels,
	ReloadVoicePacks,
	SaveSettings,
} from '../../../wailsjs/go/main/App';
import type {ConfigField, ConfigSchema} from '../../interfaces/config';
import RecursiveField from '../config/RecursiveField.vue';
import {config as configuration} from '../../../wailsjs/go/models';
import configBase = configuration.Base;

// Engine metadata: tags and custom action buttons
interface EngineAction {
	label: string;
	icon: string;
	action: () => void;
}

interface EngineMeta {
	actions: EngineAction[];
}

const engineMeta: Record<string, EngineMeta> = {
	piper: {
		actions: [{label: 'Download Models', icon: 'pi pi-download', action: () => { /* TODO */ }}],
	},
};

// Map engine ID to config schema path prefix
const engineConfigPathMap: Record<string, string> = {
	piper: 'engine.local.piper',
	mssapi4: 'engine.local.msSapi4',
	mssapi5: 'engine.local.msSapi5',
	openai: 'engine.api.openAI',
	elevenlabs: 'engine.api.elevenLabs',
	google: 'engine.api.google',
	gemini: 'engine.api.gemini',
};

// Tag color mapping
function getTagSeverity(tag: string): string | undefined {
	switch (tag) {
		case 'local': return 'success';
		case 'api': return 'info';
		case 'gpu': return 'warning';
		case 'realtime': return undefined;
		default: return undefined;
	}
}

// State
const engines = ref<Engine[]>([]);
const models = ref<Record<string, Model>>({});
const modelToggles = reactive<Record<string, boolean>>({});
const selectedEngineId = ref<string>('');
const reloadModelsButtonDisabled = ref<boolean>(false);

// Settings modal state
const showSettingsDialog = ref(false);
const settingsConfig = ref<configBase>({} as configBase);
const settingsSchema = ref<ConfigSchema | null>(null);
const settingsLoading = ref(false);
const settingsHasChanges = ref(false);
const settingsOriginalConfig = ref('');

// Engine type filter
const engineFilter = ref('All');
const engineFilterLabel = computed(() => engineFilter.value === 'All' ? 'All' : engineFilter.value);
const engineFilterItems = [
	{label: 'All', command: () => { engineFilter.value = 'All'; }},
	{label: 'Local', command: () => { engineFilter.value = 'Local'; }},
	{label: 'API', command: () => { engineFilter.value = 'API'; }},
];

const filteredEngines = computed(() => {
	if (engineFilter.value === 'All') return engines.value;
	const type = engineFilter.value === 'Local' ? 'local' : 'api';
	return engines.value.filter(e => e.type === type);
});

// Computed
const selectedEngine = computed(() =>
	engines.value.find(e => e.id === selectedEngineId.value)
);

const selectedEngineModels = computed(() => {
	if (!selectedEngineId.value) return {};
	const result: Record<string, Model> = {};
	for (const [key, model] of Object.entries(models.value)) {
		if (model.engine === selectedEngineId.value) {
			result[key] = model;
		}
	}
	return result;
});

const selectedEngineModelCount = computed(() =>
	Object.keys(selectedEngineModels.value).length
);

const selectedEngineModelList = computed(() =>
	Object.values(selectedEngineModels.value)
);

function getEngineModelCount(engineId: string): number {
	return Object.values(models.value).filter(m => m.engine === engineId).length;
}

const selectedEngineMeta = computed(() =>
	engineMeta[selectedEngineId.value] ?? {actions: []}
);

const engineSettingsFields = computed((): ConfigField[] => {
	if (!settingsSchema.value?.fields || !selectedEngineId.value) return [];
	const prefix = engineConfigPathMap[selectedEngineId.value];
	if (!prefix) return [];
	return settingsSchema.value.fields.filter(f => f.path === prefix);
});

// Actions
async function loadData() {
	const [enginesResult, modelsResult, settings] = await Promise.all([
		GetEngines(),
		GetAvailableModels(),
		GetSettings(),
	]);

	engines.value = JSON.parse(enginesResult);
	models.value = JSON.parse(modelsResult);

	const savedToggles = settings.modelToggles;
	Object.entries(models.value).forEach(([, model]) => {
		const toggleKey = `${model.engine}:${model.id}`;
		modelToggles[toggleKey] = savedToggles[toggleKey] ?? false;
	});

	if (engines.value.length > 0 && !selectedEngineId.value) {
		selectedEngineId.value = engines.value[0].id;
	}
}

function selectEngine(engineId: string) {
	selectedEngineId.value = engineId;
}

async function handleToggle() {
	const payload = await GetSettings();
	payload.modelToggles = modelToggles;
	await SaveSettings(payload);
	await RefreshModels();
}

async function disableAll() {
	for (const model of Object.values(selectedEngineModels.value)) {
		const toggleKey = `${model.engine}:${model.id}`;
		modelToggles[toggleKey] = false;
	}
	await handleToggle();
}

async function reloadEngines() {
	reloadModelsButtonDisabled.value = true;
	await ReloadVoicePacks();

	const [modelsResult, settings] = await Promise.all([
		GetAvailableModels(),
		GetSettings(),
	]);

	models.value = JSON.parse(modelsResult);
	const savedToggles = settings.modelToggles;
	Object.entries(models.value).forEach(([, model]) => {
		const toggleKey = `${model.engine}:${model.id}`;
		modelToggles[toggleKey] = savedToggles[toggleKey] ?? false;
	});

	reloadModelsButtonDisabled.value = false;
}

// Settings modal
async function openSettings() {
	settingsLoading.value = true;
	showSettingsDialog.value = true;
	settingsHasChanges.value = false;

	const [config, schemaResult] = await Promise.all([
		GetSettings(),
		GetConfigSchema(),
	]);

	settingsConfig.value = config;
	settingsOriginalConfig.value = JSON.stringify(config);
	settingsSchema.value = JSON.parse(schemaResult);
	settingsLoading.value = false;
}

function getSettingsValueByPath(path: string): any {
	const parts = path.split('.');
	let value: any = settingsConfig.value;
	for (const part of parts) {
		if (value && typeof value === 'object' && part in value) {
			value = value[part];
		} else {
			return undefined;
		}
	}
	return value;
}

function setSettingsValueByPath(path: string, value: any) {
	const parts = path.split('.');
	let obj: any = settingsConfig.value;
	for (let i = 0; i < parts.length - 1; i++) {
		const part = parts[i];
		if (!(part in obj)) {
			obj[part] = {};
		}
		obj = obj[part];
	}
	obj[parts[parts.length - 1]] = value;
	settingsHasChanges.value = JSON.stringify(settingsConfig.value) !== settingsOriginalConfig.value;
}

async function saveSettings() {
	await SaveSettings(settingsConfig.value);
	settingsOriginalConfig.value = JSON.stringify(settingsConfig.value);
	settingsHasChanges.value = false;
	showSettingsDialog.value = false;
}

onMounted(loadData);
</script>

<template>
	<div class="voice-packs">
		<!-- Sidebar -->
		<div class="voice-packs__sidebar">
			<div class="voice-packs__sidebar__toolbar">
				<Button
					icon="pi pi-refresh"
					label="Reload"
					title="Reload engines & voice packs"
					@click="reloadEngines"
					:disabled="reloadModelsButtonDisabled"
					size="small"
					class="voice-packs__toolbar-btn"
				/>
				<SplitButton
					:label="engineFilterLabel"
					:model="engineFilterItems"
					@click="engineFilter = 'All'"
					size="small"
					class="voice-packs__toolbar-btn"
				/>
			</div>
			<div class="voice-packs__sidebar__list">
				<div
					v-for="engine in filteredEngines"
					:key="engine.id"
					class="voice-packs__engine-card"
					:class="{'voice-packs__engine-card--active': selectedEngineId === engine.id}"
					@click="selectEngine(engine.id)"
				>
					<div class="voice-packs__engine-card__header">
						<span class="voice-packs__engine-card__name">{{ engine.name }}</span>
						<Badge :value="getEngineModelCount(engine.id)" class="voice-packs__badge"/>
					</div>
					<div class="voice-packs__engine-card__tags">
						<Tag
							v-for="tag in (engine.tags ?? [])"
							:key="tag"
							:value="tag"
							:severity="getTagSeverity(tag)"
							rounded
							class="voice-packs__tag-pill"
						/>
					</div>
				</div>
			</div>
		</div>

		<!-- Content -->
		<div class="voice-packs__content" v-if="selectedEngine">
			<!-- Action buttons -->
			<div class="voice-packs__content__actions">
				<Button
					v-for="action in selectedEngineMeta.actions"
					:key="action.label"
					:icon="action.icon"
					:label="action.label"
					@click="action.action"
					:title="action.label"
				/>
				<Button
					icon="pi pi-ban"
					label="Disable All"
					@click="disableAll"
					title="Disable all models for this engine"
					:disabled="selectedEngineModelCount === 0"
				/>
				<Button
					icon="pi pi-cog"
					label="Settings"
					@click="openSettings"
					title="Engine settings"
				/>
			</div>

			<!-- Voice packs list -->
			<DataView :value="selectedEngineModelList" dataKey="id" layout="list" class="voice-packs__dataview">
				<template #list="{ items }">
					<div class="voice-packs__list">
						<div
							v-for="model in items"
							:key="model.engine + ':' + model.id"
							class="voice-packs__list-row"
						>
							<span class="voice-packs__list-row__name">{{ model.name }}</span>
							<span class="voice-packs__list-row__key">{{ model.engine }}:{{ model.id }}</span>
							<InputSwitch
								v-model="modelToggles[model.engine + ':' + model.id]"
								@update:modelValue="handleToggle"
							/>
						</div>
					</div>
				</template>
				<template #empty>
					<div class="voice-packs__content__empty">
						<p>No voice packs available for this engine.</p>
					</div>
				</template>
			</DataView>
		</div>

		<div class="voice-packs__content voice-packs__content--empty" v-else>
			<p>Select an engine from the sidebar.</p>
		</div>

		<!-- Settings Dialog -->
		<Dialog
			v-model:visible="showSettingsDialog"
			:header="(selectedEngine?.name ?? 'Engine') + ' Settings'"
			:modal="true"
			:style="{ width: '500px' }"
		>
			<div v-if="settingsLoading" class="voice-packs__settings-loading">
				<i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
				<p>Loading settings...</p>
			</div>
			<div v-else-if="settingsSchema" class="voice-packs__settings-content">
				<RecursiveField
					v-for="field in engineSettingsFields"
					:key="field.path"
					:field="field"
					:schema="settingsSchema"
					:get-value-by-path="getSettingsValueByPath"
					:set-value-by-path="setSettingsValueByPath"
				/>
				<p v-if="engineSettingsFields.length === 0" class="voice-packs__settings-none">
					No configurable settings for this engine.
				</p>
			</div>
			<template #footer>
				<Button label="Cancel" @click="showSettingsDialog = false" class="p-button-text"/>
				<Button
					label="Save"
					@click="saveSettings"
					:disabled="!settingsHasChanges"
				/>
			</template>
		</Dialog>
	</div>
</template>
