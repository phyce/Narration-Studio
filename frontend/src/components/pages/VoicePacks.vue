<script setup lang="ts">
import '../../css/pages/voice-packs.css';

import Card from 'primevue/card';
import Button from "primevue/button";
import InputSwitch from 'primevue/inputswitch';
import {onMounted, reactive, ref} from "vue";
import {Model} from '../interfaces/engine';
import {
	GetAvailableModels,
	GetSetting,
	SaveSetting,
	RefreshModels,
	ReloadVoicePacks,
} from '../../../wailsjs/go/main/App';

const models = ref<Record<string, Model>>({});
const modelToggles = reactive<Record<string, boolean>>({});

onMounted(async () => {
	const savedModelTogglesResult = await GetSetting("modelToggles");
	const savedModelToggles = JSON.parse(JSON.parse(savedModelTogglesResult || '{}'));

	const availableModelsResult = await GetAvailableModels();
	models.value = JSON.parse(availableModelsResult);

	Object.entries(models.value).forEach(([key, model]) => {
		const toggleKey = `${model.engine}:${model.id}`;
		modelToggles[toggleKey] = savedModelToggles[toggleKey] ?? false;
	});
});

const handleCheckboxToggle = async () => {
	const stringModelToggles = JSON.stringify(modelToggles);
	await SaveSetting("modelToggles", stringModelToggles).then(() => {
		RefreshModels();
	});
}

const reloadModels = () => {
	ReloadVoicePacks();
}
</script>

<template>
	<div class="voice-packs">
		<div class="voice-packs__header">
			<Button class="voices-packs__header__save"
					title="Reload voice packs & voices"
					aria-label="Reload voice packs & voices"
					@click="reloadModels()"
			>
				<i class="pi pi-refresh"/>&nbsp;
				Reload Packs
			</Button>
		</div>
		<div  class="voice-packs__container"
			  :key="model.engine + ':' + model.id"
			  v-for="model in models"
		>
			<Card class="voice-pack">
				<template #title>{{ model.name }} {{model.key}}</template>
				<template #content>
					<div class="voice-pack__container">
						<div class="voice-pack__container__info" :title="model.engine + ':' + model.id">
							{{ model.engine + ':' + model.id }}
						</div>
						<div class="voice-pack__container__toggle">
							<InputSwitch v-model="modelToggles[model.engine + ':' + model.id]" @update:modelValue="handleCheckboxToggle" />
						</div>
					</div>
				</template>
			</Card>
		</div>
	</div>
</template>