<script setup lang="ts">
import '../../css/pages/profiles.css';

import InputText from 'primevue/inputtext';
import Button from "primevue/button";
import Dropdown, {DropdownChangeEvent} from "primevue/dropdown";
import Dialog from 'primevue/dialog';
import {computed, nextTick, onMounted, onUnmounted, reactive, ref} from "vue";
import {
	CreateProfile,
	DeleteProfile,
	EventTrigger,
	GetEngines,
	GetEnginesForProfile,
	GetModelVoices,
	GetProfiles,
	GetProfileSettings,
	GetProfileVoices,
	Play,
	SaveProfileSettings,
	SaveProfileVoices
} from '../../../bindings/nstudio/app';
import {CharacterVoice, Engine, ProfileSettings, Voice} from '../interfaces/engine';
import Checkbox from 'primevue/checkbox';
import Listbox from 'primevue/listbox';
import TreeSelect from "primevue/treeselect";
import type {TreeNode} from "primevue/treenode";
import {useLocalStorage} from '@vueuse/core';

interface Profile {
	id: string;
	name: string;
	description?: string;
	created_at?: string;
	updated_at?: string;
	voice_count?: number;
}

const engineModelNodes = ref<any[]>([]);
const engines = ref<{ [key: string]: Engine }>({});
const voiceOptions = ref<Record<string, Record<string, Voice>>>({});
const voiceOptionsMap = ref<Record<string, Voice[]>>({});
const characterVoices = ref<Record<string, CharacterVoice>>({});
const selectedProfile = useLocalStorage<string>('characterVoicesProfile', 'default');
const profileOptions = ref<Profile[]>([]);
const selectedProfileObj = ref<Profile | null>(null);
const showCreateDialog = ref(false);
const showDeleteDialog = ref(false);
const showSettingsDialog = ref(false);
const newProfileId = ref('');
const newProfileName = ref('');
const newProfileDescription = ref('');

const profileSettings = ref<ProfileSettings>({});
const settingsCacheEnabled = ref<boolean | null>(null);
const modelOptions = ref<{key: string, label: string, engine: string, model: string}[]>([]);
const selectedModelKeys = ref<{key: string, label: string, engine: string, model: string}[]>([]);

const selectedModels: Record<string, any> = reactive({});
const selectedVoices: Record<string, any> = reactive({});

const voiceIssues = computed<Record<string, string>>(() => {
	const issues: Record<string, string> = {};
	for (const [rowKey, character] of Object.entries(characterVoices.value)) {
		const selectionKeys = selectedModels[rowKey] ? Object.keys(selectedModels[rowKey]) : [];
		const modelKey = selectionKeys[0] || character.key;
		if (!modelKey) continue;

		const options = voiceOptions.value[modelKey];
		if (!options) {
			const [engineId, ...modelParts] = modelKey.split(':');
			const modelId = modelParts.join(':');
			const engineEntry = engines.value[engineId];
			const engineName = engineEntry?.name || engineId;
			const engineModels = engineEntry?.models ?? {};
			issues[rowKey] = Object.keys(engineModels).length > 0
				? `Model "${engineName}: ${modelId}" is not enabled`
				: `Engine "${engineName}" is not enabled`;
			continue;
		}

		const voiceID = selectedVoices[rowKey]?.voiceID || character.voice;
		if (voiceID && !options[voiceID]) {
			issues[rowKey] = `Voice "${voiceID}" is not available`;
		}
	}
	return issues;
});

async function getEngines() {
	const result = await GetEngines();
	const engineList: Engine[] = JSON.parse(result);

	for (const engine of engineList) {
		engines.value[engine.id] = engine;

		for (const index in engine.models) {
			if (engine.models.hasOwnProperty(index)) {
				const model = engine.models[index];
				await getModelVoices(engine.id, model.id);
			}
		}
	}

	return engines;
}

async function getModelVoices(engine: string, model: string) {
	const result = await GetModelVoices(engine, model);

	const voicesList: Voice[] = JSON.parse(result);
	const key = `${engine}:${model}`;
	const voicesMap: Record<string, Voice> = voicesList.reduce((acc, voice) => {
		acc[voice.voiceID] = voice;
		return acc;
	}, {} as Record<string, Voice>);
	if (voicesMap != null) {
		voiceOptions.value[key] = voicesMap;
	} else {
		voiceOptions.value[key] = {};
	}
}

