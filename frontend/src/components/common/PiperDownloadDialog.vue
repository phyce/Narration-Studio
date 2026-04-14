<script setup lang="ts">
import {computed, onBeforeUnmount, onMounted, reactive, ref, watch} from 'vue';
import Dialog from 'primevue/dialog';
import Button from 'primevue/button';
import DataView from 'primevue/dataview';
import ProgressSpinner from 'primevue/progressspinner';
import Tag from 'primevue/tag';
import {
	PiperDeleteModel,
	PiperDownloadModel,
	PiperFetchAvailableModels,
} from '../../../wailsjs/go/main/App';
import {EventsOn, EventsOff} from '../../../wailsjs/runtime/runtime';

interface AvailableModel {
	id: string;
	name: string;
	tagName: string;
	description: string;
	language: string;
	voices: number;
	modelUrl: string;
	configUrl: string;
	metadataUrl: string;
	size: number;
	installed: boolean;
}

interface Props {
	visible: boolean;
}

const props = defineProps<Props>();
const emit = defineEmits<{
	'update:visible': [value: boolean];
	'models-changed': [];
}>();

const models = ref<AvailableModel[]>([]);
const loading = ref(false);
const error = ref<string | null>(null);
const busyIds = ref<Set<string>>(new Set());
const progress = reactive<Record<string, number>>({});

const isBusy = (id: string) => busyIds.value.has(id);

const visibleModel = computed({
	get: () => props.visible,
	set: (v) => emit('update:visible', v),
});

