<script setup lang="ts">

import {Status, StatusDisplayNames} from "./enums/status";
import {onMounted, onUnmounted, ref, computed} from "vue";
import {eventManager} from "../util/eventManager";
import {GetStatus} from "../../wailsjs/go/main/App";


const status = ref<number>(Status.Unknown);
const title = ref<string>("");

function updateStatus(data: { status: Status; message: string }) {
  status.value = data.status ?? Status.Unknown;
  title.value = data.message;
}

let unsubscribeStatus = () => {};

onMounted(async () => {
	unsubscribeStatus = eventManager.subscribe("status", updateStatus);
	const statusString = await GetStatus();
	const status = JSON.parse(statusString) as {status: Status, message: string};

	updateStatus(status);
});

onUnmounted(() => {
	unsubscribeStatus();
});


const currentStatus = computed(() => StatusDisplayNames[status.value as Status]);
</script>

<template>
	<footer class="bg-neutral-800 py-1 text-xs text-gray-200" :title="title">
		{{ currentStatus }}
	</footer>
</template>

<style scoped>

</style>