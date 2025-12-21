<script setup lang="ts">
import {computed} from 'vue';
import type {ConfigField, ConfigSchema} from '../../interfaces/config';
import FormSection from './FormSection.vue';
import FieldFactory from './FieldFactory.vue';

interface Props {
	field: ConfigField;
	schema: ConfigSchema;
	getValueByPath: (path: string) => any;
	setValueByPath: (path: string, value: any) => void;
}

const props = defineProps<Props>();

// Get direct children of this field
const children = computed(() => {
	if (!props.schema?.fields) return [];

	return props.schema.fields.filter(f => {
		const fieldParts = f.path.split('.');
		const parentParts = props.field.path.split('.');

		if (fieldParts.length !== parentParts.length + 1) {
			return false;
		}

		for (let i = 0; i < parentParts.length; i++) {
			if (fieldParts[i] !== parentParts[i]) {
				return false;
			}
		}

		return true;
	});
});

const isObject = computed(() => props.field.metadata?.type === 'object');

const depth = computed(() => props.field.path.split('.').length - 1);
</script>

<template>
	<!-- If it's an object type, render as a section with recursive children -->
	<FormSection
		v-if="isObject"
		:path="field.path"
		:metadata="field.metadata"
		:depth="depth"
	>
		<RecursiveField
			v-for="childField in children"
			:key="childField.path"
			:field="childField"
			:schema="schema"
			:get-value-by-path="getValueByPath"
			:set-value-by-path="setValueByPath"
		/>
	</FormSection>

	<!-- If it's a regular field, render the field factory -->
	<FieldFactory
		v-else
		:field="field"
		:model-value="getValueByPath(field.path)"
		@update:model-value="(val) => setValueByPath(field.path, val)"
	/>
</template>
