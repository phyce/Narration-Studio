<script setup lang="ts">
import {config as configuration} from "../../../wailsjs/go/models";
import configBase = configuration.Base;
import {onMounted, reactive, ref} from "vue";
import {GetSettings} from "../../../wailsjs/go/main/App";

const config = reactive<configBase>({} as configBase);
const loading = ref<boolean>(true);

onMounted(async () => {
	Object.assign(config, await GetSettings());
	loading.value = false;
});
</script>

<template>
	<div class="start" v-if="!loading">
		<h1 class="start__header">{{ config.info.name }} v{{config.info.version}}</h1>
		<img class="start__logo" id="logo" alt="Wails logo" src="../../assets/images/logo.png"/>
		<a target="_blank" class="link" :href="config.info.website">{{config.info.website}}</a>
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