<script setup lang="ts">
import '../../css/pages/voice-packs.css';

import Button from 'primevue/button';
import InputSwitch from 'primevue/inputswitch';
import Dialog from 'primevue/dialog';
import Tag from 'primevue/tag';
import Badge from 'primevue/badge';
import DataView from 'primevue/dataview';
import SplitButton from 'primevue/splitbutton';
import ProgressBar from 'primevue/progressbar';
import PiperDownloadDialog from '../common/PiperDownloadDialog.vue';
import {computed, onBeforeUnmount, onMounted, reactive, ref} from 'vue';
import {Engine, Model} from '../interfaces/engine';
import {
	EnginePackFetchAvailable,
	EnginePackImport,
	EnginePackInstall,
	EnginePackUninstall,
	GetAvailableModels,
	GetConfigSchema,
	GetEngines,
	GetSettings,
	RefreshModels,
	ReloadVoicePacks,
	SaveSettings,
	SelectFile,
} from '../../../bindings/nstudio/app';
import {Events} from '@wailsio/runtime';
import type {ConfigField, ConfigSchema} from '../../interfaces/config';
import RecursiveField from '../config/RecursiveField.vue';
import * as configuration from '../../../bindings/nstudio/app/config/models';
import configBase = configuration.Base;

interface EngineAction {
	label: string;
	icon: string;
	action: () => void;
}

interface EngineMeta {
	actions: EngineAction[];
}

const showPiperDownloadDialog = ref(false);

const engineMeta: Record<string, EngineMeta> = {
	piper: {
		actions: [{
			label: 'Download Models',
			icon: 'pi pi-download',
			action: () => { showPiperDownloadDialog.value = true; },
		}],
	},
};

const engineConfigPathMap: Record<string, string> = {
	piper: 'engine.local.piper',
	mssapi5: 'engine.local.msSapi5',
	openai: 'engine.api.openAI',
	elevenlabs: 'engine.api.elevenLabs',
	google: 'engine.api.google',
	gemini: 'engine.api.gemini',
};

function getTagSeverity(tag: string): string | undefined {
	switch (tag) {
		case 'local': return 'success';
		case 'api': return 'info';
		case 'gpu': return 'warning';
		case 'realtime': return undefined;
		default: return undefined;
	}
}

// Manifest shapes mirror Go-side python.Manifest / pack.AvailablePack.
interface ManifestVoice { id: string; name: string; gender: string; }
interface ManifestModel { id: string; name: string; voices: ManifestVoice[]; }
interface PythonManifest {
	schemaVersion: number;
	id: string;
	name: string;
	version: string;
	description?: string;
	type: 'local' | 'api';
	tags: string[] | null;
	os: string;
	arch: string;
	approxInstallSizeMB?: number;
	models: ManifestModel[];
}
interface AvailablePack {
	manifest: PythonManifest;
	bundleUrl: string;
	bundleSize: number;
	installed: boolean;
	tagName: string;
}

const engines = ref<Engine[]>([]);
const models = ref<Record<string, Model>>({});
const modelToggles = reactive<Record<string, boolean>>({});
const selectedEngineId = ref<string>('');
const reloadModelsButtonDisabled = ref<boolean>(false);

const availablePacks = ref<AvailablePack[]>([]);
const installProgress = reactive<Record<string, number>>({});
const installBusy = reactive<Record<string, boolean>>({});
const installedOnly = ref<boolean>(false);
const importBusy = ref<boolean>(false);

const showSettingsDialog = ref(false);
const settingsConfig = ref<configBase>({} as configBase);
const settingsSchema = ref<ConfigSchema | null>(null);
const settingsLoading = ref(false);
const settingsHasChanges = ref(false);
const settingsOriginalConfig = ref('');

const engineFilter = ref('All');
const engineFilterLabel = computed(() => engineFilter.value === 'All' ? 'All' : engineFilter.value);
const engineFilterItems = [
	{label: 'All', command: () => { engineFilter.value = 'All'; }},
	{label: 'Local', command: () => { engineFilter.value = 'Local'; }},
	{label: 'API', command: () => { engineFilter.value = 'API'; }},
];

interface SidebarEntry {
	id: string;
	name: string;
	type: 'local' | 'api';
	tags: string[];
	installed: boolean;
	modelCount: number;
	pack?: AvailablePack;
	engine?: Engine;
}

const sidebarEntries = computed<SidebarEntry[]>(() => {
	const installedIDs = new Set(engines.value.map(engine => engine.id));
	const installed: SidebarEntry[] = engines.value.map(installedEngine => ({
		id: installedEngine.id,
		name: installedEngine.name,
		type: installedEngine.type,
		tags: installedEngine.tags ?? [],
		installed: true,
		modelCount: getEngineModelCount(installedEngine.id),
		engine: installedEngine,
		pack: availablePacks.value.find(availablePack => availablePack.manifest.id === installedEngine.id),
	}));
	const onlyAvailable: SidebarEntry[] = availablePacks.value
		.filter(availablePack => !installedIDs.has(availablePack.manifest.id))
		.map(availablePack => ({
			id: availablePack.manifest.id,
			name: availablePack.manifest.name,
			type: availablePack.manifest.type,
			tags: availablePack.manifest.tags ?? [],
			installed: false,
			modelCount: (availablePack.manifest.models ?? []).length,
			pack: availablePack,
		}));
	return [...installed, ...onlyAvailable];
});

