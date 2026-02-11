<script setup lang="ts">
import '../../css/pages/profiles.css';

import InputText from 'primevue/inputtext';
import Button from "primevue/button";
import Dropdown, {DropdownChangeEvent} from "primevue/dropdown";
import Dialog from 'primevue/dialog';
import {nextTick, onMounted, onUnmounted, reactive, ref} from "vue";
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
} from '../../../wailsjs/go/main/App';
import {CharacterVoice, Engine, ProfileSettings, Voice} from '../interfaces/engine';
import Checkbox from 'primevue/checkbox';
import Listbox from 'primevue/listbox';
import TreeSelect from "primevue/treeselect";
import {TreeNode} from "primevue/treenode";
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

	for (const name in characterVoiceData) {
		const data = characterVoiceData[name];
		const {key, voice} = data;

		selectedModels[name] = {[key]: true};

		const voiceOption = voiceOptions.value[key];
		if (voiceOption && voiceOption[voice]) {
			selectedVoices[name] = voiceOption[voice];
		} else {
			data.engine = "";
			data.model = "";
			data.voice = "";
			selectedModels[name] = {};
			selectedVoices[name] = null;
		}

		const engineModelKey = data.engine + ":" + data.model;
		const voiceOptionsRecord = voiceOptions.value[engineModelKey] || {};
		voiceOptionsMap.value[name] = Object.values(voiceOptionsRecord);
	}

	addEmptyCharacterVoice();
}