async function getCharacterVoices() {
	const result = await GetProfileVoices(selectedProfile.value);

	const characterVoiceData = JSON.parse(result);

	characterVoices.value = characterVoiceData;
	savedFingerprint.value = voicesFingerprint(characterVoiceData);

	for (const name in characterVoiceData) {
		const data = characterVoiceData[name];
		const {key, voice} = data;

		selectedModels[name] = key ? {[key]: true} : {};

		const voiceOption = voiceOptions.value[key];
		if (voiceOption && voiceOption[voice]) {
			selectedVoices[name] = voiceOption[voice];
		} else {
			selectedVoices[name] = null;
		}

		const voiceOptionsRecord = voiceOptions.value[key] || {};
		voiceOptionsMap.value[name] = Object.values(voiceOptionsRecord);
	}

	addEmptyCharacterVoice();
}

async function loadProfiles() {
	try {
		const result = await GetProfiles();
		profileOptions.value = JSON.parse(result);

		const profile = profileOptions.value.find(p => p.id === selectedProfile.value)
			?? profileOptions.value.find(p => p.id === 'default')
			?? profileOptions.value[0];
		if (profile) {
			selectedProfileObj.value = profile;
			selectedProfile.value = profile.id;
		}
	} catch (error) {
		console.error('Failed to load profiles:', error);
	}
}

function openCreateDialog() {
	newProfileId.value = '';
	newProfileName.value = '';
	newProfileDescription.value = '';
	showCreateDialog.value = true;
}

async function createNewProfile() {
	if (!newProfileId.value.trim()) {
		return;
	}

	try {
		const result = await CreateProfile(
			newProfileId.value.trim(),
			newProfileName.value.trim() || newProfileId.value.trim(),
			newProfileDescription.value.trim()
		);

		if (result) {
			await loadProfiles();
			const newProfile = profileOptions.value.find(p => p.id === newProfileId.value);
			if (newProfile) {
				selectedProfileObj.value = newProfile;
				selectedProfile.value = newProfile.id;
				await onProfileChange();
			}
		}

		showCreateDialog.value = false;
	} catch (error) {
		console.error('Failed to create profile:', error);
	}
}

function openDeleteDialog() {
	if (selectedProfileObj.value && selectedProfileObj.value.id !== 'default') {
		showDeleteDialog.value = true;
	}
}

async function deleteCurrentProfile() {
	if (!selectedProfileObj.value || selectedProfileObj.value.id === 'default') {
		return;
	}

	try {
		await DeleteProfile(selectedProfileObj.value.id);
		selectedProfile.value = 'default';
		await loadProfiles();
		await onProfileChange();
		showDeleteDialog.value = false;
	} catch (error) {
		console.error('Failed to delete profile:', error);
	}
}

async function openSettingsDialog() {
	if (!selectedProfileObj.value) return;

	try {
		const settingsResult = await GetProfileSettings(selectedProfile.value);
		const currentSettings: ProfileSettings = settingsResult ? JSON.parse(settingsResult) : {};
		profileSettings.value = currentSettings;

		settingsCacheEnabled.value = currentSettings.cacheEnabled ?? null;

		const enginesResult = await GetEngines();
		const enginesList: Engine[] = JSON.parse(enginesResult);

		const options: {key: string, label: string, engine: string, model: string}[] = [];
		const selected: {key: string, label: string, engine: string, model: string}[] = [];

		for (const engine of enginesList) {
			for (const modelId in engine.models) {
				const key = `${engine.id}:${modelId}`;
				const modelName = engine.models[modelId].name || modelId;
				const option = {
					key,
					label: `${engine.name} - ${modelName}`,
					engine: engine.name,
					model: modelName
				};
				options.push(option);

				if (currentSettings.modelToggles?.[key]) {
					selected.push(option);
				}
			}
		}

		modelOptions.value = options;
		selectedModelKeys.value = selected;

		showSettingsDialog.value = true;
	} catch (error) {
		console.error('Failed to load profile settings:', error);
	}
}

async function saveSettingsDialog() {
	if (!selectedProfileObj.value) return;

	try {
		const toggles: Record<string, boolean> = {};
		for (const option of selectedModelKeys.value) {
			toggles[option.key] = true;
		}

		const settings: ProfileSettings = {
			modelToggles: toggles,
			cacheEnabled: settingsCacheEnabled.value ?? undefined
		};

		const settingsJSON = JSON.stringify(settings);
		await SaveProfileSettings(selectedProfile.value, settingsJSON);

		showSettingsDialog.value = false;

		await rebuildEngineModelNodes();
	} catch (error) {
		console.error('Failed to save profile settings:', error);
	}
}

