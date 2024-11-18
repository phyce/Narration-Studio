<script setup lang="ts">
import '../../css/pages/voice-packs.css';

import Card from 'primevue/card';
import Button from "primevue/button";
import InputSwitch from 'primevue/inputswitch';
import {onMounted, reactive, ref} from "vue";
import {Model} from '../interfaces/engine';
import {
	GetAvailableModels,
	GetSettings,
	RefreshModels,
	ReloadVoicePacks,
	SaveSettings,
} from '../../../wailsjs/go/main/App';
import {config as configuration} from "../../../wailsjs/go/models";
import configBase = configuration.Base;

const models = ref<Record<string, Model>>({});
const modelToggles = reactive<Record<string, boolean>>({});
const reloadModelsButtonDisabled = ref<boolean>(false);

onMounted(async () => {
	const savedModelToggles = (await GetSettings()).modelToggles;

	const availableModelsResult = await GetAvailableModels();
	models.value = JSON.parse(availableModelsResult);

	Object.entries(models.value).forEach(([key, model]) => {
		const toggleKey = `${model.engine}:${model.id}`;
		modelToggles[toggleKey] = savedModelToggles[toggleKey] ?? false;
	});
});

const reloadModels = async () => {
	reloadModelsButtonDisabled.value = true;

	await ReloadVoicePacks();

	const savedModelToggles = (await GetSettings()).modelToggles;
	models.value = await JSON.parse(await GetAvailableModels());

	Object.entries(models.value).forEach(([key, model]) => {
		const toggleKey = `${model.engine}:${model.id}`;
		modelToggles[toggleKey] = savedModelToggles[toggleKey] ?? false;
	});
	reloadModelsButtonDisabled.value = false;
};

const handleCheckboxToggle = async () => {
	const payload = new configBase;
	payload.modelToggles = modelToggles;

	await SaveSettings(payload).then(() => {
		RefreshModels();
	});
}
</script>

<template>
	<div class="voice-packs">
		<div class="voice-packs__header">
			<Button class="voices-packs__header__save"
					id="voices-packs__header__save"
					title="Reload voice packs & voices"
					aria-label="Reload voice packs & voices"
					@click="reloadModels"
					:disabled="reloadModelsButtonDisabled"
			>
				<i class="pi pi-refresh"/>&nbsp;
				Reload Packs
			</Button>
		</div>
		<div  class="voice-packs__container"
			  :key="key"
			  v-for="(model, key) in models"
		>
			<Card class="voice-pack">
				<template #title>{{ model.name }} {{model.key}}</template>
				<template #content>
					<div class="voice-pack__container">
						<div class="voice-pack__container__info" :title="model.engine + ':' + model.id">
							{{ model.engine + ':' + model.id }}
						</div>
						<div class="voice-pack__container__toggle">
							<InputSwitch
								v-model="modelToggles[model.engine + ':' + model.id]"
								@update:modelValue="handleCheckboxToggle"
							/>
						</div>
					</div>
				</template>
			</Card>
		</div>
	</div>
</template>