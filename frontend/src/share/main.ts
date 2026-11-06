// The page a share link opens. It is its own entry, so a visitor loads the look of CannonCommander and nothing of
// the file manager behind it.
import '@glimstone/core/tokens.css';
import '../styles/app.css';
import './share.css';
import { createApp } from 'vue';
import { applyPrefs } from '@/core/prefs';
import { installTips } from '@/core/tips';
import { addMessages, setLanguage, useLocales } from '@/i18n';
import de from './de.json';
import en from './en.json';
import SharePage from './SharePage.vue';

addMessages('visit', { de, en });
applyPrefs();
// Visitors have no settings here, so the browser's language decides. A language loads with only the texts this page shows.
useLocales(import.meta.glob('../i18n/locales/*.json', { query: '?share', import: 'default' }));
setLanguage('');
installTips();
createApp(SharePage).mount('#root');
