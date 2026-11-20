// The welcome window greets a first visit and stays in the help menu for later. The desktop app opens it from its
// own first start, once the server has said it is the desktop app.
import './welcome.css';
import { action } from '@/core/actions';
import { host } from '@/core/host';
import { prefs, setPref } from '@/core/prefs';
import { win, winStack } from '@/core/windows';
import { useStyle } from '@/features/desktop/style';
import { t, tl } from '@/i18n';
import WelcomeBody from './WelcomeBody.vue';
import { welcomeOnLoad } from './welcome';

// Closed without a pick, the app stays as it ships and the window does not come back by itself.
function settle() {
  if (host.desktop && !prefs.desktopStyle) useStyle('commander');
  setPref('welcomed', true);
}

export function openWelcome() {
  if (winStack.some((w) => w.cls === 'welcome')) return;
  win({
    title: t('welcome.title'),
    cls: 'welcome',
    wide: true,
    body: WelcomeBody,
    foot: [{ label: t('welcome.start'), g: 'IconCheck', primary: true, wayOut: true }],
    onClose: settle,
  });
}

export default function setup() {
  action('welcome', { label: tl('welcome.action'), g: 'IconApp', run: openWelcome });
  if (!import.meta.env.DESKTOP && welcomeOnLoad()) openWelcome();
}
