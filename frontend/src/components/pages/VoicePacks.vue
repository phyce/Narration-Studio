<script setup lang="ts">
import '../../css/pages/voice-packs.css';

import Button from 'primevue/button';
import InputSwitch from 'primevue/inputswitch';
import Checkbox from 'primevue/checkbox';
import Dialog from 'primevue/dialog';
import Dropdown from 'primevue/dropdown';
import Tag from 'primevue/tag';
import ProgressBar from 'primevue/progressbar';
import PiperDownloadDialog from '../common/PiperDownloadDialog.vue';
import EnginePackBuilds from '../common/EnginePackBuilds.vue';
import {computed, onBeforeUnmount, onMounted, reactive, ref} from 'vue';
import {Engine, Model} from '../interfaces/engine';
import type {AvailablePack, EnginePackListing, InstalledPack, PythonManifest} from '../interfaces/enginePack';
import {findUpdate, newestFirst} from '../../util/enginePacks';
import {
	EnginePackCancelInstall,
	EnginePackFetchAvailable,
	EnginePackImport,
	EnginePackInstall,
	EnginePackPauseInstall,
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

function getTagSeverity(tag: string): string {
	switch (tag) {
		case 'local': return 'success';
		case 'api': return 'info';
		case 'gpu': return 'warning';
		default: return 'secondary';
	}
}

const engines = ref<Engine[]>([]);
const models = ref<Record<string, Model>>({});
const modelToggles = reactive<Record<string, boolean>>({});
const selectedEngineId = ref<string>('');
const reloadModelsButtonDisabled = ref<boolean>(false);

const availablePacks = ref<AvailablePack[]>([]);
const installedPacks = ref<InstalledPack[]>([]);
const installProgress = reactive<Record<string, number>>({});
const installStage = reactive<Record<string, string>>({});
const importStatus = ref<string>('');
const importEngineID = ref<string>('');

const installStageLabels: Record<string, string> = {
	downloading: 'Downloading',
	extracting: 'Extracting',
	weights: 'Downloading model weights',
	verifying: 'Checking it works',
};

function installStatusText(stage: string | undefined, percent: number | undefined): string {
	const label = installStageLabels[stage ?? ''] ?? 'Installing';
	return stage === 'verifying' ? `${label}…` : `${label}… ${percent ?? 0}%`;
}

const trustPrompt = reactive({
	visible: false,
	packLabel: '',
	sourcePath: '',
	weightsNote: '',
	understood: false,
	confirm: () => {},
});

function confirmLocalInstall(packLabel: string, sourcePath: string, weightsNote: string, confirm: () => void) {
	Object.assign(trustPrompt, {visible: true, packLabel, sourcePath, weightsNote, understood: false, confirm});
}

function acceptTrustPrompt() {
	trustPrompt.visible = false;
	trustPrompt.confirm();
}

function weightsDownloadNote(manifest: PythonManifest): string {
	const prepare = manifest.runtime?.prepare;
	return prepare
		? `It also downloads ~${prepare.downloadSizeMB} MB of model weights from ${prepare.source} during install.`
		: '';
}
const installBusy = reactive<Record<string, boolean>>({});
const installedOnly = ref<boolean>(false);
const importBusy = ref<boolean>(false);

const showSettingsDialog = ref(false);
const settingsConfig = ref<configBase>({} as configBase);
const settingsSchema = ref<ConfigSchema | null>(null);
const settingsLoading = ref(false);
const settingsHasChanges = ref(false);
const settingsOriginalConfig = ref('');

const engineFilter = ref<'all' | 'local' | 'api'>('all');
const engineFilterOptions = [
	{label: 'All', value: 'all'},
	{label: 'Local', value: 'local'},
	{label: 'Online', value: 'api'},
];

interface SidebarEntry {
	id: string;
	name: string;
	type: 'local' | 'api';
	tags: string[];
	installed: boolean;
	modelCount: number;
	builds: AvailablePack[];
	installedPack?: InstalledPack;
	hasUpdate: boolean;
	paused: boolean;
	engine?: Engine;
}

const buildsByEngine = computed(() => {
	const groups: Record<string, AvailablePack[]> = {};
	for (const availablePack of availablePacks.value) {
		(groups[availablePack.manifest.id] ??= []).push(availablePack);
	}
	return groups;
});

function packEntryFields(engineID: string, installedPack: InstalledPack | undefined) {
	const builds = buildsByEngine.value[engineID] ?? [];
	return {
		builds,
		installedPack,
		hasUpdate: !!findUpdate(builds, installedPack),
		paused: builds.some(build => build.paused),
	};
}

const sidebarEntries = computed<SidebarEntry[]>(() => {
	const registeredIDs = new Set(engines.value.map(engine => engine.id));
	const installedPacksByID = new Map(installedPacks.value.map(installedPack => [installedPack.manifest.id, installedPack]));

	const registered: SidebarEntry[] = engines.value.map(registeredEngine => ({
		id: registeredEngine.id,
		name: registeredEngine.name,
		type: registeredEngine.type,
		tags: registeredEngine.tags ?? [],
		installed: true,
		modelCount: getEngineModelCount(registeredEngine.id),
		engine: registeredEngine,
		...packEntryFields(registeredEngine.id, installedPacksByID.get(registeredEngine.id)),
	}));
	const installedButNotLoaded: SidebarEntry[] = installedPacks.value
		.filter(installedPack => !registeredIDs.has(installedPack.manifest.id))
		.map(installedPack => ({
			id: installedPack.manifest.id,
			name: installedPack.manifest.name,
			type: installedPack.manifest.type,
			tags: installedPack.manifest.tags ?? [],
			installed: true,
			modelCount: 0,
			...packEntryFields(installedPack.manifest.id, installedPack),
		}));
	const onlyAvailable: SidebarEntry[] = Object.entries(buildsByEngine.value)
		.filter(([engineID]) => !registeredIDs.has(engineID) && !installedPacksByID.has(engineID))
		.map(([engineID, builds]) => {
			const newestManifest = newestFirst(builds)[0].manifest;
			return {
				id: engineID,
				name: newestManifest.name,
				type: newestManifest.type,
				tags: newestManifest.tags ?? [],
				installed: false,
				modelCount: (newestManifest.models ?? []).length,
				...packEntryFields(engineID, undefined),
			};
		});
	return [...registered, ...installedButNotLoaded, ...onlyAvailable];
});

const filteredEngines = computed<SidebarEntry[]>(() => {
	let list = sidebarEntries.value;
	if (installedOnly.value) {
		list = list.filter(entry => entry.installed);
	}
	if (engineFilter.value !== 'all') {
		list = list.filter(entry => entry.type === engineFilter.value);
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

const selectedShowcaseManifest = computed<PythonManifest | undefined>(() => {
	const entry = selectedEntry.value;
	if (!entry) return undefined;
	return entry.installedPack?.manifest ?? newestFirst(entry.builds)[0]?.manifest;
});

const selectedIsUninstallable = computed(() => !!selectedEntry.value?.installedPack);

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

const engineSettingsPath = computed(() => engineConfigPathMap[selectedEngineId.value]);

const engineSettingsFields = computed((): ConfigField[] => {
	if (!settingsSchema.value?.fields || !engineSettingsPath.value) return [];
	return settingsSchema.value.fields.filter(field => field.path === engineSettingsPath.value);
});

const selectedEnabledCount = computed(() =>
	selectedEngineModelList.value.filter(model => modelToggles[`${model.engine}:${model.id}`]).length
);

const selectedSubtitle = computed(() => {
	const entry = selectedEntry.value;
	if (!entry) return '';
	const kind = entry.type === 'api' ? 'Online service' : 'Runs on this computer';
	const parts = [kind];
	if (!entry.installed) {
		parts.push('not installed');
	} else if (selectedEngine.value) {
		const total = selectedEngineModelCount.value;
		parts.push(`${selectedEnabledCount.value} of ${total} model${total === 1 ? '' : 's'} enabled`);
	} else {
		parts.push('installed, not loaded');
	}
	const installedManifest = entry.installedPack?.manifest;
	if (installedManifest) {
		parts.push(`version ${installedManifest.version}${installedManifest.variant ? ` (${installedManifest.variant})` : ''}`);
	}
	return parts.join(' · ');
});

async function loadData() {
	const [enginesResult, modelsResult, settings, packsResult] = await Promise.all([
		GetEngines(),
		GetAvailableModels(),
		GetSettings(),
		EnginePackFetchAvailable().catch(() => '{}'),
	]);

	engines.value = JSON.parse(enginesResult);
	models.value = JSON.parse(modelsResult);

	try {
		const listing: Partial<EnginePackListing> = JSON.parse(packsResult) || {};
		availablePacks.value = listing.packs ?? [];
		installedPacks.value = listing.installed ?? [];
	} catch {
		availablePacks.value = [];
		installedPacks.value = [];
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
	if (pack.local && !pack.paused) {
		confirmLocalInstall(
			`${pack.manifest.name} ${pack.manifest.version} (${pack.manifest.variant})`,
			pack.bundleUrl,
			weightsDownloadNote(pack.manifest),
			() => runInstall(pack),
		);
		return;
	}
	await runInstall(pack);
}

async function runInstall(pack: AvailablePack) {
	const packID = pack.manifest.id;
	if (installBusy[packID]) return;
	installBusy[packID] = true;
	installProgress[packID] = 0;
	try {
		await EnginePackInstall(JSON.stringify(pack));
		await loadData();
	} finally {
		installBusy[packID] = false;
		delete installProgress[packID];
		delete installStage[packID];
	}
}

function pauseInstall(engineID: string) {
	EnginePackPauseInstall(engineID);
}

function cancelInstall(engineID: string) {
	EnginePackCancelInstall(engineID);
}

function cancelImport() {
	if (importEngineID.value) {
		EnginePackCancelInstall(importEngineID.value);
	}
}

async function importPack() {
	if (importBusy.value) return;
	const path = await SelectFile('');
	if (!path) return;
	if (!path.toLowerCase().endsWith('.nstudio')) {
		await EnginePackImport(path);
		return;
	}
	confirmLocalInstall('', path, '', () => runImport(path));
}

async function runImport(path: string) {
	importBusy.value = true;
	importStatus.value = '';
	importEngineID.value = '';
	try {
		await EnginePackImport(path);
		await loadData();
	} finally {
		importBusy.value = false;
		importStatus.value = '';
		importEngineID.value = '';
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

function voiceCountLabel(count: number): string {
	if (count === 0) return 'No built-in voices';
	return `${count} voice${count === 1 ? '' : 's'}`;
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
		const progress = event.data as { engineID: string; stage: string; percent: number; done: boolean };
		installProgress[progress.engineID] = progress.done ? 100 : progress.percent;
		if (progress.stage) {
			installStage[progress.engineID] = progress.stage;
		}
		if (importBusy.value && !installBusy[progress.engineID]) {
			importEngineID.value = progress.engineID;
			importStatus.value = progress.done ? '' : installStatusText(progress.stage, progress.percent);
		}
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
	<div class="page voice-packs">
		<aside class="page-sidebar voice-packs__sidebar">
			<div class="voice-packs__sidebar__row">
				<Button
					icon="pi pi-file-import"
					:label="importBusy ? (importStatus || 'Importing…') : 'Import pack'"
					title="Install an engine pack (.nstudio) from a file"
					severity="secondary"
					size="small"
					class="voice-packs__sidebar__import"
					:loading="importBusy"
					:disabled="importBusy"
					@click="importPack"
				/>
				<Button
					v-if="importBusy"
					icon="pi pi-times"
					title="Cancel the import"
					aria-label="Cancel the import"
					severity="danger"
					text
					size="small"
					:disabled="!importEngineID"
					@click="cancelImport"
				/>
				<Button
					v-else
					icon="pi pi-refresh"
					title="Reload engines and voice packs"
					aria-label="Reload engines and voice packs"
					severity="secondary"
					size="small"
					:loading="reloadModelsButtonDisabled"
					:disabled="reloadModelsButtonDisabled"
					@click="reloadEngines"
				/>
			</div>
			<div class="voice-packs__sidebar__row">
				<Dropdown
					v-model="engineFilter"
					:options="engineFilterOptions"
					optionLabel="label"
					optionValue="value"
					aria-label="Engine type"
					class="voice-packs__sidebar__filter"
				/>
				<div class="check-row voice-packs__installed-only">
					<InputSwitch v-model="installedOnly" inputId="vp-installed-only"/>
					<label for="vp-installed-only">Installed</label>
				</div>
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
						<i v-if="!entry.installed" class="pi pi-download voice-packs__engine-card__status" title="Not installed"></i>
						<span v-else class="voice-packs__engine-card__count" :title="`${entry.modelCount} model${entry.modelCount === 1 ? '' : 's'}`">
							{{ entry.modelCount }}
						</span>
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
						<Tag v-if="!entry.installed" value="available" rounded class="voice-packs__tag-pill"/>
						<Tag v-if="entry.hasUpdate" value="update" severity="warning" rounded class="voice-packs__tag-pill"/>
						<Tag
							v-if="entry.paused && !installBusy[entry.id]"
							value="paused"
							severity="secondary"
							rounded
							class="voice-packs__tag-pill"
						/>
						<Tag
							v-if="entry.installedPack && !entry.installedPack.compatible"
							value="needs another version"
							severity="danger"
							rounded
							class="voice-packs__tag-pill"
						/>
					</div>
					<ProgressBar
						v-if="installBusy[entry.id]"
						:value="installProgress[entry.id] ?? 0"
						:mode="installStage[entry.id] === 'verifying' ? 'indeterminate' : 'determinate'"
						class="voice-packs__engine-card__progress"
						:showValue="false"
					/>
				</div>
				<div v-if="filteredEngines.length === 0" class="voice-packs__sidebar__empty">
					No engines match these filters.
				</div>
			</div>
		</aside>

		<div class="voice-packs__content" v-if="selectedEntry">
			<div class="page-toolbar voice-packs__header">
				<div class="voice-packs__header__text">
					<h2 class="page-title">{{ selectedShowcaseManifest?.name ?? selectedEntry.name }}</h2>
					<span class="page-subtitle">{{ selectedSubtitle }}</span>
				</div>
				<div class="page-toolbar__spacer"/>
				<template v-if="selectedEngine">
					<Button
						v-for="action in selectedEngineMeta.actions"
						:key="action.label"
						:icon="action.icon"
						:label="action.label"
						@click="action.action"
					/>
					<Button
						v-if="engineSettingsPath"
						icon="pi pi-cog"
						label="Settings"
						severity="secondary"
						@click="openSettings"
					/>
				</template>
				<Button
					v-if="selectedIsUninstallable"
					icon="pi pi-trash"
					label="Uninstall"
					severity="danger"
					outlined
					:disabled="installBusy[selectedEntry.id]"
					title="Remove this engine pack from disk"
					@click="uninstallPack(selectedEntry.id)"
				/>
			</div>

			<div class="page-body voice-packs__body">
				<p v-if="selectedIsAvailable && selectedShowcaseManifest?.description" class="voice-packs__description">
					{{ selectedShowcaseManifest.description }}
				</p>

				<div v-if="selectedIsAvailable || selectedEntry.installedPack" class="card">
					<EnginePackBuilds
						:builds="selectedEntry.builds"
						:installed="selectedEntry.installedPack"
						:busy="!!installBusy[selectedEntry.id]"
						:stage="installStage[selectedEntry.id]"
						:percent="installProgress[selectedEntry.id]"
						@install="installPack"
						@pause="pauseInstall(selectedEntry.id)"
						@cancel="cancelInstall(selectedEntry.id)"
					/>
				</div>

				<section v-if="selectedEngine" class="voice-packs__models">
					<div class="voice-packs__section-header">
						<h3 class="voice-packs__section-title">Models</h3>
						<div class="page-toolbar__spacer"/>
						<Button
							:disabled="selectedEnabledCount === 0"
							label="Disable all"
							severity="secondary"
							text
							size="small"
							@click="disableAll"
						/>
					</div>
					<div v-if="selectedEngineModelList.length" class="voice-packs__list">
						<label
							v-for="model in selectedEngineModelList"
							:key="model.engine + ':' + model.id"
							class="voice-packs__list-row"
						>
							<span class="voice-packs__list-row__name">{{ model.name }}</span>
							<span class="voice-packs__list-row__key">{{ model.engine }}:{{ model.id }}</span>
							<InputSwitch
								v-model="modelToggles[model.engine + ':' + model.id]"
								@update:modelValue="handleToggle"
							/>
						</label>
					</div>
					<div v-else class="empty-state voice-packs__models-empty">
						<i class="pi pi-inbox"></i>
						<span>This engine has no models yet.</span>
						<Button
							v-for="action in selectedEngineMeta.actions"
							:key="action.label"
							:icon="action.icon"
							:label="action.label"
							size="small"
							@click="action.action"
						/>
					</div>
				</section>

				<section v-else-if="selectedIsAvailable" class="voice-packs__models">
					<div class="voice-packs__section-header">
						<h3 class="voice-packs__section-title">Included models</h3>
					</div>
					<div class="voice-packs__list">
						<div v-for="model in (selectedShowcaseManifest?.models ?? [])" :key="model.id" class="voice-packs__list-row">
							<span class="voice-packs__list-row__name">{{ model.name }}</span>
							<span class="voice-packs__list-row__key">{{ voiceCountLabel((model.voices ?? []).length) }}</span>
						</div>
					</div>
				</section>
			</div>
		</div>

		<div class="voice-packs__content" v-else>
			<div class="empty-state">
				<i class="pi pi-box"></i>
				<span v-if="installedOnly && availablePacks.length > 0">
					Showing installed engines only. Turn off "Installed" to browse engines you can download.
				</span>
				<span v-else>Select an engine on the left.</span>
			</div>
		</div>

		<PiperDownloadDialog
			v-model:visible="showPiperDownloadDialog"
			@models-changed="loadData"
		/>

		<Dialog
			v-model:visible="showSettingsDialog"
			:header="(selectedEngine?.name ?? 'Engine') + ' settings'"
			:modal="true"
			:style="{ width: '520px' }"
		>
			<div v-if="settingsLoading" class="voice-packs__settings-loading">
				<i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
				<p>Loading settings…</p>
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
					This engine has no settings.
				</p>
			</div>
			<template #footer>
				<Button label="Cancel" text severity="secondary" @click="showSettingsDialog = false"/>
				<Button
					label="Save"
					@click="saveSettings"
					:disabled="!settingsHasChanges"
				/>
			</template>
		</Dialog>

		<Dialog
			v-model:visible="trustPrompt.visible"
			header="Install an engine pack from a file?"
			:modal="true"
			:style="{ width: '520px' }"
		>
			<div class="voice-packs__trust">
				<p class="voice-packs__trust__warning">
					<i class="pi pi-exclamation-triangle"></i>
					<span>
						Engine packs run their own code on this computer, with the same access to your files as
						Narration Studio. Only install packs from sources you trust — a pack from an untrusted
						source could damage your system or steal your data.
					</span>
				</p>
				<div class="voice-packs__trust__details">
					<div v-if="trustPrompt.packLabel"><strong>Pack:</strong> {{ trustPrompt.packLabel }}</div>
					<div><strong>File:</strong> {{ trustPrompt.sourcePath }}</div>
					<div v-if="trustPrompt.weightsNote">{{ trustPrompt.weightsNote }}</div>
				</div>
				<div class="check-row">
					<Checkbox v-model="trustPrompt.understood" inputId="vp-trust-understood" :binary="true"/>
					<label for="vp-trust-understood">I understand the risks and trust the source of this pack</label>
				</div>
			</div>
			<template #footer>
				<Button label="Cancel" text severity="secondary" @click="trustPrompt.visible = false"/>
				<Button
					label="Install"
					icon="pi pi-download"
					severity="warning"
					@click="acceptTrustPrompt"
					:disabled="!trustPrompt.understood"
				/>
			</template>
		</Dialog>
	</div>
</template>
