<script setup lang="ts"></script>

<template>
	<div class="start">
		<h1 class="start__header">Narrator Studio v0.9.0</h1>
		<img class="start__logo" id="logo" alt="Wails logo" src="../../assets/images/logo.png"/>
		Made by <a class="link" href="https://phyce.dev" target="_blank">Phyce</a>
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