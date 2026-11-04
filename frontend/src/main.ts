import '@glimstone/core/tokens.css';
import './styles/app.css';
import './styles/shell.css';
import { createPinia } from 'pinia';
import { createApp } from 'vue';
import { api } from '@/api/client';
import { connect } from '@/api/sse';
import { installAnnouncer } from '@/core/announce';
import { installKeyboard } from '@/core/keyboard';
import { bindDefaults } from '@/core/keymap';
import { installNav } from '@/core/nav';
import { applyPrefs, mergeServerPrefs, onPrefsSaved, prefs } from '@/core/prefs';
import { installTips } from '@/core/tips';
import { registerView } from '@/core/views';
import { loadFeatures } from '@/features';
import { registerFileActions } from '@/files/actions';
import FilesView from '@/files/FilesView.vue';
import { filesKey } from '@/files/keys';
import { setLanguage } from '@/i18n';
import SettingsPage from '@/pages/SettingsPage.vue';
import { usePanels } from '@/stores/panels';
import { watchWidth } from '@/shell/state';
import { useUser } from '@/stores/user';
import App from './App.vue';

applyPrefs();
setLanguage(prefs.language);
matchMedia('(prefers-color-scheme: light)').addEventListener('change', () => prefs.theme === 'system' && applyPrefs());

const app = createApp(App).use(createPinia());
registerView('files', { component: FilesView, key: filesKey });
registerView('settings', { page: true, component: SettingsPage });
bindDefaults();
usePanels();
registerFileActions();
loadFeatures();
installKeyboard();
installTips();
installAnnouncer();
installNav();
watchWidth();
useUser().load();

// The server's copy of the settings follows the local one; while the server has none, the local copy stands.
api.prefs().then(mergeServerPrefs, () => {});
let saving: ReturnType<typeof setTimeout> | undefined;
onPrefsSaved((p) => {
  clearTimeout(saving);
  saving = setTimeout(() => api.savePrefs(p).catch(() => {}), 800);
});
if (import.meta.env.MODE !== 'fixture') connect();

app.mount('#root');