function isModelOptionSelected(option: {key: string}): boolean {
	return selectedModelKeys.value.some(selected => selected.key === option.key);
}

function cancelSettingsDialog() {
	showSettingsDialog.value = false;
}

async function rebuildEngineModelNodes() {
	const settingsResult = await GetProfileSettings(selectedProfile.value);
	const currentSettings: ProfileSettings = settingsResult ? JSON.parse(settingsResult) : {};

	let enginesList: Engine[];

	if (currentSettings.modelToggles && Object.keys(currentSettings.modelToggles).length > 0) {
		const enginesResult = await GetEnginesForProfile(selectedProfile.value);
		enginesList = JSON.parse(enginesResult);
	} else {
		enginesList = Object.values(engines.value);
	}

	engineModelNodes.value = enginesList
		.filter(engine => Object.keys(engine.models ?? {}).length > 0)
		.map(engine => ({
			key: engine.id,
			label: engine.name,
			selectable: false,
			children: Object.entries(engine.models ?? {}).map(([modelId, modelData]) => ({
				selectable: true,
				key: `${engine.id}:${modelId}`,
				label: modelData.name,
			}))
		}));
}

async function onProfileDropdownChange() {
	if (selectedProfileObj.value) {
		selectedProfile.value = selectedProfileObj.value.id;
		await onProfileChange();
	}
}

async function onProfileChange() {
	savedFingerprint.value = null;
	characterVoices.value = {};
	Object.keys(selectedModels).forEach(key => delete selectedModels[key]);
	Object.keys(selectedVoices).forEach(key => delete selectedVoices[key]);
	voiceOptionsMap.value = {};

	await rebuildEngineModelNodes();

	await getCharacterVoices();
}

function onModelSelect(nodeKey: TreeNode, characterKey: string) {
	const [engineId, modelId] = nodeKey.key?.split(':') ?? '';
	const character = characterVoices.value[characterKey];
	character.key = nodeKey.key ?? '';
	character.engine = engineId;
	character.model = modelId;

	const rowVoiceOptions = Object.values(voiceOptions.value[`${engineId}:${modelId}`] ?? {});
	voiceOptionsMap.value[characterKey] = rowVoiceOptions;

	const voiceStillAvailable = (voiceID: string) => rowVoiceOptions.some(option => option.voiceID === voiceID);
	const currentVoice = selectedVoices[characterKey];
	if (currentVoice && !voiceStillAvailable(currentVoice.voiceID)) {
		selectedVoices[characterKey] = null;
	}
	if (character.voice && !voiceStillAvailable(character.voice)) {
		character.voice = '';
	}
}

async function onVoiceSelect(key: string, event: DropdownChangeEvent) {
	await previewVoice(key);
}

function buildProfileVoices(): Record<string, CharacterVoice> {
	return Object.entries(characterVoices.value)
		.filter(([key, voice]) => voice.name && voice.name.trim() !== '')
		.reduce((accumulator, [key, voice]) => {
			const selectionKeys = selectedModels[key] ? Object.keys(selectedModels[key]) : [];
			const modelKey = selectionKeys[0] || voice.key;
			if (!modelKey) {
				return accumulator;
			}

			const [engine, model] = modelKey.split(':');
			const voiceOption = selectedVoices[key];
			const voiceID = voiceOption ? voiceOption.voiceID : (modelKey === voice.key ? voice.voice : '');

			const newKey = key.startsWith('_') ? voice.name : key;

			accumulator[newKey] = {
				key: modelKey,
				name: voice.name,
				engine: engine,
				model: model,
				voice: voiceID || '',
			};

			return accumulator;
		}, {} as Record<string, CharacterVoice>);
}

function voicesFingerprint(voices: Record<string, CharacterVoice>): string {
	return JSON.stringify(Object.entries(voices)
		.filter(([, voice]) => voice.key && voice.name?.trim())
		.map(([key, voice]) => [key, voice.name, voice.key, voice.voice || ''])
		.sort());
}

