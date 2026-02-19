<script setup lang="ts">
import {computed} from 'vue';
import type {ConfigField, ConfigSchema, FieldCondition} from '../../interfaces/config';
import FormSection from './FormSection.vue';
import FieldFactory from './FieldFactory.vue';

interface Props {
	field: ConfigField;
	schema: ConfigSchema;
	getValueByPath: (path: string) => any;
	setValueByPath: (path: string, value: any) => void;
	disabled?: boolean;
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

function evalConditions(conditions: FieldCondition | FieldCondition[] | undefined): boolean {
	if (!conditions) return false;
	const arr = Array.isArray(conditions) ? conditions : [conditions];
	return arr.some(c => props.getValueByPath(c.field) === c.value);
}

function isFieldHidden(f: ConfigField): boolean {
	return evalConditions(f.metadata?.hideWhen);
}

function isFieldDisabled(f: ConfigField): boolean {
	return evalConditions(f.metadata?.disableWhen);
}
</script>

<template>
	<!-- If it's an object type, render as a section with recursive children -->
	<FormSection
		v-if="isObject"
		:path="field.path"
		:metadata="field.metadata"
		:depth="depth"
	>
		<template v-for="childField in children" :key="childField.path">
			<RecursiveField
				v-if="!childField.metadata?.hidden && !isFieldHidden(childField)"
				:field="childField"
				:schema="schema"
				:get-value-by-path="getValueByPath"
				:set-value-by-path="setValueByPath"
				:disabled="isFieldDisabled(childField)"
			/>
		</template>
	</FormSection>

	<!-- If it's a regular field, render the field factory -->
	<FieldFactory
		v-else
		:field="field"
		:model-value="getValueByPath(field.path)"
		:disabled="disabled"
		@update:model-value="(val) => setValueByPath(field.path, val)"
	/>
</template>
