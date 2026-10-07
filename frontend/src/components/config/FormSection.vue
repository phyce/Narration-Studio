<script setup lang="ts">
import {computed, ref} from 'vue';
import Tooltip from 'primevue/tooltip';
import type {ConfigFieldMetadata} from '../../interfaces/config';

interface Props {
	metadata?: ConfigFieldMetadata;
	path: string;
	collapsed?: boolean;
	depth?: number;
}

const props = withDefaults(defineProps<Props>(), {
	collapsed: undefined,
	depth: 0
});

const vTooltip = Tooltip;

// Only the outermost boxes start expanded.
const isCollapsed = ref(props.collapsed ?? props.depth > 0);

const label = computed(() => {
	return props.metadata?.label || props.path.split('.').pop() || props.path;
});

const description = computed(() => props.metadata?.description);
</script>

<template>
	<section class="config-section" :class="{'config-section--nested': depth > 0}">
		<button type="button"
				class="config-section__header"
				:aria-expanded="!isCollapsed"
				@click="isCollapsed = !isCollapsed"
		>
			<i class="pi config-section__chevron" :class="isCollapsed ? 'pi-chevron-right' : 'pi-chevron-down'"></i>
			<span class="config-section__title">{{ label }}</span>
			<i v-if="description"
			   v-tooltip.top="description"
			   class="pi pi-question-circle config-section__help-icon"
			   @click.stop></i>
		</button>
		<div v-show="!isCollapsed" class="config-section__content">
			<slot></slot>
		</div>
	</section>
</template>

<style scoped>
.config-section {
	margin-bottom: 0.5rem;
	border: 1px solid var(--surface-border);
	border-radius: 6px;
	background-color: var(--app-card);
}

.config-section--nested {
	background-color: rgba(255, 255, 255, 0.025);
}

.config-section__header {
	display: flex;
	align-items: center;
	gap: 0.5rem;
	width: 100%;
	padding: 0.5rem 0.75rem;
	border-radius: 6px;
	text-align: left;
	cursor: pointer;
}

.config-section__header:hover {
	background-color: rgba(255, 255, 255, 0.03);
}

.config-section__chevron {
	width: 0.75rem;
	font-size: 0.7rem;
	color: var(--text-color-secondary);
}

.config-section__title {
	font-size: 1rem;
	font-weight: 700;
	color: var(--text-color);
}

.config-section--nested > .config-section__header .config-section__title {
	font-size: 0.9rem;
}

.config-section__help-icon {
	color: var(--text-color-secondary);
	font-size: 0.85rem;
	cursor: help;
}

.config-section__content {
	padding: 0.25rem 0.75rem 0.5rem;
}
</style>
