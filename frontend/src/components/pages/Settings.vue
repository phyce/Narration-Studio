<script setup lang="ts">

import InputText from "primevue/inputtext";
import FloatLabel from "primevue/floatlabel";
import InputGroup from "primevue/inputgroup";
import InputGroupAddon from "primevue/inputgroupaddon";
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
		<div class="flex-grow background-secondary flex flex-col p-2">
			<InputGroup class="mb-2">
				<InputGroupAddon>Piper path</InputGroupAddon>
				<InputText placeholder="Select a directory" class="disabled:bg-neutral-800" disabled />
				<Button icon="pi pi-folder-open" title="Browse" aria-label="Browse" />
			</InputGroup>
			<InputGroup class="mb-2">
				<InputGroupAddon>Models directory</InputGroupAddon>
				<InputText placeholder="Output Path" class="disabled:bg-neutral-800" disabled />
				<Button icon="pi pi-folder-open" title="Browse" aria-label="Generate" />
			</InputGroup>
			<FloatLabel class="w-full md:w-14rem mt-5">
				<Dropdown v-model="selectedOutputType" :options="outputTypes" inputId="outputType" optionLabel="label" placeholder="select type" class="w-full" />
				<label for="outputType">Output type</label>
			</FloatLabel>
		</div>
	</div>
</template>

<style scoped>

</style>