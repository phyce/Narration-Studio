<script setup lang="ts">
import {computed, ref} from 'vue';
import Panel from 'primevue/panel';
import Tooltip from 'primevue/tooltip';
import type {ConfigFieldMetadata} from '../../interfaces/config';

interface Props {
	metadata?: ConfigFieldMetadata;
	path: string;
	collapsed?: boolean;
	depth?: number;
}

const props = withDefaults(defineProps<Props>(), {
	collapsed: false,
	depth: 0
});

const vTooltip = Tooltip;

const isCollapsed = ref(props.collapsed);

const label = computed(() => {
	return props.metadata?.label || props.path.split('.').pop() || props.path;
});

const description = computed(() => props.metadata?.description);
</script>

<template>
	<div class="config-section">
		<Panel :header="label" :collapsed="isCollapsed" toggleable>
			<template #header>
				<div class="config-section__header">
					<span class="config-section__title">{{ label }}</span>
					<i v-if="description"
					   v-tooltip.top="description"
					   class="pi pi-question-circle config-section__help-icon"></i>
				</div>
			</template>
			<div class="config-section__content">
				<slot></slot>
			</div>
		</Panel>
	</div>
</template>

<style scoped>
.config-section {
	margin-bottom: 1rem;
}

.config-section__header {
	display: flex;
	align-items: center;
	gap: 0.5rem;
	text-align: left;
	width: 100%;
}

.config-section__title {
	font-size: 1.1rem;
	font-weight: 600;
	color: var(--text-color);
}

.config-section__help-icon {
	color: var(--text-color-secondary);
	font-size: 0.9rem;
	cursor: help;
}

.config-section__content {
	padding: 1rem 1rem 1rem 0 !important;
	margin-left: 0 !important;
}

/* Remove left padding from nested sections */
.config-section :deep(.p-panel) {
	margin-left: 0 !important;
	padding-left: 0 !important;
}

.config-section :deep(.p-panel-content) {
	padding-left: 0 !important;
	margin-left: 0 !important;
}

.config-section :deep(.p-panel-header) {
	padding-left: 0 !important;
	margin-left: 0 !important;
}
</style>
