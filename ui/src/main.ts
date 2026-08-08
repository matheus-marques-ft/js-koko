import { createApp } from 'vue';
import VueCookies from 'vue3-cookies';

// Import directives
import { draggable } from '@/directive/sidebarDraggable.ts';

import App from './App.vue';
import pinia from './store';
import i18n from './locales';
import router from './router';
import './main.css';

// Import xterm styles
import '@xterm/xterm/css/xterm.css';

const app = createApp(App);

app.use(i18n);
app.use(pinia);
app.use(router);
app.use(VueCookies);

app.directive('draggable', draggable);

app.mount('#app');
