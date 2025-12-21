<script setup lang="ts">
import {computed} from 'vue';
import Checkbox from 'primevue/checkbox';
import Tooltip from 'primevue/tooltip';
import type {ConfigFieldMetadata} from '../../interfaces/config';

interface Props {
	modelValue: boolean;
	metadata?: ConfigFieldMetadata;
	path: string;
}

interface Emits {
	(e: 'update:modelValue', value: boolean): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const vTooltip = Tooltip;

const value = computed({
	get: () => props.modelValue,
	set: (val: boolean) => emit('update:modelValue', val)
});

const label = computed(() => props.metadata?.label || props.path);
const description = computed(() => props.metadata?.description);
</script>

<template>
	<div class="config-field">
		<div class="config-field__checkbox-container">
			<Checkbox
				:id="path"
				v-model="value"
				:binary="true"
				class="config-field__checkbox"
			/>
			<label :for="path" class="config-field__label">
				{{ label }}
				<i v-if="description"
				   v-tooltip.top="description"
				   class="pi pi-question-circle config-field__help-icon"></i>
			</label>
		</div>
	</div>
</template>

<style scoped>
.config-field {
	display: flex;
	flex-direction: column;
	gap: 0.5rem;
	margin-bottom: 1rem;
}

.config-field__checkbox-container {
	display: flex;
	align-items: center;
	gap: 0.5rem;
}

.config-field__label {
	display: flex;
	align-items: center;
	gap: 0.5rem;
	font-weight: 600;
	color: var(--text-color);
	cursor: pointer;
}

.config-field__checkbox {
	cursor: pointer;
}

.config-field__help-icon {
	color: var(--text-color-secondary);
	font-size: 0.85rem;
	cursor: help;
}
</style>
