<script lang="ts" setup>
// import HelloWorld from './components/HelloWorld.vue'
// import ColorPicker from 'primevue/colorpicker';
import Header from './components/Header.vue'
import {ComponentOptionsMixin, DefineComponent, ref} from "vue";
import Sandbox from './components/pages/Sandbox.vue';
import ScriptEditor from './components/pages/ScriptEditor.vue';
import CharacterVoices from './components/pages/CharacterVoices.vue';
import VoicePacks from './components/pages/VoicePacks.vue';
import Settings from './components/pages/Settings.vue';

const activePage = ref('sandbox');

function handleUpdateActivePage(newPage: string) {
	activePage.value = newPage;
	console.log(activePage.value);
}

interface pageComponent {
	[key: string]: any;
}


const pageComponents: pageComponent = {
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
		<main class="flex-grow bg-gray-400">
			<component :is="pageComponents[activePage]" />
		</main>

		<footer class="bg-gray-600">
			Made by Phyce
		</footer>
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