const filteredEngines = computed<SidebarEntry[]>(() => {
	let list = sidebarEntries.value;
	if (installedOnly.value) {
		list = list.filter(entry => entry.installed);
	}
	if (engineFilter.value !== 'All') {
		const type = engineFilter.value === 'Local' ? 'local' : 'api';
		list = list.filter(entry => entry.type === type);
	}
	return list;
});

const selectedEngine = computed(() =>
	engines.value.find(engine => engine.id === selectedEngineId.value)
);

const selectedEntry = computed<SidebarEntry | undefined>(() =>
	sidebarEntries.value.find(entry => entry.id === selectedEngineId.value)
);

const selectedIsAvailable = computed(() =>
	!!selectedEntry.value && !selectedEntry.value.installed
);

const selectedPack = computed<AvailablePack | undefined>(() => selectedEntry.value?.pack);

const selectedIsUninstallable = computed(() =>
	!!selectedEntry.value?.installed && !!selectedEntry.value?.pack
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
	return Object.values(models.value).filter(model => model.engine === engineId).length;
}

const selectedEngineMeta = computed(() =>
	engineMeta[selectedEngineId.value] ?? {actions: []}
);

const engineSettingsFields = computed((): ConfigField[] => {
	if (!settingsSchema.value?.fields || !selectedEngineId.value) return [];
	const prefix = engineConfigPathMap[selectedEngineId.value];
	if (!prefix) return [];
	return settingsSchema.value.fields.filter(field => field.path === prefix);
});

async function loadData() {
	const [enginesResult, modelsResult, settings, packsResult] = await Promise.all([
		GetEngines(),
		GetAvailableModels(),
		GetSettings(),
		EnginePackFetchAvailable().catch(() => '[]'),
	]);

	engines.value = JSON.parse(enginesResult);
	models.value = JSON.parse(modelsResult);

	try {
		availablePacks.value = JSON.parse(packsResult) || [];
	} catch {
		availablePacks.value = [];
	}

	const savedToggles = settings.modelToggles;
	Object.entries(models.value).forEach(([, model]) => {
		const toggleKey = `${model.engine}:${model.id}`;
		modelToggles[toggleKey] = savedToggles[toggleKey] ?? false;
	});

	if (engines.value.length > 0 && !selectedEngineId.value) {
		selectedEngineId.value = engines.value[0].id;
	}
}

async function installPack(pack: AvailablePack) {
	const packID = pack.manifest.id;
	if (installBusy[packID]) return;
	installBusy[packID] = true;
	installProgress[packID] = 0;
	try {
		const err = await EnginePackInstall(JSON.stringify(pack));
		if (!err) {
			await loadData();
		}
	} finally {
		installBusy[packID] = false;
		delete installProgress[packID];
	}
}

async function importPack() {
	if (importBusy.value) return;
	const path = await SelectFile('');
	if (!path || !path.toLowerCase().endsWith('.nstudio')) {
		if (path && !path.toLowerCase().endsWith('.nstudio')) {
			await EnginePackImport(path);
		}
		return;
	}
	importBusy.value = true;
	try {
		const err = await EnginePackImport(path);
		if (!err) {
			await loadData();
		}
	} finally {
		importBusy.value = false;
	}
}

async function uninstallPack(engineID: string) {
	if (installBusy[engineID]) return;
	installBusy[engineID] = true;
	try {
		const err = await EnginePackUninstall(engineID);
		if (!err) {
			if (selectedEngineId.value === engineID) {
				selectedEngineId.value = '';
			}
			await loadData();
			if (!selectedEngineId.value && engines.value.length > 0) {
				selectedEngineId.value = engines.value[0].id;
			}
		}
	} finally {
		installBusy[engineID] = false;
	}
}

function formatSize(bytes: number): string {
	if (!bytes) return '—';
	if (bytes < 1024) return `${bytes} B`;
	if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
	if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
	return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
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
	let cursor: any = settingsConfig.value;
	for (let index = 0; index < parts.length - 1; index++) {
		const part = parts[index];
		if (!(part in cursor)) {
			cursor[part] = {};
		}
		cursor = cursor[part];
	}
	cursor[parts[parts.length - 1]] = value;
	settingsHasChanges.value = JSON.stringify(settingsConfig.value) !== settingsOriginalConfig.value;
}

async function saveSettings() {
	await SaveSettings(settingsConfig.value);
	settingsOriginalConfig.value = JSON.stringify(settingsConfig.value);
	settingsHasChanges.value = false;
	showSettingsDialog.value = false;
	await loadData();
}

