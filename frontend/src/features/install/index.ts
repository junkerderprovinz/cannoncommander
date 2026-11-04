// Als App installieren: the browser's own prompt where it offers one, otherwise how to do it by hand.
import './install.css';
import { action } from '@/core/actions';
import { toast } from '@/core/toast';
import { win } from '@/core/windows';
import { t, tl } from '@/i18n';
import InstallBody from './InstallBody.vue';

interface InstallPrompt extends Event { prompt(): void; userChoice: Promise<{ outcome: string }> }

export default function setup() {
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
