<script lang="ts" setup>
import Header from './components/Header.vue'
import Footer from './components/Footer.vue'
import {ComponentOptionsMixin, DefineComponent, ref} from "vue";
import Sandbox from './components/pages/Sandbox.vue';
import ScriptEditor from './components/pages/ScriptEditor.vue';
import CharacterVoices from './components/pages/CharacterVoices.vue';
import VoicePacks from './components/pages/VoicePacks.vue';
import Settings from './components/pages/Settings.vue';
import Start from './components/pages/Start.vue';
import {Status} from "./components/enums/status";
import Toast from 'primevue/toast';

const activePage = ref<string>('start');
const status = ref<number>(Status.Ready);

function handleUpdateActivePage(newPage: string) {
	activePage.value = newPage;
	console.log(activePage.value);
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

</script>


<template>
	<div class="flex flex-col h-full">
		<Header :activePage="activePage" @updateActivePage="handleUpdateActivePage"/>
		<main class="flex-grow bg-neutral-700 overflow-y-auto">
			<component :is="pageComponents[activePage]" />
		</main>
		<Footer :status="status" />
	</div>
</template>

<style>
#logo {
  display: block;
  width: 50%;
  height: 50%;
  margin: auto;
  padding: 10% 0 0;
  background-position: center;
  background-repeat: no-repeat;
  background-size: 100% 100%;
  background-origin: content-box;
}
</style>

<script lang="ts">

</script>