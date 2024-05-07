<script setup lang="ts">
	import { ref, defineProps, defineEmits } from 'vue';

	const emit = defineEmits(['updateActivePage']);

	const props = defineProps({
		activePage: String
	});

	const views = ref([
		{ id: 'sandbox', display: 'Sandbox'},
		{ id: 'script-editor', display: 'Script Editor'},
		{ id: 'character-voices', display: 'Character Voices' },
		{ id: 'voice-packs', display: 'Voice Packs' },
		{ id: 'settings', display: 'Settings' },
	]);

	const currentView = ref(props.activePage);

	function setCurrentView(view:string) {
		currentView.value = view;
		emit('updateActivePage', view);
	}
</script>

<template>
	<header className="bg-neutral-800">
		<div class="text-sm font-medium text-center text-gray-400 border-b border-gray-200">
			<ul class="flex flex-wrap text-center justify-center">
				<li v-for="view in views" :key="view.id" class="mr-2">
					<a href="#"
						@click="setCurrentView(view.id)"
					   :class="[
							'inline-block p-4 border-b-2 rounded-t-lg',
							currentView === view.id ? 'border-orange-600 text-orange-600' : 'border-transparent hover:border-gray-300'
						]"
					   :aria-current="currentView === view.id ? 'page' : undefined"
					>
						{{ view.display }}
					</a>
				</li>
			</ul>
		</div>
	</header>
</template>