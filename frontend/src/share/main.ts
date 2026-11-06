// The page a share link opens. It is its own entry, so a visitor loads the look of CannonCommander and nothing of
// the file manager behind it.
import '@glimstone/core/tokens.css';
import '../styles/app.css';
import './share.css';
import { createApp } from 'vue';
import { applyPrefs } from '@/core/prefs';
import { installTips } from '@/core/tips';
import { addMessages, setLanguage } from '@/i18n';
import de from './de.json';
import en from './en.json';
import SharePage from './SharePage.vue';

addMessages('visit', { de, en });
applyPrefs();
// Visitors have no settings here, so the browser's language decides.
setLanguage(navigator.language?.toLowerCase().startsWith('de') ? 'de' : 'en');
installTips();
createApp(SharePage).mount('#root');
