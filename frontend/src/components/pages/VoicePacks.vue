<script setup lang="ts">
import Card from 'primevue/card';
import InputSwitch from 'primevue/inputswitch';
import {onMounted, reactive, ref} from "vue";
import {Model} from '../interfaces/engine';
import { GetAvailableModels, GetSetting, SaveSetting, RefreshModels } from '../../../wailsjs/go/main/App'

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
	console.log("toggling models");
	console.log(modelToggles);
	const stringModelToggles = JSON.stringify(modelToggles);
	await SaveSetting("modelToggles", stringModelToggles).then(async () => {
		await RefreshModels();
	});
}
</script>

<template>
	<div class="flex flex-wrap mt-2 mb-2 mr-2">
		<div v-for="model in models" :key="model.engine + ':' + model.id" class="w-full md:w-1/3 pl-2 mb-2">
			<Card class="text-left">
				<template #title>{{ model.name }} {{model.key}}</template>
				<template #content>
					<div class="flex justify-between items-center">
						<div class="flex-grow">

							{{ model.engine }}
							{{ model.engine + ':' + model.id }}
						</div>
						<div class="flex-initial">
							<InputSwitch v-model="modelToggles[model.engine + ':' + model.id]" @update:modelValue="handleCheckboxToggle" />
						</div>
					</div>
				</template>
			</Card>
		</div>
	</div>
</template>