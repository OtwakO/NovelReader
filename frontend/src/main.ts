import { createApp } from 'vue';
import App from './App.vue';
import { pinia } from './app/pinia';
import { router } from './app/router';
import { i18n, initializeI18n } from './i18n';
import { useSessionStore } from './stores/session';
import './ui/styles/base.css';

await initializeI18n();

const app = createApp(App);
app.use(pinia);
app.use(i18n);
app.use(router);

const session = useSessionStore(pinia);
const removeAuthenticationLossHandler = session.installAuthenticationLossHandler(() => {
  session.returnTo = router.currentRoute.value.meta.requiresAuth ? router.currentRoute.value.fullPath : '/shelf';
  void router.replace('/login');
});

window.addEventListener('pagehide', removeAuthenticationLossHandler, { once: true });
app.mount('#app');
