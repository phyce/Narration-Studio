<script setup lang="ts">
import '../css/header.css';
import { ref, defineProps, defineEmits } from 'vue';

const emit = defineEmits(['updateActivePage']);

const props = defineProps({
	activePage: String
});

const views = ref([
	// { id: 'streaming', display: 'Streaming'},
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
	<header class="header">
		<ul class="header__list">
			<li class="header__list__item"
				:key="view.id"
				v-for="view in views"
			>
				<button :class="[
							'header__list__item__button',
							currentView === view.id && 'button--active'
						]"
						@click="setCurrentView(view.id)"
				   		:aria-current="currentView === view.id ? 'page' : undefined"
				>
					{{ view.display }}
				</button>
			</li>
		</ul>
	</header>
</template>