onMounted(() => {
	loadData();

	Events.On('engine.download.progress', (event: any) => {
		const progress = event.data as { engineID: string; percent: number; done: boolean };
		installProgress[progress.engineID] = progress.done ? 100 : progress.percent;
	});

	Events.On('engines.updated', () => {
		loadData();
	});
});

onBeforeUnmount(() => {
	Events.Off('engine.download.progress');
	Events.Off('engines.updated');
});
</script>

<template>
	<div class="voice-packs">
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
				<Button
					icon="pi pi-folder-open"
					label="Import"
					title="Import an engine pack (.nstudio) from disk"
					@click="importPack"
					:loading="importBusy"
					:disabled="importBusy"
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
			<div class="voice-packs__sidebar__toolbar voice-packs__sidebar__toolbar--row">
				<InputSwitch v-model="installedOnly" inputId="vp-installed-only"/>
				<label for="vp-installed-only" class="voice-packs__filter-label">Show only installed</label>
			</div>
			<div class="voice-packs__sidebar__list">
				<div
					v-for="entry in filteredEngines"
					:key="entry.id"
					class="voice-packs__engine-card"
					:class="{
						'voice-packs__engine-card--active': selectedEngineId === entry.id,
						'voice-packs__engine-card--available': !entry.installed,
					}"
					@click="selectEngine(entry.id)"
				>
					<div class="voice-packs__engine-card__header">
						<span class="voice-packs__engine-card__name">{{ entry.name }}</span>
						<span v-if="!entry.installed" class="voice-packs__engine-card__status" title="Not installed">
							<i class="pi pi-download"></i>
						</span>
						<Badge v-else :value="entry.modelCount" class="voice-packs__badge"/>
					</div>
					<div class="voice-packs__engine-card__tags">
						<Tag
							v-for="tag in (entry.tags ?? [])"
							:key="tag"
							:value="tag"
							:severity="getTagSeverity(tag)"
							rounded
							class="voice-packs__tag-pill"
						/>
						<Tag
							v-if="!entry.installed"
							value="Available"
							severity="info"
							rounded
							class="voice-packs__tag-pill"
						/>
					</div>
					<ProgressBar
						v-if="installBusy[entry.id]"
						:value="installProgress[entry.id] ?? 0"
						class="voice-packs__engine-card__progress"
						:showValue="false"
					/>
				</div>
			</div>
		</div>

		<div class="voice-packs__content" v-if="selectedIsAvailable && selectedPack">
			<div class="voice-packs__content__actions">
				<Button
					:icon="installBusy[selectedPack.manifest.id] ? 'pi pi-spin pi-spinner' : 'pi pi-download'"
					:label="installBusy[selectedPack.manifest.id]
						? `Installing… ${installProgress[selectedPack.manifest.id] ?? 0}%`
						: 'Install'"
					@click="installPack(selectedPack)"
					:disabled="installBusy[selectedPack.manifest.id]"
				/>
			</div>
			<div class="voice-packs__pack-detail">
				<h3 class="voice-packs__pack-detail__title">{{ selectedPack.manifest.name }}</h3>
				<p v-if="selectedPack.manifest.description" class="voice-packs__pack-detail__description">
					{{ selectedPack.manifest.description }}
				</p>
				<div class="voice-packs__pack-detail__meta">
					<div><strong>Version:</strong> {{ selectedPack.manifest.version || selectedPack.tagName }}</div>
					<div><strong>Platform:</strong> {{ selectedPack.manifest.os }} / {{ selectedPack.manifest.arch }}</div>
					<div><strong>Download size:</strong> {{ formatSize(selectedPack.bundleSize) }}</div>
					<div v-if="selectedPack.manifest.approxInstallSizeMB">
						<strong>Disk space after install:</strong> ~{{ selectedPack.manifest.approxInstallSizeMB }} MB
					</div>
				</div>
				<div class="voice-packs__pack-detail__models">
					<h4>Voice packs in this engine</h4>
					<div v-for="model in (selectedPack.manifest.models ?? [])" :key="model.id" class="voice-packs__pack-detail__model">
						<div class="voice-packs__pack-detail__model-name">
							{{ model.name }}
							<Badge :value="(model.voices ?? []).length" class="voice-packs__badge"/>
						</div>
					</div>
				</div>
				<ProgressBar
					v-if="installBusy[selectedPack.manifest.id]"
					:value="installProgress[selectedPack.manifest.id] ?? 0"
				/>
			</div>
		</div>

		<div class="voice-packs__content" v-else-if="selectedEngine">
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
				<Button
					v-if="selectedIsUninstallable"
					icon="pi pi-trash"
					label="Uninstall"
					severity="danger"
					@click="uninstallPack(selectedEngine.id)"
					:disabled="installBusy[selectedEngine.id]"
					title="Remove this engine pack from disk"
				/>
			</div>

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
			<p v-if="installedOnly && availablePacks.length > 0">
				Showing installed engines only. Toggle "Show only installed" off to browse downloadable engines.
			</p>
			<p v-else>Select an engine from the sidebar.</p>
		</div>

		<PiperDownloadDialog
			v-model:visible="showPiperDownloadDialog"
			@models-changed="loadData"
		/>

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
