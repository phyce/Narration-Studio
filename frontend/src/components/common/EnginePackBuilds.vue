<script setup lang="ts">
import Button from 'primevue/button';
import Dropdown from 'primevue/dropdown';
import ProgressBar from 'primevue/progressbar';
import {computed, ref, watch} from 'vue';
import type {AvailablePack, InstalledPack} from '../interfaces/enginePack';
import {compareVersions, findUpdate, isSameBuild, newestFirst} from '../../util/enginePacks';

interface Props {
	builds: AvailablePack[];
	installed?: InstalledPack;
	busy: boolean;
	stage?: string;
	percent?: number;
}

const props = defineProps<Props>();
const emit = defineEmits<{
	install: [build: AvailablePack];
	pause: [];
	cancel: [];
}>();

const stageLabels: Record<string, string> = {
	downloading: 'Downloading',
	extracting: 'Extracting',
	weights: 'Downloading model weights',
	verifying: 'Checking it works',
};

const sortedBuilds = computed(() => newestFirst(props.builds));
const update = computed(() => findUpdate(props.builds, props.installed));

const selectedVersion = ref('');
const selectedBuildKey = ref('');

function buildKey(build: AvailablePack): string {
	return `${build.manifest.version}|${build.manifest.variant}|${build.local ? 'local' : 'catalog'}`;
}

const versionOptions = computed(() => {
	const versions = [...new Set(sortedBuilds.value.map(build => build.manifest.version))];
	return versions.map(version => ({label: versionLabel(version), value: version}));
});

const variantOptions = computed(() => sortedBuilds.value
	.filter(build => build.manifest.version === selectedVersion.value)
	.map(build => ({
		label: (build.manifest.variant || 'default') + (build.local ? ' (local file)' : ''),
		value: buildKey(build),
		variant: build.manifest.variant,
	})));

const selectedBuild = computed(() => sortedBuilds.value.find(build => buildKey(build) === selectedBuildKey.value));

const selectedIsInstalled = computed(() =>
	!!props.installed && !!selectedBuild.value && isSameBuild(selectedBuild.value.manifest, props.installed.manifest));

const pausable = computed(() => props.stage === 'downloading' || props.stage === 'weights');

const installLabel = computed(() => {
	const build = selectedBuild.value;
	if (!build) return 'Install';
	if (build.paused) return 'Resume';
	if (!build.compatible) return 'Needs a different Narration Studio version';
	if (!props.installed) return 'Install';
	if (selectedIsInstalled.value) return 'Reinstall';
	if (build.manifest.version === props.installed.manifest.version) return `Switch to ${build.manifest.variant}`;
	if (build.manifest.variant === props.installed.manifest.variant
		&& compareVersions(build.manifest.version, props.installed.manifest.version) > 0) {
		return `Update to ${build.manifest.version}`;
	}
	return `Install ${build.manifest.version} instead`;
});

const progressText = computed(() => {
	const label = stageLabels[props.stage ?? ''] ?? 'Installing';
	return props.stage === 'verifying' ? `${label}…` : `${label}… ${props.percent ?? 0}%`;
});

function versionLabel(version: string): string {
	const tags: string[] = [];
	if (props.installed?.manifest.version === version) tags.push('installed');
	if (sortedBuilds.value[0]?.manifest.version === version) tags.push('latest');
	return tags.length ? `${version} (${tags.join(', ')})` : version;
}

function chooseDefaultBuild() {
	const preferred = update.value
		?? sortedBuilds.value.find(build => props.installed && isSameBuild(build.manifest, props.installed.manifest))
		?? sortedBuilds.value.find(build => build.paused)
		?? sortedBuilds.value.find(build => build.compatible)
		?? sortedBuilds.value[0];
	selectedVersion.value = preferred?.manifest.version ?? '';
	selectedBuildKey.value = preferred ? buildKey(preferred) : '';
}

watch(() => [props.builds, props.installed], chooseDefaultBuild, {immediate: true});

