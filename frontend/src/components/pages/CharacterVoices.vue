<script setup lang="ts">
import InputText from 'primevue/inputtext';
import Button from "primevue/button";
import Dropdown, {DropdownChangeEvent} from "primevue/dropdown";
import {computed, nextTick, onMounted, reactive, ref, watch} from "vue";
import {GetVoices, GetEngines, Play, GetCharacterVoices, SaveCharacterVoices} from '../../../wailsjs/go/main/App'
import {CharacterVoice, Engine, Model, Voice} from '../interfaces/engine';
import {formatToTreeSelectData} from "../../util/util";
import {useToast} from "primevue/usetoast";
import TreeSelect from "primevue/treeselect";
import {TreeNode} from "primevue/treenode";
const toast = useToast();

const engineModelNodes = ref<any[]>([]);
// const engineTreeNodes = ref<any[]>([]);
// const voices = ref<{ [key: string]: Voice[] }>({});
const engines = ref<{ [key: string]: Engine }>({});

const voiceOptions = ref<Record<string, Voice[]>>({});
const voiceOptionsMap = ref<Record<string, Voice[]>>({});
// const characterVoices = ref<{ [key: string]: CharacterVoice }>({});

const characterVoices = ref<Record<string, CharacterVoice>>({});


const selectedModels: Record<string, any> = reactive({});
const selectedVoices: Record<string, any> = reactive({});

async function getEngines() {
	const result = await GetEngines();
	try {
		// const engines: Engine[] = JSON.parse(result);
		const engineList: Engine[] = JSON.parse(result);

		for (const engine of engineList) {
			engines.value[engine.id] = engine;

			for(const index in engine.models) {
				if (engine.models.hasOwnProperty(index)) {
					const model = engine.models[index];
					await getVoices(engine.id, model.id);
				}
			}
		}

		return engines;
	} catch (error) {
		toast.add({ severity: 'error', summary: 'Error getting engines:', detail: error, life: 5000});
	}
}

async function getVoices(engine: string, model: string) {
	try {
		const result = await GetVoices(engine, model);
		const voicesList: Voice[] = JSON.parse(result);
		const key = `${engine}:${model}`;
		voiceOptions.value[key] = voicesList;
	} catch (error) {
		toast.add({ severity: 'error', summary: 'Error getting voices:', detail: String(error), life: 5000 });
	}
}

async function getCharacterVoices() {
	try {
		const result = await GetCharacterVoices();
		const characterVoiceData: { [key: string]: CharacterVoice } = JSON.parse(result);
		characterVoices.value = characterVoiceData;

		for (const name in characterVoiceData) {
			const { engine, model, voice } = characterVoiceData[name];
			selectedModels[name] = {
				[characterVoiceData[name].key]: true
			};
			selectedVoices[name] = voiceOptions.value[characterVoiceData[name].key][parseInt(voice)];
			voiceOptionsMap.value[name] = voiceOptions.value[engine + ":" + model];
		}

	} catch (error) {
		toast.add({
			severity: 'error',
			summary: 'Error getting character voices:',
			detail: String(error),
			life: 5000
		});
	}
}

onMounted(async () => {
	await getEngines();
	await getCharacterVoices();
	await nextTick();

	engineModelNodes.value = Object.values(engines.value).map(engine => ({
		key: engine.id,
		label: engine.name,
		selectable: false,
		children: Object.entries(engine.models ?? {}).map(([modelId, modelData]) => ({
			selectable: true,
			key: `${engine.id}:${modelId}`,
			label: modelData.name,
		}))
	}));
});

function onModelSelect(nodeKey: TreeNode, characterKey: string) {
	const [engineId, modelId] = nodeKey.key?.split(':') ?? '';
	const character = characterVoices.value[characterKey];
	character.engine = engineId;
	character.model = modelId;

	voiceOptionsMap.value[characterKey] = voiceOptions.value[`${engineId}:${modelId}`] || [];

}

function onVoiceSelect(event: DropdownChangeEvent) {
	console.log(selectedVoices);
	console.log(event);
}

const saveCharacterVoices = () => {
	console.log(characterVoices.value);
	console.log("characterVoices.value");
	const dataToSave = Object.entries(characterVoices.value).map(([key, voice]) => {
		const modelKey = Object.keys(selectedModels[key])[0];
		const [engine, model] = modelKey.split(':');
		const voiceID = selectedVoices[key].voiceID;

		return {
			key: modelKey,
			name: voice.name,
			engine: engine,
			model: model,
			voice: voiceID
		};
	});
	console.log("dataToSave");
	console.log(dataToSave);
	const dataString = JSON.stringify(dataToSave);
	console.log(dataString);

	SaveCharacterVoices(dataString);
}

</script>
<template>
	<div class="flex flex-col w-full h-full">
		<div class="w-full px-2 mb-2 flex">
			<Button class="mt-2 mr-2" icon="pi pi-save" title="Save All" label="Save All" aria-label="Save All" @click="saveCharacterVoices()" />
		</div>
		<div v-for="(voice, key) in characterVoices" :key="key" class="flex p-2 border-b">
			<div class="flex-grow p-2">
				<InputText v-model="voice.name" class="w-full" type="text" placeholder="Character name" />
			</div>
			<div class="flex-none p-2 flex items-center">
				<TreeSelect
					v-if="engineModelNodes && engineModelNodes.length > 0"
					:options="engineModelNodes"
					v-model="selectedModels[key]"
					@node-select="node => onModelSelect(node, key)"
					placeholder="Select a model"
					class="w-full"
				/>
			</div>
			<div class="flex-none p-2 flex items-center">
				 <Dropdown
					 @change="onVoiceSelect"
					 v-model="selectedVoices[key]"
					 :options="voiceOptionsMap[key]"
					 filter
					 optionLabel="name"
					 placeholder="Select a voice"
					 class="w-full ml-2 mt-2"
				 />
			</div>
			<div class="flex-none pl-2 flex flex-col items-start">
				<div class="flex justify-end w-full">
					<Button class="mt-2 mr-2 inline-block button-start" icon="pi pi-volume-up" title="Preview" aria-label="Preview" />
					<Button class="mt-2 inline-block button-stop" icon="pi pi-trash" title="Remove" aria-label="Remove" />
				</div>
			</div>
		</div>
	</div>
</template>

<style scoped>

</style>