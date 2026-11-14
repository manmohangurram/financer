import { createApp } from 'vue';
import App from './App.vue';
import router from './router';
import '@fontsource-variable/newsreader';
import '@fontsource-variable/ibm-plex-sans';
import './assets/main.css';

createApp(App).use(router).mount('#app');
