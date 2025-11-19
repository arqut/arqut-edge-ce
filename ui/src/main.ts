import { createApp } from 'vue';
import { Quasar, Notify, Dialog, Loading } from 'quasar';
import { createPinia } from 'pinia';

import App from './App.vue';
import router from './router';

import '@quasar/extras/roboto-font/roboto-font.css';
import '@quasar/extras/material-icons/material-icons.css';
import 'quasar/dist/quasar.css';
import './css/app.scss';

const app = createApp(App);

app.use(Quasar, {
  plugins: {
    Notify,
    Dialog,
    Loading,
  },
  config: {
    notify: {},
    loading: {},
  },
});

app.use(createPinia());
app.use(router);

app.mount('#q-app');
