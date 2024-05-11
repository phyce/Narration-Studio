<script setup lang="ts">
import Button from 'primevue/button'
import InputGroup from 'primevue/inputgroup'
import InputText from 'primevue/inputtext'
import Editor from "../common/Editor.vue";

const regexes = [
	{ regex: /^[^\S\r\n]*([^:\r\n]+):\s*(.*?)(?=\r?\n|$)/gm, className: 'matching-sentence' },
	{ regex: /^([^\s:]+):\s?(?=\S)/gm, className: 'matching-character' }
];
//pi-folder-open
</script>

<template>
	<div class="flex w-full h-full">
		<div class="w-1/5 p-2">
			<InputGroup class="">
				<InputText placeholder="Output Path" class="disabled:bg-neutral-800" disabled />
				<Button icon="pi pi-folder-open" title="Browse" aria-label="Generate" />
			</InputGroup>
			<Button class="w-full mt-2" icon="pi pi-upload" title="Generate" aria-label="Generate" />
		</div>
		<div class="w-4/5">
			<Editor :regexes="regexes"/>
		</div>
	</div>
</template>

<style scoped>

</style>