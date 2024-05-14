<script setup lang="ts">
import Editor from '../common/Editor.vue'
import Button from 'primevue/button'
import CascadeSelect from 'primevue/cascadeselect'
import Checkbox from 'primevue/checkbox';
import {ref} from "vue"
import { Engine, Model, Voice, engines } from '../common/voiceData';

const regexes = [
	{ regex: /^[^\S\r\n]*([^:\r\n]+):\s*(.*?)(?=\r?\n|$)/gm, className: 'matching-sentence' },
	{ regex: /^([^\s:]+):\s?(?=\S)/gm, className: 'matching-character' }
];

const selectedVoice = ref<Voice>();
const overrideVoices = ref<boolean>(false);
const saveNewCharacters = ref<boolean>(false);
</script>

<template>
	<div class="flex w-full h-full">
		<div class="w-1/5 p-2">

			<Button class="w-full" icon="pi pi-play" title="Play All" aria-label="Play" />
			<CascadeSelect
				:options="engines"
				:changeOnSelect="true"
				:optionGroupChildren="[ 'models', 'voices' ]"
				v-model="selectedVoice"
				class="w-full mt-2"
				optionLabel="name"
				optionGroupLabel="name"
				placeholder="Select a voice"
			/>
			<div class="flex items-center justify-center w-full pt-1">
				<Checkbox v-model="overrideVoices" inputId="overrideVoices" name="overrideVoices" value="1" />
				<label for="overrideVoices" class="ml-2 cursor-pointer select-none"> Override Voices </label>
			</div>
			<div class="flex items-center justify-center w-full pt-1">
				<Checkbox v-model="saveNewCharacters" inputId="saveNewCharacters" name="saveNewCharacters" value="1" />
				<label for="saveNewCharacters" class="ml-2 cursor-pointer select-none"> Save new characters </label>
			</div>
		</div>
		<div class="w-4/5">
			<Editor :regexes="regexes"/>
		</div>
	</div>
</template>

<style scoped>

</style>