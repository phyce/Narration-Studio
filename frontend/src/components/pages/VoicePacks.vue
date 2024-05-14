<script setup lang="ts">
import Card from 'primevue/card';
import InputSwitch from 'primevue/inputswitch';
import { Model, getModels} from "../common/voiceData";
import {onMounted, ref} from "vue";

// const engines = ref<Engine[]>([]);
const models = ref<Model[]>([]);
const checked = ref(true);

onMounted(() => {
	// engines.value = getEngines();
	models.value = getModels();
});
</script>

<template>
	<div class="flex flex-wrap mt-2 mb-2 mr-2">
		<div v-for="model in models" :key="model.id" class="w-full md:w-1/3 pl-2 mb-2">
			<Card class="text-left">
				<template #title>{{ model.name }}</template>
				<template #content>
					<div class="flex justify-between items-center">
						<div class="flex-grow">
							{{ model.engine }}
						</div>
						<div class="flex-initial">
<!--							<InputSwitch v-model="model.checked" />-->
							<InputSwitch v-model="checked" />
						</div>
					</div>
				</template>
			</Card>
		</div>
	</div>
</template>

<style scoped>

</style>