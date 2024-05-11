<script setup lang="ts">
import Editor from '../common/Editor.vue'
import Button from 'primevue/button'
import CascadeSelect from 'primevue/cascadeselect'
import Checkbox from 'primevue/checkbox';
import {ref} from "vue"

const regexes = [
	{ regex: /^[^\S\r\n]*([^:\r\n]+):\s*(.*?)(?=\r?\n|$)/gm, className: 'matching-sentence' },
	{ regex: /^([^\s:]+):\s?(?=\S)/gm, className: 'matching-character' }
];

interface Engine {
	id: number;
	name: string;
	models: Model[];
}
interface Model {
	id: number;
	name: string;
	voices: Voice[];
}
interface Voice {
	id: number;
	name: string;
	gender: string;
}

const selectedVoice = ref<Voice>();
const overrideVoices = ref<boolean>(false);
const models = ref<Engine[]>([
	{
		id: 1,
		name: 'Piper',
		models: [
			{
				id: 1,
				name: 'LibriTTS',
				voices: [
					{
						id: 1,
						name: 'Test voice 1',
						gender: 'male',
					},
					{
						id: 2,
						name: 'Test voice 2',
						gender: 'Female',
					},
				]
			},
		]
	},
	{
		id: 2,
		name: 'Suno Bark',
		models: [
			{
				id: 1,
				name: 'Default',
				voices: [
					{
						id: 1,
						name: 'Test voice 1',
						gender: 'male',
					},
					{
						id: 2,
						name: 'Test voice 2',
						gender: 'Female',
					},
				]
			},
		]
	},
	{
		id: 3,
		name: 'Microsoft',
		models: [
			{
				id: 1,
				name: 'SAPI 4',
				voices: [
					{
						id: 1,
						name: 'Test voice 1',
						gender: 'male',
					},
					{
						id: 2,
						name: 'Test voice 2',
						gender: 'Female',
					},
				]
			},
			{
				id: 1,
				name: 'SAPI 5',
				voices: [
					{
						id: 1,
						name: 'Test voice 1',
						gender: 'male',
					},
					{
						id: 2,
						name: 'Test voice 2',
						gender: 'Female',
					},
				]
			},
		]
	},
])
</script>

<template>
	<div class="flex w-full h-full">
		<div class="w-1/5 p-2">

			<Button class="w-full" icon="pi pi-play" title="Play All" aria-label="Play" />
			<CascadeSelect
				:options="models"
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
		</div>
		<div class="w-4/5">
			<Editor :regexes="regexes"/>
		</div>
	</div>
</template>

<style scoped>

</style>