const savedFingerprint = ref<string | null>(null);
const saving = ref(false);
const hasUnsavedChanges = computed(() =>
	savedFingerprint.value !== null && voicesFingerprint(buildProfileVoices()) !== savedFingerprint.value);

async function saveProfileVoices() {
	saving.value = true;
	try {
		await SaveProfileVoices(selectedProfile.value, JSON.stringify(buildProfileVoices()));
		savedFingerprint.value = voicesFingerprint(JSON.parse(await GetProfileVoices(selectedProfile.value)));
	} finally {
		saving.value = false;
	}
}

function isPlaceholderRow(key: string): boolean {
	const keys = Object.keys(characterVoices.value);
	return key === keys[keys.length - 1] && !characterVoices.value[key]?.name?.trim();
}

async function previewVoice(key: string) {
	const voice = characterVoices.value[key];
	if (!selectedVoices[key] || !selectedModels[key]) return;

	const modelVoiceID = "::" + Object.keys(selectedModels[key])[0] + ":" + selectedVoices[key].voiceID;

	await EventTrigger('notification.enabled', false);
	await Play(voice.name + ": " + voice.name, false, modelVoiceID, selectedProfile.value);
	await EventTrigger('notification.enabled', true);
}

function removeVoice(key: string) {
	if (isPlaceholderRow(key)) return;

	delete characterVoices.value[key];
}

function onNameInput(voice: any, key: string) {
	const keys = Object.keys(characterVoices.value);
	const lastKey = keys[keys.length - 1];
	if (key === lastKey) {
		if (voice.name && voice.name.trim() !== '') {
			addEmptyCharacterVoice();
		}
	}
}

function addEmptyCharacterVoice() {
	const key = "_" + Object.keys(characterVoices.value).length;
	characterVoices.value[key] = {
		key: "",
		name: "",
		engine: "",
		model: "",
		voice: ""
	};

	selectedModels[key] = {};

	selectedVoices[key] = null;

	voiceOptionsMap.value[key] = [];
}

onMounted(async () => {
	await loadProfiles();
	await getEngines();
	await rebuildEngineModelNodes();
	await getCharacterVoices();
	await nextTick();
});

onUnmounted(() => {
	EventTrigger('notification.enabled', true);
})

