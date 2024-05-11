<script setup lang="ts">

import {computed, defineProps, ref} from "vue";
import {Status, StatusDisplayNames} from "./enums/status";

	const props = defineProps({
		status: Number
	});

	const currentView = ref(props.status);
	const currentStatusText = computed(() => StatusDisplayNames[currentView.value as Status]);
</script>

<template>
	<footer class="bg-neutral-800 py-1 text-xs text-gray-200">
		{{ currentStatusText }}
	</footer>
</template>

<style scoped>

</style>