watch(selectedVersion, () => {
	if (!variantOptions.value.some(option => option.value === selectedBuildKey.value)) {
		const installedVariant = props.installed?.manifest.variant;
		const sameVariant = variantOptions.value.find(option => option.variant === installedVariant);
		selectedBuildKey.value = (sameVariant ?? variantOptions.value[0])?.value ?? '';
	}
});

function formatSize(bytes: number): string {
	if (!bytes) return '—';
	if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
	if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
	return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

function formatDate(isoDate: string | undefined): string {
	return isoDate ? new Date(isoDate).toLocaleDateString() : '';
}
</script>

<template>
	<div class="voice-packs__builds">
		<div v-if="installed" class="voice-packs__builds__installed">
			<span><strong>Installed:</strong> {{ installed.manifest.version }}<template v-if="installed.manifest.variant"> · {{ installed.manifest.variant }}</template></span>
			<span v-if="!installed.compatible" class="voice-packs__builds__warning">
				This version needs a different version of Narration Studio. Install another version below.
			</span>
			<span v-else-if="update && !busy" class="voice-packs__builds__update">
				Version {{ update.manifest.version }} is available.
			</span>
		</div>

		<div v-if="builds.length" class="voice-packs__builds__picker">
			<label class="voice-packs__builds__field">
				<span>Version</span>
				<Dropdown v-model="selectedVersion" :options="versionOptions" optionLabel="label" optionValue="value" :disabled="busy"/>
			</label>
			<label v-if="variantOptions.length > 1 || selectedBuild?.manifest.variant" class="voice-packs__builds__field">
				<span>Variant</span>
				<Dropdown v-model="selectedBuildKey" :options="variantOptions" optionLabel="label" optionValue="value" :disabled="busy"/>
			</label>
			<template v-if="busy">
				<Button v-if="pausable" icon="pi pi-pause" label="Pause" severity="secondary" @click="emit('pause')"/>
				<Button icon="pi pi-times" label="Cancel" severity="danger" text @click="emit('cancel')"/>
			</template>
			<Button
				v-else
				:icon="selectedBuild?.paused ? 'pi pi-play' : 'pi pi-download'"
				:label="installLabel"
				:disabled="!selectedBuild || !selectedBuild.compatible"
				@click="selectedBuild && emit('install', selectedBuild)"
			/>
		</div>
		<p v-else class="voice-packs__builds__none">
			{{ installed ? 'No other versions are available to download right now.' : 'No versions of this engine are available to download right now.' }}
		</p>

		<div v-if="busy" class="voice-packs__builds__progress">
			<span>{{ progressText }}</span>
			<ProgressBar :value="percent ?? 0" :mode="stage === 'verifying' ? 'indeterminate' : 'determinate'" :showValue="false" class="voice-packs__builds__bar"/>
		</div>

		<div v-if="selectedBuild" class="voice-packs__builds__meta">
			<div v-if="selectedBuild.manifest.variantDescription">
				<strong>{{ selectedBuild.manifest.variant }}:</strong> {{ selectedBuild.manifest.variantDescription }}
			</div>
			<div><strong>Download size:</strong> {{ formatSize(selectedBuild.bundleSize) }}</div>
			<div v-if="selectedBuild.manifest.approxInstallSizeMB">
				<strong>Disk space after install:</strong> ~{{ selectedBuild.manifest.approxInstallSizeMB }} MB
			</div>
			<div v-if="selectedBuild.manifest.runtime?.prepare">
				<strong>Model weights:</strong>
				~{{ selectedBuild.manifest.runtime.prepare.downloadSizeMB }} MB from
				{{ selectedBuild.manifest.runtime.prepare.source }}, downloaded during install
			</div>
			<div v-if="selectedBuild.publishedAt"><strong>Released:</strong> {{ formatDate(selectedBuild.publishedAt) }}</div>
			<div v-if="selectedBuild.local"><strong>From:</strong> local file</div>
		</div>
		<div v-if="selectedBuild?.notes" class="voice-packs__builds__notes">
			<strong>What's new in {{ selectedBuild.manifest.version }}</strong>
			<p>{{ selectedBuild.notes }}</p>
		</div>
	</div>
</template>
