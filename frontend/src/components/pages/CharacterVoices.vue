<script setup lang="ts">
import InputText from 'primevue/inputtext';
import Button from "primevue/button";
import Dropdown from "primevue/dropdown";
import TreeSelect from "primevue/treeselect";
import {ref} from "vue";
import {Engine, Model, Voice, engines} from "../common/voiceData";
import {/*findById,*/ formatToTreeSelectData} from "../../util/util";


// const treeNodes = formatToTreeSelectData(engines);

const selectedModel = ref<Model>();
const selectedVoice = ref<Voice>();
const voices = ref<Voice[]>([]);

const nodes = engines.value.map(engine => ({
	key: engine.id,
	label: engine.name,
	selectable: false,
	children: engine.models?.map(model => ({
		selectable: true,
		key: model.id,
		label: model.name,
		data: model
	}))
}));

// function onModelSelect(node: any) {
// 	const selected = findById(node.id, engines);
// 	console.log(selected);
// 	if (selected && 'voices' in selected) {
// 		voices.value = selected.voices as Voice[];
// 	}
// }


</script>

<template>
	<div class="flex flex-col w-full h-full">
		<div class="w-full px-2 mb-2 flex">
			<Button class="mt-2 mr-2" icon="pi pi-save" title="Save All" label="Save All" aria-label="Save All" />
			<Button class="mt-2 button-start" icon="pi pi-power-off" title="Start Preview" label="Start Preview" aria-label="Start Preview" />
		</div>
		<div class="flex-grow background-secondary flex">
			<div class="w-3/12 p-2">
				<InputText class="w-full" type="text"  placeholder="Character" />
			</div>
			<div class="w-3/12">
<!--				<TreeSelect :options="treeNodes" v-model="selectedModel" @node-select="onModelSelect" placeholder="Select a model" class="w-full mt-2" />-->
			</div>
			<div class="w-4/12">
				<Dropdown v-model="selectedVoice" :options="voices" filter optionLabel="name" placeholder="Select a voice" class="w-full ml-2 mt-2" />
			</div>
			<div class="w-2/12 pl-2 flex flex-col">
				<div>
					<Button class="mt-2 mr-2 inline-block button-start" icon="pi pi-volume-up" title="Preview" aria-label="Preview" />
					<Button class="mt-2 inline-block button-stop" icon="pi pi-trash" title="Remove" aria-label="Remove" />
				</div>
			</div>
		</div>
	</div>
</template>

<style scoped>

</style>