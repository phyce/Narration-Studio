<script lang="ts" setup>
import './css/app.css';
import Header from './components/Header.vue'
import Footer from './components/Footer.vue'
import {onMounted, onUnmounted, ref} from "vue";
import Sandbox from './components/pages/Sandbox.vue';
import ScriptEditor from './components/pages/ScriptEditor.vue';
import CharacterVoices from './components/pages/CharacterVoices.vue';
import VoicePacks from './components/pages/VoicePacks.vue';
import Settings from './components/pages/Settings.vue';
import Start from './components/pages/Start.vue';
import {eventManager} from "./util/eventManager";
import Toast, {ToastMessageOptions} from 'primevue/toast';
import {useToast} from "primevue/usetoast";

const toast = useToast();

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
	'character-voices': CharacterVoices,
	'voice-packs': VoicePacks,
	'settings': Settings
};

function showNotification(data: ToastMessageOptions) {
	const severity = data.severity || 'info';
	const summary = data.summary || '';
	const detail = data.detail || '';
	const life = data.life || 5000;

	if (life) toast.add({ severity, summary, detail, life });
	else toast.add({ severity, summary, detail });
}

let unsubscribeNotification: () => void;
onMounted(() => {
	unsubscribeNotification = eventManager.subscribe('notification', showNotification);
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
			<component :is="pageComponents[activePage]" />
		</main>
		<Footer class="app__footer" />
		<Toast position="bottom-center" />
	</div>
</template>