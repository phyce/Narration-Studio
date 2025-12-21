<script setup lang="ts">
import {onMounted, ref, watch, withDefaults} from 'vue';
import Dropdown from 'primevue/dropdown';
import Button from 'primevue/button';
import Dialog from 'primevue/dialog';
import InputText from 'primevue/inputtext';
import {CreateProfile, DeleteProfile, GetProfiles} from '../../../wailsjs/go/main/App';

interface Profile {
	id: string;
	name: string;
	description?: string;
	created_at?: string;
	updated_at?: string;
	voice_count?: number;
}

interface Props {
	modelValue?: string;
	showButtons?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
	showButtons: false
});
const emit = defineEmits(['update:modelValue', 'change']);

const profiles = ref<Profile[]>([]);
const selectedProfile = ref<Profile | null>(null);
const showCreateDialog = ref(false);
const showDeleteDialog = ref(false);
const newProfileId = ref('');
const newProfileName = ref('');
const newProfileDescription = ref('');

async function loadProfiles() {
	try {
		const result = await GetProfiles();
		profiles.value = JSON.parse(result);

		// If modelValue is provided, select that profile
		if (props.modelValue) {
			const profile = profiles.value.find(p => p.id === props.modelValue);
			if (profile) {
				selectedProfile.value = profile;
			}
		} else if (profiles.value.length > 0) {
			// Otherwise select default or first profile
			const defaultProfile = profiles.value.find(p => p.id === 'default');
			selectedProfile.value = defaultProfile || profiles.value[0];
			emit('update:modelValue', selectedProfile.value.id);
			emit('change', selectedProfile.value.id);
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
			const newProfile = profiles.value.find(p => p.id === newProfileId.value);
			if (newProfile) {
				selectedProfile.value = newProfile;
				emit('update:modelValue', newProfile.id);
				emit('change', newProfile.id);
			}
		}

		showCreateDialog.value = false;
	} catch (error) {
		console.error('Failed to create profile:', error);
	}
}

function openDeleteDialog() {
	if (selectedProfile.value && selectedProfile.value.id !== 'default') {
		showDeleteDialog.value = true;
	}
}

async function deleteCurrentProfile() {
	if (!selectedProfile.value || selectedProfile.value.id === 'default') {
		return;
	}

	try {
		await DeleteProfile(selectedProfile.value.id);
		await loadProfiles();
		showDeleteDialog.value = false;
	} catch (error) {
		console.error('Failed to delete profile:', error);
	}
}

function onProfileChange() {
	if (selectedProfile.value) {
		emit('update:modelValue', selectedProfile.value.id);
		emit('change', selectedProfile.value.id);
	}
}

// Watch for external changes to modelValue
watch(() => props.modelValue, (newVal) => {
	if (newVal && selectedProfile.value?.id !== newVal) {
		const profile = profiles.value.find(p => p.id === newVal);
		if (profile) {
			selectedProfile.value = profile;
		}
	}
});

onMounted(async () => {
	await loadProfiles();
});
</script>

<template>
	<div>
		<Dropdown
			v-model="selectedProfile"
			:options="profiles"
			optionLabel="name"
			placeholder="Select Profile"
			class="w-full"
			@change="onProfileChange"
		/>
		<div v-if="showButtons" class="flex gap-2 mt-2">
			<Button
				icon="pi pi-plus"
				title="Create New Profile"
				class="flex-1"
				@click="openCreateDialog"
			/>
			<Button
				icon="pi pi-trash"
				title="Delete Profile"
				class="flex-1"
				:disabled="!selectedProfile || selectedProfile.id === 'default'"
				@click="openDeleteDialog"
			/>
		</div>

		<!-- Create Profile Dialog -->
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
						placeholder="minecraft"
						class="profile-dialog__input"
					/>
					<small>Unique identifier (no spaces or special characters)</small>
				</div>
				<div class="profile-dialog__field">
					<label for="profile-name">Display Name</label>
					<InputText
						id="profile-name"
						v-model="newProfileName"
						placeholder="Minecraft"
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

		<!-- Delete Confirmation Dialog -->
		<Dialog
			v-model:visible="showDeleteDialog"
			header="Delete Profile"
			:modal="true"
			:style="{ width: '400px' }"
		>
			<p>Are you sure you want to delete the profile "{{ selectedProfile?.name }}"?</p>
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
	</div>
</template>

<style scoped>
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
</style>
