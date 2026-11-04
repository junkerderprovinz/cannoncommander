// Als App installieren: the browser's own prompt where it offers one, otherwise how to do it by hand.
import './install.css';
import { watchEffect } from 'vue';
import { action } from '@/core/actions';
import { prefs } from '@/core/prefs';
import { toast } from '@/core/toast';
import { win } from '@/core/windows';
import { t, tl } from '@/i18n';
import { iconUrl } from './icon';
import InstallBody from './InstallBody.vue';

interface InstallPrompt extends Event { prompt(): void; userChoice: Promise<{ outcome: string }> }

// The manifest and the icons are built here, in the accent of the moment, so the page needs no files of its own for them.
function manifest() {
  const icon = iconUrl(prefs.accent), here = new URL('.', location.href).href;
  const data = (type: string, text: string) => `data:${type},${encodeURIComponent(text)}`;
  const body = {
    name: 'CannonCommander', short_name: 'Cannon', description: t('install.description'), lang: 'de',
    start_url: location.href.split('#')[0], scope: here, display: 'standalone', display_override: ['window-controls-overlay', 'standalone'],
    background_color: '#161616', theme_color: prefs.accent,
    icons: [{ src: icon, sizes: 'any', type: 'image/svg+xml', purpose: 'any' }, { src: icon, sizes: 'any', type: 'image/svg+xml', purpose: 'maskable' }],
  };
  const put = (selector: string, make: () => HTMLElement, set: (el: HTMLElement) => void) => set(document.head.querySelector<HTMLElement>(selector) ?? document.head.appendChild(make()));
  const link = (rel: string) => () => Object.assign(document.createElement('link'), { rel });
  put('link[rel="manifest"]', link('manifest'), (el) => { (el as HTMLLinkElement).href = data('application/manifest+json', JSON.stringify(body)); });
  put('link[rel="icon"]', link('icon'), (el) => { (el as HTMLLinkElement).href = icon; });
  put('link[rel="apple-touch-icon"]', link('apple-touch-icon'), (el) => { (el as HTMLLinkElement).href = icon; });
  put('meta[name="theme-color"]', () => Object.assign(document.createElement('meta'), { name: 'theme-color' }), (el) => { (el as HTMLMetaElement).content = prefs.accent; });
}

export default function setup() {
  watchEffect(manifest);
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
