<script setup lang="ts">
import '../css/footer.css';
import {Status, StatusDisplayNames} from "./enums/status";
import {computed, onMounted, onUnmounted, ref} from "vue";
import {eventManager} from "../util/eventManager";
import {GetStatus, OpenActivityLog} from "../../bindings/nstudio/app";

const status = ref<number>(Status.Unknown);
const title = ref<string>("");
let latestSequence = 0;

function updateStatus(data: { status: Status; message: string; sequence?: number }) {
	const sequence = data.sequence ?? 0;
	if (sequence < latestSequence) return;
	latestSequence = sequence;
	status.value = data.status ?? Status.Unknown;
	title.value = data.message;
}

let unsubscribeStatus = () => {
};
onMounted(async () => {
	unsubscribeStatus = eventManager.subscribe("status", updateStatus);
	const statusString = await GetStatus();
	const status = JSON.parse(statusString) as { status: Status, message: string };
	updateStatus(status);
});
onUnmounted(() => {
	unsubscribeStatus();
});

const currentStatus = computed(() => StatusDisplayNames[status.value as Status]);
</script>

<template>
	<footer class="footer" title="Open the Activity Log" @click="OpenActivityLog">
		<span v-html="currentStatus"/>
		<span v-if="title" class="footer__hint">{{ title }}</span>
	</footer>
</template>