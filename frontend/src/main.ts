import {createApp} from 'vue'
import PrimeVue from 'primevue/config';
import App from './App.vue'
import './style.css';
import Aura from './components/prime/aura';
import ToastService from 'primevue/toastservice';
//@ts-ignore
import { install as VueMonacoEditorPlugin } from '@guolao/vue-monaco-editor'

const app = createApp(App);
app.use(PrimeVue, {
	unstyled: true,
	pt: Aura
});
app.use(ToastService);
app.use(VueMonacoEditorPlugin, {
	paths: {
		vs: 'https://cdn.jsdelivr.net/npm/monaco-editor@0.43.0/min/vs'
	},
});
app.mount('#app')
