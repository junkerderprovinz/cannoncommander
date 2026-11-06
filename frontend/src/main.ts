import '@glimstone/core/tokens.css';
import './styles/app.css';
import './styles/shell.css';
import './styles/files.css';
import { createPinia } from 'pinia';
import { createApp, h, watch } from 'vue';
import { api, ApiError } from '@/api/client';
import { connect, disconnect } from '@/api/sse';
import { installAnnouncer } from '@/core/announce';
import { coverKeys, installKeyboard } from '@/core/keyboard';
import { bindDefaults } from '@/core/keymap';
import { installNav } from '@/core/nav';
import { applyPrefs, mergeServerPrefs, onPrefsSaved, prefs } from '@/core/prefs';
import { installTips } from '@/core/tips';
import { toast } from '@/core/toast';
import ToastHost from '@/core/ToastHost.vue';
import { registerView } from '@/core/views';
import { loadFeatures } from '@/features';
import { registerFileActions } from '@/files/actions';
import FilesView from '@/files/FilesView.vue';
import { installDragDrop } from '@/files/dnd';
import { filesKey, installJumpReset } from '@/files/keys';
import { setLanguage, useLocales } from '@/i18n';
import SettingsPage from '@/pages/SettingsPage.vue';
import { usePanels } from '@/stores/panels';
import { watchWidth } from '@/shell/state';
import { installSignOut, session, takeNote } from '@/signin';
import SignInPage from '@/signin/SignInPage.vue';
import { useUser } from '@/stores/user';
import App from './App.vue';

applyPrefs();
useLocales(import.meta.glob('./i18n/locales/*.json', { import: 'default' }));
setLanguage(prefs.language);
watch(() => prefs.language, setLanguage);
matchMedia('(prefers-color-scheme: light)').addEventListener('change', () => prefs.theme === 'system' && applyPrefs());
installTips();

// Nobody signed in sees only the sign in, so the file manager starts with a session or not at all.
function startSignIn() {
  createApp({ render: () => [h(SignInPage), h(ToastHost)] }).mount('#root');
}

function startApp() {
  const app = createApp(App).use(createPinia());
  registerView('files', { component: FilesView, key: filesKey });
  registerView('settings', { page: true, component: SettingsPage });
  bindDefaults();
  usePanels();
  registerFileActions();
  installDragDrop();
  installJumpReset();
  installSignOut();
  loadFeatures();
  installKeyboard();
  coverKeys(() => session.out);
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
  connect();
  watch(() => session.out, (out) => (out ? disconnect() : connect()));

  app.mount('#root');
  const note = takeNote();
  if (note) toast(note);
  if (import.meta.env.VITE_E2E === '1') import('./e2e/shim').then((m) => m.installShim());
}

// Only a clear 401 means signed out. A server that cannot say, as with the fixture's older parts, gets the app.
api.me().then(startApp, (e) => (e instanceof ApiError && e.status === 401 ? startSignIn() : startApp()));