</script>
<template>
	<div class="page page--column">
		<div class="page-toolbar">
			<label class="field__label" for="profiles-profile">Profile</label>
			<Dropdown
				v-model="selectedProfileObj"
				inputId="profiles-profile"
				:options="profileOptions"
				optionLabel="name"
				placeholder="Select a profile"
				class="w-64"
				@change="onProfileDropdownChange"
			/>
			<Button
				icon="pi pi-plus"
				severity="secondary"
				title="New profile"
				aria-label="New profile"
				@click="openCreateDialog"
			/>
			<Button
				icon="pi pi-cog"
				severity="secondary"
				title="Profile settings"
				aria-label="Profile settings"
				:disabled="!selectedProfileObj"
				@click="openSettingsDialog"
			/>
			<Button
				icon="pi pi-trash"
				severity="secondary"
				title="Delete this profile"
				aria-label="Delete this profile"
				:disabled="!selectedProfileObj || selectedProfileObj.id === 'default'"
				@click="openDeleteDialog"
			/>
			<div class="page-toolbar__spacer"/>
			<span v-if="hasUnsavedChanges" class="unsaved-note">
				<i class="pi pi-circle-fill" style="font-size: 0.45rem"/> Unsaved changes
			</span>
			<Button
				icon="pi pi-save"
				label="Save"
				:loading="saving"
				:disabled="!hasUnsavedChanges || saving"
				@click="saveProfileVoices"
			/>
		</div>

		<div class="page-body">
			<div class="profiles__grid">
				<div class="profiles__grid__header">
					<span>Character</span>
					<span>Model</span>
					<span>Voice</span>
					<span/>
				</div>
				<div class="profiles__row"
					 :key="key"
					 v-for="(voice, key) in characterVoices"
				>
					<InputText v-model="voice.name"
							   type="text"
							   placeholder="Add a character…"
							   aria-label="Character name"
							   @input="onNameInput(voice, key)"
					/>
					<TreeSelect v-if="engineModelNodes && engineModelNodes.length > 0"
								class="w-full"
								:class="{'profiles__model--invalid': voiceIssues[key]}"
								v-tooltip.top="voiceIssues[key]"
								:options="engineModelNodes"
								v-model="selectedModels[key]"
								@node-select="node => onModelSelect(node, key)"
								placeholder="Select a model"
					/>
					<span v-else class="field__hint">No models enabled</span>
					<Dropdown class="w-full"
							  @change="(event) => onVoiceSelect(key, event)"
							  v-model="selectedVoices[key]"
							  :options="voiceOptionsMap[key]"
							  filter
							  optionLabel="name"
							  placeholder="Select a voice"
					/>
					<div class="profiles__row__actions">
						<Button icon="pi pi-volume-up"
								text
								severity="secondary"
								title="Preview this voice"
								aria-label="Preview this voice"
								:disabled="!selectedVoices[key]"
								@click="previewVoice(key)"
						/>
						<Button v-if="!isPlaceholderRow(key)"
								icon="pi pi-trash"
								text
								severity="danger"
								title="Remove this character"
								aria-label="Remove this character"
								@click="removeVoice(key)"
						/>
					</div>
				</div>
			</div>
		</div>

		<Dialog
			v-model:visible="showCreateDialog"
			header="New profile"
			:modal="true"
			:style="{ width: '420px' }"
		>
			<div class="profile-dialog">
				<div class="field">
					<label class="field__label" for="profile-id">Profile ID</label>
					<InputText
						id="profile-id"
						v-model="newProfileId"
						placeholder="my-game"
						class="w-full"
					/>
					<small class="field__hint">Unique identifier, no spaces or special characters. Used by the server API.</small>
				</div>
				<div class="field">
					<label class="field__label" for="profile-name">Display name</label>
					<InputText
						id="profile-name"
						v-model="newProfileName"
						placeholder="My Game"
						class="w-full"
					/>
					<small class="field__hint">Optional, defaults to the ID.</small>
				</div>
				<div class="field">
					<label class="field__label" for="profile-description">Description</label>
					<InputText
						id="profile-description"
						v-model="newProfileDescription"
						placeholder="Voices for My Game"
						class="w-full"
					/>
				</div>
			</div>
			<template #footer>
				<Button label="Cancel" text severity="secondary" @click="showCreateDialog = false"/>
				<Button
					label="Create"
					@click="createNewProfile"
					:disabled="!newProfileId.trim()"
				/>
			</template>
		</Dialog>

		<Dialog
			v-model:visible="showDeleteDialog"
			header="Delete profile"
			:modal="true"
			:style="{ width: '420px' }"
		>
			<p class="m-0">Delete the profile "{{ selectedProfileObj?.name }}" and all its character voices? This can't be undone.</p>
			<template #footer>
				<Button label="Cancel" text severity="secondary" @click="showDeleteDialog = false"/>
				<Button
					label="Delete"
					icon="pi pi-trash"
					severity="danger"
					@click="deleteCurrentProfile"
				/>
			</template>
		</Dialog>

		<Dialog
			v-model:visible="showSettingsDialog"
			header="Profile settings"
			:modal="true"
			:style="{ width: '520px' }"
		>
			<div class="profile-dialog">
				<div class="check-row">
					<Checkbox
						v-model="settingsCacheEnabled"
						:binary="true"
						inputId="cache-enabled"
						:indeterminate="settingsCacheEnabled === null"
					/>
					<label for="cache-enabled">Cache audio</label>
				</div>

				<div class="field">
					<label class="field__label">Models</label>
					<p class="field__hint m-0">If no models are selected, all available ones will be used</p>
					<Listbox
						v-model="selectedModelKeys"
						:options="modelOptions"
						optionLabel="label"
						filter
						multiple
						:filterPlaceholder="'Search models...'"
						listStyle="max-height:300px"
						class="w-full"
					>
						<template #option="{ option }">
							<span class="flex items-center gap-2">
								<i class="pi" :class="isModelOptionSelected(option) ? 'pi-check-square' : 'pi-stop'"></i>
								{{ option.label }}
							</span>
						</template>
					</Listbox>
				</div>
			</div>
			<template #footer>
				<Button label="Cancel" text severity="secondary" @click="cancelSettingsDialog"/>
				<Button
					label="Save"
					@click="saveSettingsDialog"
				/>
			</template>
		</Dialog>
	</div>
</template>

<style scoped>
.profile-dialog {
	display: flex;
	flex-direction: column;
	gap: 1.25rem;
	padding: 0.25rem 0;
}
</style>
