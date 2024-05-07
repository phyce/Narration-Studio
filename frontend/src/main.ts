import {createApp} from 'vue'
import PrimeVue from 'primevue/config';
import App from './App.vue'
import './style.css';
// import Lara from './components/prime/lara';

const app = createApp(App);
app.use(PrimeVue, {
	ungstyled: true,
	// pt: Lara
});
app.mount('#app')
