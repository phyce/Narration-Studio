<script setup lang="ts">
import '../../css/pages/character-voices.css';
import InputText from 'primevue/inputtext';
import Button from "primevue/button";
import Dropdown, {DropdownChangeEvent} from "primevue/dropdown";
import {nextTick, onMounted, reactive, ref} from "vue";
import {GetVoices, GetEngines, Play, GetCharacterVoices, SaveCharacterVoices} from '../../../wailsjs/go/main/App'
import {CharacterVoice, Engine, Voice} from '../interfaces/engine';
import TreeSelect from "primevue/treeselect";
import {TreeNode} from "primevue/treenode";

const engineModelNodes = ref<any[]>([]);
const engines = ref<{ [key: string]: Engine }>({});
const voiceOptions = ref<Record<string, Voice[]>>({});
const voiceOptionsMap = ref<Record<string, Voice[]>>({});
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

		addEmptyCharacterVoice();

	} catch (error) {
		toast.add({
			severity: 'error',
			summary: 'Error getting character voices:',
			detail: String(error),
			life: 5000
		});
	}
}

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
  const dataToSave = Object.entries(characterVoices.value)
    .filter(([key, voice]) => {
      // Skip entries where the name is empty or selectedModels[key] is empty
      const hasName = voice.name && voice.name.trim() !== '';
      const hasSelection = selectedModels[key] && Object.keys(selectedModels[key]).length > 0;
      return hasName && hasSelection;
    })
    .reduce((accumulator, [key, voice]) => {
      const modelKeys = Object.keys(selectedModels[key]);
      const modelKey = modelKeys[0];
      const [engine, model] = modelKey.split(':');
      const voiceOption = selectedVoices[key];
      const voiceID = voiceOption ? voiceOption.voiceID : '';

      // Replace keys starting with '_' with the character's name
      const newKey = key.startsWith('_') ? voice.name : key;

      accumulator[newKey] = {
        key: modelKey,
        name: voice.name,
        engine: engine,
        model: model,
        voice: voiceID
      };

      return accumulator;
    }, {} as Record<string, CharacterVoice>);

  const dataString = JSON.stringify(dataToSave);
  console.log("Data to save:", dataToSave);

  SaveCharacterVoices(dataString);
};

async function previewVoice(voice: CharacterVoice) {
	console.log("should be playing")
	await Play(voice.name + ": " + voice.name, false, voice.model + voice.voice);
}

async function removeVoice(key: string, voice: CharacterVoice) {
	if(key in characterVoices.value) delete characterVoices.value[key];
}

function onNameInput(voice: any, key: string) {
	const keys = Object.keys(characterVoices.value);
	const lastKey = keys[keys.length - 1];
	console.log(keys);
	console.log(key, lastKey, keys.length);

	console.log("should be adding a voice now");
	if (key === lastKey) {
        if (voice.name && voice.name.trim() !== '') {
			addEmptyCharacterVoice();
        }
	}
}

function addEmptyCharacterVoice() {
	console.log("appending to characterVoices");
	const key = "_" + Object.keys(characterVoices.value).length;
	characterVoices.value[key] = {
		key: "",
		name: "",
		engine: "",
		model: "",
		voice: ""
	};

	selectedModels[key] = {};

  	selectedVoices[key] = null;

  	voiceOptionsMap.value[key] = [];
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

</script>
<template>
	<div class="voices">
		<div class="voices__actions">
			<Button class="voices__actions__save"
					title="Save All"
					aria-label="Save All"
					@click="saveCharacterVoices()"
			>
				<i class="pi pi-save"/>&nbsp;
				Save All
			</Button>
		</div>
		<div class="voices__entry"
			 :key="key"
			 v-for="(voice, key) in characterVoices"
		>
			<div class="voices__entry__name">
				<InputText class="voices__entry__name__input"
						   v-model="voice.name"
						   type="text"
						   placeholder="Character name"
						   @input="onNameInput(voice, key)"
				/>
			</div>
			<div class="voices__entry__model">
				<TreeSelect class="voices__entry__model__tree"
							v-if="engineModelNodes && engineModelNodes.length > 0"
							:options="engineModelNodes"
							v-model="selectedModels[key]"
							@node-select="node => onModelSelect(node, key)"
							placeholder="Select a model"
				/>
			</div>
			<div class="voices__entry__voice">
				 <Dropdown class="voices__entry__voice__dropdown"
						   @change="onVoiceSelect"
						   v-model="selectedVoices[key]"
						   :options="voiceOptionsMap[key]"
						   filter
						   optionLabel="name"
						   placeholder="Select a voice"
				 />
			</div>
			<div class="voices__entry__actions">
				<div class="voices__entry__actions__container">
					<Button class="button-start"
							@click="previewVoice(voice)"
							icon="pi pi-volume-up"
							title="Preview"
							aria-label="Preview"
					/>
					<Button class="button-stop"
							@click="removeVoice(key, voice)"
							icon="pi pi-trash"
							title="Remove"
							aria-label="Remove"
					/>
				</div>
			</div>
		</div>
	</div>
</template>