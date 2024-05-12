<script setup lang="ts">

import InputText from "primevue/inputtext";
import Editor from "../common/Editor.vue";
import InputGroup from "primevue/inputgroup";
import Button from "primevue/button";
import Dropdown from "primevue/dropdown";
import {ref} from "vue";

interface OutputType {
	id: number;
	name: string;
	label: string;
}

const selectedOutputType = ref<OutputType>();
const outputTypes: OutputType[] = [
	{
		id: 1,
		name: 'Combined File',
		label: 'Combined File'
	},
	{
		id: 2,
		name: 'Split Files',
		label: 'Split Files'
	}
];
</script>

<template>
	<div class="flex flex-col w-full h-full">
		<div class="w-full px-2 mb-2 flex">
			<Button class="mt-2 mr-2" icon="pi pi-save" title="Save Settings" label="Save Settings" aria-label="Save Settings" />
		</div>
		<div class="flex-grow background-secondary flex flex-col">
			<InputGroup class="">
				<InputText placeholder="Output Path" class="disabled:bg-neutral-800" disabled />
				<Button icon="pi pi-folder-open" title="Browse" aria-label="Generate" />
			</InputGroup>
			<InputGroup class="">
				<InputText placeholder="Output Path" class="disabled:bg-neutral-800" disabled />
				<Button icon="pi pi-folder-open" title="Browse" aria-label="Generate" />
			</InputGroup>
			<Dropdown v-model="selectedOutputType" :options="outputTypes" optionLabel="label" placeholder="Output Types" class="w-full ml-2 mt-2" />
		</div>
	</div>
</template>

<style scoped>

</style>