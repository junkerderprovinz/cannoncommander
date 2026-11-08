// What the desktop app keeps of the file system it runs on, the way the system's own file manager shows it.
import { h, watch } from 'vue';
import type { Item } from '@/api/client';
import { tools } from '@/api/tools';
import { action } from '@/core/actions';
import { addTo } from '@/core/extend';
import { host } from '@/core/host';
import { prefs, setPref } from '@/core/prefs';
import { toast } from '@/core/toast';
import { addListMenuItems, addRowMenuItems } from '@/files/menus';
import { join } from '@/files/paths';
import { plural, t, tl } from '@/i18n';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import { freeName, namesIn, reveal } from '@/ui/tools';
import CloudBadge from './CloudBadge.vue';
import { registerNewMenu } from './newmenu';

// Explorer's "always keep on this device" fetches the files, and "free up space" leaves only the placeholder.
export function keepOnDevice(sel: Item[], folder: string, keep: boolean) {
  const paths = sel.map((k) => join(k.at || folder, k.name));
  tools.attrs({ paths, flags: { pinned: keep } }).then(() => {
    useListing().changed(folder);
    toast(t(keep ? 'filesystem.kept' : 'filesystem.freed', { objects: plural(paths.length, 'ui.objects') }));
  }, (e) => toast(e.message, 'warn'));
}

// Explorer's "Create shortcut" puts a .lnk next to each selected entry.
export async function createShortcuts() {
  const F = usePanels(), folder = F.T().path, taken = namesIn(folder);
  let last = '';
  for (const k of F.sel()) {
    last = freeName(taken, t('filesystem.shortcutName', { name: k.name }) + '.lnk');
    taken.add(last);
    try {
      await tools.shortcut(join(k.at || folder, k.name), join(folder, last));
    } catch (e) {
      return toast((e as Error).message, 'warn');
    }
  }
  if (last) await reveal(join(folder, last));
}

export default function setup() {
  // Only Windows has protected system files, and the server says where it runs after the start.
  watch(() => host.os, (os) => {
    if (os !== 'windows') return;
    action('systemFiles', { label: tl('filesystem.systemFiles'), g: 'IconShieldOn', checked: () => !!prefs.showSystem, run: () => setPref('showSystem', !prefs.showSystem) });
    action('createShortcut', { label: tl('filesystem.createShortcut'), g: 'IconLink', needs: ['sel', 'writable'], run: createShortcuts });
  }, { immediate: true });

  const stop = watch(() => host.desktop, (desktop) => {
    if (!desktop) return;
    void registerNewMenu();
    queueMicrotask(() => stop());
  }, { immediate: true });

  addTo('nameBadges', (k) => (k.cloud ? h(CloudBadge, { k }) : null));
  // A launcher of the Linux desktop names itself; a command line says little.
  addTo('nameBadges', (k) => {
    const sc = k.shortcut, to = sc?.kind === 'desktop' ? sc.name : sc?.target;
    return to ? h('span', { class: 'link' }, `→ ${to}`) : null;
  });
  addRowMenuItems(() => (host.os === 'windows' ? ['createShortcut'] : []));
  addListMenuItems(() => ['newMenu']);
  addRowMenuItems((sel, path) => (sel.some((k) => k.cloud) ? [
    { label: t('filesystem.keep'), g: 'IconPin', run: () => keepOnDevice(sel, path, true) },
    { label: t('filesystem.free'), g: 'IconCloud', run: () => keepOnDevice(sel, path, false) },
  ] : []));
}
