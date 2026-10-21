import {createApp} from 'vue'
import PrimeVue from 'primevue/config';
import App from './App.vue'
import './style.css';
import Aura from './components/prime/aura';
import ToastService from 'primevue/toastservice';
import Tooltip from './util/tooltip';
//@ts-ignore
import {install as VueMonacoEditorPlugin, loader} from '@guolao/vue-monaco-editor'
import * as monaco from 'monaco-editor/esm/vs/editor/editor.api'
//@ts-ignore
import EditorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker'

(self as any).MonacoEnvironment = {
	getWorker() {
		return new EditorWorker();
	}
};
loader.config({monaco});

const app = createApp(App);
app.use(PrimeVue, {
	unstyled: true,
	pt: Aura
});
app.use(ToastService);
app.use(VueMonacoEditorPlugin);
app.directive('tooltip', Tooltip);
app.mount('#app');
