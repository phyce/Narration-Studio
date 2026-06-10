<script lang="ts" setup>
import './css/app.css';
import Header from './components/Header.vue'
import Footer from './components/Footer.vue'
import {onMounted, onUnmounted, ref} from "vue";
import Sandbox from './components/pages/Sandbox.vue';
import ScriptEditor from './components/pages/ScriptEditor.vue';
import Profiles from './components/pages/Profiles.vue';
import VoicePacks from './components/pages/VoicePacks.vue';
import Settings from './components/pages/Settings.vue';
import Start from './components/pages/Start.vue';
import Server from './components/pages/Server.vue';
import {eventManager} from "./util/eventManager";
import Toast, {ToastMessageOptions} from 'primevue/toast';
import {useToast} from "primevue/usetoast";
import {OpenActivityLog, OpenActivityLogChain} from "../bindings/nstudio/app";

const toast = useToast();
const toastDetailLimit = 180;

const activePage = ref<string>('start');

function handleUpdateActivePage(newPage: string) {
	activePage.value = newPage;
}

interface pageComponent {
	[key: string]: any;
}

const pageComponents: pageComponent = {
	'start': Start,
	'sandbox': Sandbox,
	'script-editor': ScriptEditor,
	'profiles': Profiles,
	'voice-packs': VoicePacks,
	'settings': Settings,
	'server': Server
};

function showNotification(data: ToastMessageOptions & {chainId?: number}) {
	const severity = data.severity || 'info';
	const summary = data.summary || '';
	let detail = data.detail || '';
	if (detail.length > toastDetailLimit) {
		detail = detail.slice(0, toastDetailLimit) + '…';
	}
	const life = data.life || 5000;

	toast.add({severity, summary, detail, life, chainId: data.chainId} as ToastMessageOptions);
}

function openToastInActivityLog(message: any) {
	if (typeof message.chainId === 'number' && message.chainId > 0) {
		OpenActivityLogChain(message.chainId);
	} else {
		OpenActivityLog();
	}
}

function toastIcon(severity: string): string {
	switch (severity) {
		case 'error': return 'pi-times-circle';
		case 'warn': return 'pi-exclamation-triangle';
		case 'success': return 'pi-check';
		default: return 'pi-info-circle';
	}
}

let unsubscribeNotification: () => void;
onMounted(() => {
	unsubscribeNotification = eventManager.subscribe('notification.send', showNotification);
});
onUnmounted(() => {
	unsubscribeNotification();
});
</script>


<template>
	<div class="app">
		<Header class="app__header"
				:activePage="activePage"
				@updateActivePage="handleUpdateActivePage"
		/>
		<main class="app__main">
			<component :is="pageComponents[activePage]"/>
		</main>
		<Footer class="app__footer"/>
		<Toast position="bottom-center">
			<template #message="slotProps">
				<div class="toast-content" :title="'Open in Activity Log'" @click="openToastInActivityLog(slotProps.message)">
					<span :class="['p-toast-message-icon', 'pi', toastIcon(slotProps.message.severity ?? 'info')]"/>
					<div class="p-toast-message-text">
						<span class="p-toast-summary">{{ slotProps.message.summary }}</span>
						<div class="p-toast-detail">{{ slotProps.message.detail }}</div>
					</div>
				</div>
			</template>
		</Toast>
	</div>
</template>

<style scoped>
.toast-content {
	display: flex;
	align-items: flex-start;
	gap: 8px;
	flex: 1;
	cursor: pointer;
	text-align: left;
	overflow-wrap: anywhere;
}
</style>