<script setup lang="ts">
import {computed} from 'vue';
import TextField from './TextField.vue';
import PasswordField from './PasswordField.vue';
import NumberField from './NumberField.vue';
import Checkbox from './Checkbox.vue';
import Dropdown from './Dropdown.vue';
import PathField from './PathField.vue';
import FormSection from './FormSection.vue';
import type {ConfigField} from '../../interfaces/config';

interface Props {
	field: ConfigField;
	modelValue: any;
	disabled?: boolean;
}

interface Emits {
	(e: 'update:modelValue', value: any): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const value = computed({
	get: () => props.modelValue,
	set: (val: any) => emit('update:modelValue', val)
});

const fieldType = computed(() => {
	// Use metadata type if available
	if (props.field.metadata?.type) {
		return props.field.metadata.type;
	}

	// Fall back to inferring type from value
	const val = props.field.value;
	if (typeof val === 'boolean') return 'checkbox';
	if (typeof val === 'number') return 'number';
	if (typeof val === 'string') return 'text';
	if (typeof val === 'object' && val !== null) return 'object';

	return 'text';
});
</script>

<template>
	<!-- Object type renders as a section (no direct field) -->
	<FormSection
		v-if="fieldType === 'object'"
		:path="field.path"
		:metadata="field.metadata"
	>
		<slot></slot>
	</FormSection>

	<!-- Text input -->
	<TextField
		v-else-if="fieldType === 'text'"
		v-model="value"
		:path="field.path"
		:metadata="field.metadata"
	/>

	<!-- Password input -->
	<PasswordField
		v-else-if="fieldType === 'password'"
		v-model="value"
		:path="field.path"
		:metadata="field.metadata"
	/>

	<!-- Number input -->
	<NumberField
		v-else-if="fieldType === 'number'"
		v-model="value"
		:path="field.path"
		:metadata="field.metadata"
	/>

	<!-- Checkbox -->
	<Checkbox
		v-else-if="fieldType === 'checkbox'"
		v-model="value"
		:path="field.path"
		:metadata="field.metadata"
		:disabled="disabled"
	/>

	<!-- Dropdown -->
	<Dropdown
		v-else-if="fieldType === 'dropdown'"
		v-model="value"
		:path="field.path"
		:metadata="field.metadata"
	/>

	<!-- Path picker -->
	<PathField
		v-else-if="fieldType === 'path'"
		v-model="value"
		:path="field.path"
		:metadata="field.metadata"
	/>

	<!-- Fallback to text input -->
	<TextField
		v-else
		v-model="value"
		:path="field.path"
		:metadata="field.metadata"
	/>
</template>
