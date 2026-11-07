// Als App installieren: the browser's own prompt where it offers one, otherwise how to do it by hand.
import './install.css';
import { watchEffect } from 'vue';
import { action } from '@/core/actions';
import { prefs } from '@/core/prefs';
import { toast } from '@/core/toast';
import { win } from '@/core/windows';
import { language, t, tl } from '@/i18n';
import InstallBody from './InstallBody.vue';

interface InstallPrompt extends Event { prompt(): void; userChoice: Promise<{ outcome: string }> }

// The manifest is a file of the server, as Android only installs an app, and offers it as a share target, from a
// manifest it can fetch again. The icons stand in the page itself, the colour follows the accent of the moment.
function head() {
  const put = (selector: string, make: () => HTMLElement, set: (el: HTMLElement) => void) => set(document.head.querySelector<HTMLElement>(selector) ?? document.head.appendChild(make()));
  const link = (rel: string) => () => Object.assign(document.createElement('link'), { rel });
  put('link[rel="manifest"]', link('manifest'), (el) => { (el as HTMLLinkElement).href = `${import.meta.env.BASE_URL}manifest/${language()}.webmanifest`; });
  put('meta[name="theme-color"]', () => Object.assign(document.createElement('meta'), { name: 'theme-color' }), (el) => { (el as HTMLMetaElement).content = prefs.accent; });
}

export default function setup() {
  watchEffect(head);
  let offer: InstallPrompt | null = null;
  addEventListener('beforeinstallprompt', (e) => { e.preventDefault(); offer = e as InstallPrompt; });
  addEventListener('appinstalled', () => { offer = null; toast(t('install.installed')); });
  const standalone = () => matchMedia('(display-mode: standalone)').matches || (navigator as { standalone?: boolean }).standalone === true;

  action('installApp', { label: tl('install.title'), g: 'IconInstall', run: () => {
    if (standalone()) return toast(t('install.already'));
    if (offer) {
      offer.prompt();
      offer.userChoice.then((c) => { if (c.outcome !== 'accepted') toast(t('install.declined')); });
      offer = null;
      return;
    }
    win({ title: t('install.title'), body: InstallBody, foot: [{ start: t('install.notOffered') }] });
  } });
}
