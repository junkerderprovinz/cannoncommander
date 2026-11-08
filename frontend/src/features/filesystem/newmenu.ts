// Neu, as Explorer and the Linux file managers have it: a folder, a shortcut, a text file and whatever the installed
// programs and the templates folder offer.
import { reactive } from 'vue';
import { call, json } from '@/api/client';
import type { FsChanged } from '@/api/ops';
import { tools } from '@/api/tools';
import { action, actions } from '@/core/actions';
import { glyphOf } from '@/core/glyphs';
import { host, windowsPaths } from '@/core/host';
import { menu, type MenuEntry } from '@/core/menu';
import { toast } from '@/core/toast';
import { cancelBtn, win } from '@/core/windows';
import NameBody from '@/features/names/NameBody.vue';
import { join, typedPath } from '@/files/paths';
import { t, tl } from '@/i18n';
import { usePanels } from '@/stores/panels';
import { useUser } from '@/stores/user';
import { freeName, namesIn, reveal } from '@/ui/tools';
import ShortcutBody, { type ShortcutState } from './ShortcutBody.vue';

export interface Template { id: string; name: string; ext: string; file: string }

export const listTemplates = async () => (await call<{ templates: Template[] }>('/fs/templates')).body?.templates ?? [];
const fromTemplate = (path: string, template: string) => call<FsChanged>('/fs/new', json('POST', { path, template }));

function nameFor(title: string, value: string, make: (name: string) => Promise<unknown>) {
  const folder = usePanels().T().path, st = reactive({ name: value });
  win({
    title, body: NameBody, props: { st },
    foot: [cancelBtn(), { label: t('names.create'), g: 'IconAdd', primary: true, run: () => {
      const name = st.name.trim();
      if (!name) return false;
      make(join(folder, name)).then(() => reveal(join(folder, name)), (e) => toast((e as Error).message, 'warn'));
      return true;
    } }],
  });
}

export function newFrom(tpl: Template) {
  const folder = usePanels().T().path;
  nameFor(tpl.name, freeName(namesIn(folder), tpl.file + tpl.ext), (path) => fromTemplate(path, tpl.id));
}

// The target is typed the way Windows writes it, and the shortcut takes its name unless one is given.
export function newShortcut() {
  const folder = usePanels().T().path, st: ShortcutState = reactive({ target: '', name: '' });
  win({
    title: t('filesystem.newShortcut'), body: ShortcutBody, props: { st },
    foot: [cancelBtn(), { label: t('names.create'), g: 'IconLink', primary: true, run: () => {
      const target = typedPath(st.target, { windows: windowsPaths(), home: useUser().me.home || '/' });
      if (!target.startsWith('/') || target === '/') { toast(t('filesystem.noTarget'), 'warn'); return false; }
      const name = (st.name.trim() || t('filesystem.shortcutName', { name: target.split('/').pop()! })).replace(/(\.lnk)?$/i, '.lnk');
      tools.shortcut(target, join(folder, name)).then(() => reveal(join(folder, name)), (e) => toast((e as Error).message, 'warn'));
      return true;
    } }],
  });
}

function entries(list: Template[]): MenuEntry[] {
  return [
    'newFolder',
    ...(host.os === 'windows' ? [{ label: t('filesystem.shortcut'), g: 'IconLink', run: newShortcut }] : []),
    'newFile',
    ...(list.length ? ['-'] : []),
    ...list.map((tpl) => ({ label: tpl.name, g: glyphOf({ name: 'x' + tpl.ext }), run: () => newFrom(tpl) })),
  ];
}

// The desktop app has a submenu Neu in the menus of the list; a key or the palette opens it where the panel is.
export async function registerNewMenu() {
  const list = await listTemplates().catch(() => []);
  action('newMenu', {
    label: tl('filesystem.new'), g: 'IconAdd', needs: 'writable', run: () => {
      const r = document.querySelector('.panel.active .pathbar')?.getBoundingClientRect() || { left: 300, bottom: 120 };
      menu(r.left, r.bottom + 6, entries(list));
    },
  });
  Object.assign(actions.newMenu!, { sub: entries(list) });
}
