<script setup lang="ts">
import {computed} from 'vue';
import InputGroup from 'primevue/inputgroup';
import InputText from 'primevue/inputtext';
import Button from 'primevue/button';
import Tooltip from 'primevue/tooltip';
import {SelectDirectory, SelectFile} from '../../../wailsjs/go/main/App';
import type {ConfigFieldMetadata} from '../../interfaces/config';

interface Props {
	modelValue: string;
	metadata?: ConfigFieldMetadata;
	path: string;
}

interface Emits {
	(e: 'update:modelValue', value: string): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const vTooltip = Tooltip;

const value = computed({
	get: () => props.modelValue,
	set: (val: string) => emit('update:modelValue', val)
});

const label = computed(() => props.metadata?.label || props.path);
const description = computed(() => props.metadata?.description);
const pathType = computed(() => props.metadata?.pathType || 'directory');

async function handleBrowse() {
	let selected: string;

	if (pathType.value === 'file') {
		selected = await SelectFile(value.value || '');
	} else {
		selected = await SelectDirectory(value.value || '');
	}

	if (selected && selected !== value.value) {
		value.value = selected;
	}
}
</script>

<template>
	<div class="config-field">
		<label :for="path" class="config-field__label">
			{{ label }}
			<span v-if="metadata?.required" class="config-field__required">*</span>
			<i v-if="description"
			   v-tooltip.top="description"
			   class="pi pi-question-circle config-field__help-icon"></i>
		</label>
		<InputGroup>
			<InputText
				:id="path"
				v-model="value"
				:title="value"
				class="config-field__input"
			/>
			<Button
				icon="pi pi-folder-open"
				@click="handleBrowse"
				title="Browse"
				aria-label="Browse"
			/>
		</InputGroup>
	</div>
</template>

<style scoped>
.config-field {
	display: flex;
	flex-direction: column;
	gap: 0.5rem;
	text-align: left;
	margin-bottom: 1rem;
}

.config-field__label {
	display: flex;
	align-items: center;
	gap: 0.5rem;
	font-weight: 600;
	color: var(--text-color);
}

.config-field__required {
	color: var(--red-500);
	margin-left: 0.25rem;
}

.config-field__help-icon {
	color: var(--text-color-secondary);
	font-size: 0.85rem;
	cursor: help;
}

.config-field__input {
	flex: 1;
}
</style>
