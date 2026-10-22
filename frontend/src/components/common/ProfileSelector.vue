<script setup lang="ts">
import {onMounted, ref, watch} from 'vue';
import Dropdown from 'primevue/dropdown';
import {GetProfiles} from '../../../bindings/nstudio/app';

interface Profile {
	id: string;
	name: string;
	description?: string;
}

const props = defineProps<{
	modelValue?: string;
}>();
const emit = defineEmits(['update:modelValue', 'change']);

const profiles = ref<Profile[]>([]);
const selectedProfile = ref<Profile | null>(null);

function select(profile: Profile) {
	selectedProfile.value = profile;
	if (profile.id !== props.modelValue) {
		emit('update:modelValue', profile.id);
		emit('change', profile.id);
	}
}

async function loadProfiles() {
	try {
		profiles.value = JSON.parse(await GetProfiles());
	} catch (error) {
		console.error('Failed to load profiles:', error);
		return;
	}

	const remembered = profiles.value.find(profile => profile.id === props.modelValue);
	const fallback = profiles.value.find(profile => profile.id === 'default') ?? profiles.value[0];
	const profile = remembered ?? fallback;
	if (profile) {
		select(profile);
	}
}

function onProfileChange() {
	if (selectedProfile.value) {
		select(selectedProfile.value);
	}
}

watch(() => props.modelValue, (newValue) => {
	if (newValue && selectedProfile.value?.id !== newValue) {
		const profile = profiles.value.find(candidate => candidate.id === newValue);
		if (profile) {
			selectedProfile.value = profile;
		}
	}
});

onMounted(loadProfiles);
</script>

<template>
	<Dropdown
		v-model="selectedProfile"
		:options="profiles"
		optionLabel="name"
		placeholder="Select a profile"
		class="w-full"
		@change="onProfileChange"
	/>
</template>