async function loadProfiles() {
	try {
		const result = await GetProfiles();
		profileOptions.value = JSON.parse(result);

		if (selectedProfile.value) {
			const profile = profileOptions.value.find(p => p.id === selectedProfile.value);
			if (profile) {
				selectedProfileObj.value = profile;
			}
		} else if (profileOptions.value.length > 0) {
			const defaultProfile = profileOptions.value.find(p => p.id === 'default');
			selectedProfileObj.value = defaultProfile || profileOptions.value[0];
			selectedProfile.value = selectedProfileObj.value.id;
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
		await loadProfiles();
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

	engineModelNodes.value = enginesList.map(engine => ({
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
	character.engine = engineId;
	character.model = modelId;

	voiceOptionsMap.value[characterKey] = Object.values(voiceOptions.value[`${engineId}:${modelId}`]) || [];
}

async function onVoiceSelect(key: string, event: DropdownChangeEvent) {
	await previewVoice(key);
}

const saveProfileVoices = () => {
	const dataToSave = Object.entries(characterVoices.value)
		.filter(([key, voice]) => {
			const hasName = voice.name && voice.name.trim() !== '';
			const hasSelection = selectedModels[key] && Object.keys(selectedModels[key]).length > 0;

			return hasName && hasSelection;
		})
		.reduce((accumulator, [key, voice]) => {
			const modelKeys = Object.keys(selectedModels[key]);
			const modelKey = modelKeys[0];
			const [engine, model] = modelKey.split(':');
			const voiceOption = selectedVoices[key];
			const voiceID = voiceOption ? voiceOption.voiceID : '';

			const newKey = key.startsWith('_') ? voice.name : key;

			accumulator[newKey] = {
				key: modelKey,
				name: voice.name,
				engine: engine,
				model: model,
				voice: voiceID,
			};

			return accumulator;
		}, {} as Record<string, CharacterVoice>);

	const dataString = JSON.stringify(dataToSave);

	SaveProfileVoices(selectedProfile.value, dataString);
};

async function previewVoice(key: string) {
	const voice = characterVoices.value[key];

	const modelVoiceID = "::" + Object.keys(selectedModels[key])[0] + ":" + selectedVoices[key].voiceID;

	await EventTrigger('notification.enabled', false);
	await Play(voice.name + ": " + voice.name, false, modelVoiceID, selectedProfile.value);
	await EventTrigger('notification.enabled', true);
}

async function removeVoice(key: string) {
	if (key == "_" + (Object.keys(characterVoices.value).length - 1)) return;

	if (key in characterVoices.value) delete characterVoices.value[key];
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
	<div class="voices">
		<div class="voices__actions">
			<div class="flex items-center gap-2 mt-2 mr-2">
				<label class="font-semibold whitespace-nowrap">Profile</label>
				<Dropdown
					v-model="selectedProfileObj"
					:options="profileOptions"
					optionLabel="name"
					placeholder="Select Profile"
					class="w-64 text-left"
					@change="onProfileDropdownChange"
				/>
				<div class="flex border border-gray-600 rounded overflow-hidden">
					<Button
						icon="pi pi-plus"
						title="Create New Profile"
						class="voices__action-button"
						@click="openCreateDialog"
					/>
					<Button
						icon="pi pi-cog"
						title="Profile Settings"
						class="voices__action-button"
						:disabled="!selectedProfileObj"
						@click="openSettingsDialog"
					/>
					<Button
						icon="pi pi-trash"
						title="Delete Profile"
						class="voices__action-button"
						:disabled="!selectedProfileObj || selectedProfileObj.id === 'default'"
						@click="openDeleteDialog"
					/>
				</div>
			</div>
			<Button class="voices__actions__save"
					title="Save All"
					aria-label="Save All"
					@click="saveProfileVoices()"
			>
				<i class="pi pi-save"/>&nbsp;
				Save All
			</Button>
		</div>
		<div class="voices__entry"
			 :key="key"
			 v-for="(voice, key) in characterVoices"
		>
			<div class="voices__entry__name" :data-key="key">
				<InputText class="voices__entry__name__input"
						   v-model="voice.name"
						   type="text"
						   placeholder="Character name"
						   @input="onNameInput(voice, key)"
				/>
			</div>
			<div class="voices__entry__model">
				<TreeSelect class="voices__entry__model__tree"
							v-if="engineModelNodes && engineModelNodes.length > 0"
							:options="engineModelNodes"
							v-model="selectedModels[key]"
							@node-select="node => onModelSelect(node, key)"
							placeholder="Select a model"
				/>
			</div>
			<div class="voices__entry__voice">
				<Dropdown class="voices__entry__voice__dropdown"
						  @change="(event) => onVoiceSelect(key, event)"
						  v-model="selectedVoices[key]"
						  :options="voiceOptionsMap[key]"
						  filter
						  optionLabel="name"
						  placeholder="Select a voice"
				/>
			</div>
			<div class="voices__entry__actions">
				<div class="voices__entry__actions__container">
					<Button class="button-start"
							@click="previewVoice(key)"
							icon="pi pi-volume-up"
							title="Preview"
							aria-label="Preview"
					/>
					<Button class="button-stop"
							@click="removeVoice(key)"
							icon="pi pi-trash"
							title="Remove"
							aria-label="Remove"
					/>
				</div>
			</div>
		</div>

		<Dialog
			v-model:visible="showCreateDialog"
			header="Create New Profile"
			:modal="true"
			:style="{ width: '400px' }"
		>
			<div class="profile-dialog">
				<div class="profile-dialog__field">
					<label for="profile-id">Profile ID*</label>
					<InputText
						id="profile-id"
						v-model="newProfileId"
						placeholder="appname"
						class="profile-dialog__input"
					/>
					<small>Unique identifier (no spaces or special characters)</small>
				</div>
				<div class="profile-dialog__field">
					<label for="profile-name">Display Name</label>
					<InputText
						id="profile-name"
						v-model="newProfileName"
						placeholder="App Display Name"
						class="profile-dialog__input"
					/>
					<small>Optional display name (defaults to ID)</small>
				</div>
				<div class="profile-dialog__field">
					<label for="profile-description">Description</label>
					<InputText
						id="profile-description"
						v-model="newProfileDescription"
						placeholder="Voice profile for My Game"
						class="profile-dialog__input"
					/>
				</div>
			</div>
			<template #footer>
				<Button label="Cancel" @click="showCreateDialog = false" class="p-button-text"/>
				<Button
					label="Create"
					@click="createNewProfile"
					:disabled="!newProfileId.trim()"
				/>
			</template>
		</Dialog>

		<Dialog
			v-model:visible="showDeleteDialog"
			header="Delete Profile"
			:modal="true"
			:style="{ width: '400px' }"
		>
			<p>Are you sure you want to delete the profile "{{ selectedProfileObj?.name }}"?</p>
			<p>This action cannot be undone.</p>
			<template #footer>
				<Button label="Cancel" @click="showDeleteDialog = false" class="p-button-text"/>
				<Button
					label="Delete"
					@click="deleteCurrentProfile"
					class="p-button-danger"
				/>
			</template>
		</Dialog>

		<Dialog
			v-model:visible="showSettingsDialog"
			header="Profile Settings"
			:modal="true"
			:style="{ width: '500px' }"
		>
			<div class="settings-dialog">
				<div class="settings-dialog__section">
					<div class="settings-dialog__field">
						<div class="flex items-center gap-2">
							<Checkbox
								v-model="settingsCacheEnabled"
								:binary="true"
								inputId="cache-enabled"
								:indeterminate="settingsCacheEnabled === null"
							/>
							<label for="cache-enabled">Enable Cache for this Profile</label>
						</div>
					</div>
				</div>

				<div class="settings-dialog__section">
					<p class="settings-dialog__description">
						Select which models are available for this profile. When no models are selected, all globally enabled models will be available.
					</p>
					<Listbox
						v-model="selectedModelKeys"
						:options="modelOptions"
						optionLabel="label"
						filter
						multiple
						checkbox
						:filterPlaceholder="'Search models...'"
						listStyle="max-height:300px"
						class="w-full"
					/>
				</div>
			</div>
			<template #footer>
				<Button label="Cancel" @click="cancelSettingsDialog" class="p-button-text"/>
				<Button
					label="Save"
					@click="saveSettingsDialog"
				/>
			</template>
		</Dialog>
	</div>
</template>

<style scoped>
.voices__action-button {
	border: 0 !important;
	border-radius: 0 !important;
}

.profile-dialog {
	display: flex;
	flex-direction: column;
	gap: 1.5rem;
	padding: 1rem 0;
}

.profile-dialog__field {
	display: flex;
	flex-direction: column;
	gap: 0.5rem;
}

.profile-dialog__field label {
	font-weight: 600;
}

.profile-dialog__input {
	width: 100%;
}

.profile-dialog__field small {
	color: var(--text-color-secondary);
	font-size: 0.75rem;
}

.settings-dialog {
	display: flex;
	flex-direction: column;
	gap: 1.5rem;
	padding: 0.5rem 0;
}

.settings-dialog__section {
	display: flex;
	flex-direction: column;
	gap: 0.75rem;
}

.settings-dialog__section-title {
	font-weight: 600;
	font-size: 1rem;
	margin: 0;
	padding-bottom: 0.5rem;
	border-bottom: 1px solid var(--surface-border);
}

.settings-dialog__description {
	color: var(--text-color-secondary);
	font-size: 0.875rem;
	margin: 0;
}

.settings-dialog__field {
	display: flex;
	flex-direction: column;
	gap: 0.5rem;
}

.text-left :deep(.p-dropdown-label) {
	text-align: left;
}
</style>