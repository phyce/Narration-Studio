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
		<h1 class="start__header">{{ config.info.name }} v{{ config.info.version }}</h1>
		<img class="start__logo" id="logo" alt="Wails logo" src="../../assets/images/logo.png"/>
		<a target="_blank" class="link" :href="config.info.website">{{ config.info.website }}</a>
	</div>
</template>

<style scoped>
.start {
	@apply w-full h-full pt-2;
}

.start__header {
	@apply text-3xl mb-3;
}

.start__logo {
	@apply object-contain w-full h-full;
}
</style>