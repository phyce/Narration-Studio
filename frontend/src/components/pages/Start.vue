<script setup lang="ts">
import * as configuration from "../../../bindings/nstudio/app/config/models";
import {onMounted, reactive, ref} from "vue";
import {GetSettings} from "../../../bindings/nstudio/app";
import configBase = configuration.Base;

const config = reactive<configBase>({} as configBase);
const loading = ref<boolean>(true);

onMounted(async () => {
	Object.assign(config, await GetSettings());
	loading.value = false;
});
</script>

<template>
	<div class="start" v-if="!loading">
		<img class="start__logo" alt="Narration Studio logo" src="../../assets/images/logo.png"/>
		<h1 class="start__header">{{ config.info.name }}</h1>
		<p class="start__version">Version {{ config.info.version }}</p>
		<a target="_blank" class="link" :href="config.info.website">{{ config.info.website }}</a>
	</div>
</template>

<style scoped>
.start {
	@apply flex flex-col items-center justify-center gap-2 w-full h-full p-6 text-center;
}

.start__logo {
	@apply object-contain mb-4;
	width: min(40vh, 280px);
	height: min(40vh, 280px);
}

.start__header {
	@apply text-3xl font-bold m-0;
}

.start__version {
	@apply text-sm m-0;
	color: var(--text-color-secondary);
}
</style>