function formatSize(bytes: number): string {
	if (!bytes) return '—';
	if (bytes < 1024) return `${bytes} B`;
	if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
	if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
	return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

async function loadModels() {
	loading.value = true;
	error.value = null;
	try {
		const result = await PiperFetchAvailableModels();
		models.value = JSON.parse(result);
	} catch (e: any) {
		error.value = e?.message ?? String(e);
		models.value = [];
	}
	loading.value = false;
}

async function onDownload(model: AvailableModel) {
	busyIds.value.add(model.id);
	progress[model.id] = 0;
	const err = await PiperDownloadModel(JSON.stringify(model));
	busyIds.value.delete(model.id);
	delete progress[model.id];
	if (!err) {
		await loadModels();
		emit('models-changed');
	}
}

async function onDelete(model: AvailableModel) {
	busyIds.value.add(model.id);
	const err = await PiperDeleteModel(model.id);
	busyIds.value.delete(model.id);
	if (!err) {
		await loadModels();
		emit('models-changed');
	}
}

watch(() => props.visible, (v) => {
	if (v) loadModels();
});

onMounted(() => {
	EventsOn('piper.download.progress', (p: { modelId: string; percent: number; done: boolean }) => {
		// On done, pin to 100 — onDownload's cleanup removes it once the Promise resolves.
		// Deleting here would briefly show 0% before the button label flips back to "Download".
		progress[p.modelId] = p.done ? 100 : p.percent;
	});
});

onBeforeUnmount(() => {
	EventsOff('piper.download.progress');
});
</script>

<template>
	<Dialog
		v-model:visible="visibleModel"
		header="Piper Models"
		:modal="true"
		:style="{ width: '640px' }"
	>
		<div class="piper-download">
			<div class="piper-download__header">
				<Button
					icon="pi pi-refresh"
					label="Refresh"
					size="small"
					@click="loadModels"
					:disabled="loading"
				/>
			</div>

			<div v-if="loading" class="piper-download__loading">
				<ProgressSpinner style="width: 40px; height: 40px"/>
				<span>Loading models...</span>
			</div>

			<div v-else-if="error" class="piper-download__error">
				<i class="pi pi-exclamation-triangle"/>
				<span>{{ error }}</span>
			</div>

			<DataView
				v-else
				:value="models"
				dataKey="id"
				layout="list"
				class="piper-download__dataview"
			>
				<template #list="{ items }">
					<div class="piper-download__list">
						<div
							v-for="m in items"
							:key="m.id"
							class="piper-download__row"
						>
							<div class="piper-download__row__info">
								<div class="piper-download__row__title">
									<span class="piper-download__row__name">{{ m.description || m.name }}</span>
									<Tag
										v-if="m.installed"
										value="installed"
										severity="success"
										rounded
										class="piper-download__tag"
									/>
								</div>
								<div class="piper-download__row__meta">
									<span v-if="m.language" class="piper-download__row__meta-item">
										<i class="pi pi-globe"/> {{ m.language }}
									</span>
									<span v-if="m.voices" class="piper-download__row__meta-item">
										<i class="pi pi-users"/> {{ m.voices }} voice{{ m.voices === 1 ? '' : 's' }}
									</span>
									<span class="piper-download__row__meta-item">
										<i class="pi pi-file"/> {{ formatSize(m.size) }}
									</span>
								</div>
							</div>
							<div class="piper-download__row__actions">
								<Button
									v-if="!m.installed"
									:icon="isBusy(m.id) ? 'pi pi-spin pi-spinner' : 'pi pi-download'"
									:label="isBusy(m.id) ? `${progress[m.id] ?? 0}%` : 'Download'"
									size="small"
									:disabled="isBusy(m.id)"
									@click="onDownload(m)"
								/>
								<Button
									v-else
									icon="pi pi-trash"
									label="Delete"
									size="small"
									:loading="isBusy(m.id)"
									class="piper-download__delete-btn"
									@click="onDelete(m)"
								/>
							</div>
						</div>
					</div>
				</template>
				<template #empty>
					<div class="piper-download__empty">
						<p>No models available.</p>
					</div>
				</template>
			</DataView>
		</div>

		<template #footer>
			<Button label="Close" class="p-button-text" @click="visibleModel = false"/>
		</template>
	</Dialog>
</template>

<style scoped>
.piper-download {
	display: flex;
	flex-direction: column;
	gap: 0.75rem;
	min-height: 300px;
}

.piper-download__header {
	display: flex;
	justify-content: flex-end;
}

.piper-download__loading,
.piper-download__error,
.piper-download__empty {
	display: flex;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	gap: 0.5rem;
	padding: 2rem;
	color: var(--text-color-secondary);
}

.piper-download__error {
	color: #ff7070;
}

.piper-download__dataview {
	background: transparent;
	border: none;
}

.piper-download__list {
	display: flex;
	flex-direction: column;
}

.piper-download__row {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 1rem;
	padding: 0.5rem 0.75rem;
	border-bottom: 1px solid #3a3a3a;
}

.piper-download__row:last-child {
	border-bottom: none;
}

.piper-download__row__info {
	display: flex;
	flex-direction: column;
	gap: 0.25rem;
	flex: 1;
	min-width: 0;
}

.piper-download__row__title {
	display: flex;
	align-items: center;
	gap: 0.5rem;
}

.piper-download__row__name {
	font-weight: 600;
	font-size: 0.875rem;
}

.piper-download__row__meta {
	display: flex;
	flex-wrap: wrap;
	gap: 0.75rem;
	font-size: 0.7rem;
	color: var(--text-color-secondary);
}

.piper-download__row__meta-item {
	display: inline-flex;
	align-items: center;
	gap: 0.25rem;
}

.piper-download__row__meta-item i {
	font-size: 0.65rem;
}

.piper-download__tag {
	font-size: 0.6rem !important;
	padding: 0.1rem 0.4rem !important;
}

.piper-download__row__actions {
	flex-shrink: 0;
}

.piper-download__delete-btn {
	background: #2a2a2a !important;
	border-color: #4a4a4a !important;
	color: #ffffff !important;
}

.piper-download__delete-btn:hover {
	background: #353535 !important;
	border-color: #5a5a5a !important;
}

.piper-download__delete-btn :deep(.p-button-icon) {
	color: #ef4444 !important;
}
</style>
