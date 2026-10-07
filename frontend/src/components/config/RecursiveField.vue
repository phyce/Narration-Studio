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
	// Nesting level below the first field the form renders (Settings starts at the root, the
	// Voice Packs settings dialog at one engine), so only the outermost boxes start expanded.
	level?: number;
}

const props = withDefaults(defineProps<Props>(), {
	level: 0
});

function childrenOf(path: string): ConfigField[] {
	if (!props.schema?.fields) return [];
	const prefix = path + '.';
	return props.schema.fields.filter(f =>
		f.path.startsWith(prefix) && !f.path.slice(prefix.length).includes('.'));
}

// A section whose fields are all hidden (e.g. a server pool entry with no settings) isn't rendered.
function hasVisibleContent(f: ConfigField): boolean {
	if (f.metadata?.hidden || isFieldHidden(f)) return false;
	if (f.metadata?.type !== 'object') return true;
	return childrenOf(f.path).some(hasVisibleContent);
}

const visibleChildren = computed(() => childrenOf(props.field.path).filter(hasVisibleContent));

const isObject = computed(() => props.field.metadata?.type === 'object');


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
		v-show="visibleChildren.length > 0"
		:path="field.path"
		:metadata="field.metadata"
		:depth="level"
	>
		<RecursiveField
			v-for="childField in visibleChildren"
			:key="childField.path"
			:field="childField"
			:schema="schema"
			:get-value-by-path="getValueByPath"
			:set-value-by-path="setValueByPath"
			:disabled="isFieldDisabled(childField)"
			:level="level + 1"
		/>
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
