// Öffnen mit, as in Explorer: the programs the computer offers for a file, each with its own icon and the default one
// first, then the system's own dialog to pick another. Only the desktop app shows it, since only there do the files
// and the programs live on the same computer. Explorer offers it for a single file and never for a folder.
import './openwith.css';
import { reactive } from 'vue';
import { call, json, q } from '@/api/client';
import { host } from '@/core/host';
import type { MenuEntry, MenuItem } from '@/core/menu';
import { win } from '@/core/windows';
import { addOpenMenuItems } from '@/files/menus';
import { inArchive, join } from '@/files/paths';
import { t, tl } from '@/i18n';
import { type Apps, openWith, warn } from './apps';
import ChooserBody from './ChooserBody.vue';

async function chooseOther(path: string, dialog: boolean) {
  if (!dialog) {
    win({ title: t('openwith.choose'), body: ChooserBody, props: { path } });
    return;
  }
  try {
    await call('/openwith/dialog', json('POST', { path }));
  } catch (e) {
    warn(e);
  }
}

// The menu opens at once and the rows fill in when the server has asked the system.
export function openWithItem(path: string): MenuItem {
  const sub = reactive<MenuEntry[]>([{ label: t('openwith.looking'), g: 'IconHourglass', disabled: true }]);
  call<Apps>('/openwith' + q({ path })).then(({ body }) => {
    const apps = body?.apps ?? [];
    const rows: MenuEntry[] = apps.length
      ? apps.map((a) => ({ label: a.name, img: a.icon, g: 'IconApp', run: () => openWith(path, a) }))
      : [{ label: t('openwith.none'), g: 'IconInfo', disabled: true }];
    sub.splice(0, sub.length, ...rows, '-', { label: t('openwith.other'), g: 'IconTabApps', run: () => chooseOther(path, !!body?.dialog) });
  }, (e: unknown) => {
    sub.splice(0, sub.length, { label: e instanceof Error ? e.message : String(e), g: 'IconWarning', disabled: true });
  });
  return { label: tl('openwith.menu'), g: 'IconExternalLink', sub };
}

export default function setup() {
  addOpenMenuItems((sel, path) => {
    const one = sel.length === 1 ? sel[0]! : null;
    if (!host.desktop || !one || one.dir || one.up || inArchive(path)) return [];
    return [openWithItem(join(path, one.name))];
  });
}
