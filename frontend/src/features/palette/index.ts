// Befehl suchen (Strg+K): every action by its name, with the frequent folders beside them.
import './palette.css';
import { getActivePinia } from 'pinia';
import { createApp, watch } from 'vue';
import { action } from '@/core/actions';
import { closeMenu } from '@/core/menu';
import { topFolders } from '@/files/frequent';
import { t, tl } from '@/i18n';
import { usePanels } from '@/stores/panels';
import Palette from './Palette.vue';
import { sources } from './rows';
import { pal } from './state';

export default function setup() {
  action('palette', { label: tl('palette.title'), g: 'IconBolt', key: 'Ctrl+k', run: () => {
    if (pal.open) { pal.focus++; return; }
    closeMenu();
    pal.open = true;
  } });
  sources.push({
    head: () => t('palette.frequent'),
    list: () => topFolders(12).map((p) => ({ name: p, g: 'IconFolder', go: () => usePanels().go(p) })),
  });
  watch(() => pal.open, (open) => { if (!open) document.querySelector<HTMLElement>('.panel.active .list')?.focus(); });
  const host = document.body.appendChild(document.createElement('div'));
  const pinia = getActivePinia();
  const app = createApp(Palette);
  if (pinia) app.use(pinia);
  app.mount(host);
}
