// The computer's own context menu, the way Windows 11 has it: Weitere Optionen anzeigen at the end of the app's menu,
// and the whole system menu at once with Shift and a right click or with Shift+F10. On Linux the actions of the other
// file managers stand in for it. Beside it live Als Administrator ausführen, the way to the system's properties
// dialog and the system's icons.
import { h } from 'vue';
import { action } from '@/core/actions';
import { addTo } from '@/core/extend';
import { ext } from '@/core/glyphs';
import { bindKey } from '@/core/keymap';
import type { MenuEntry } from '@/core/menu';
import { prefs } from '@/core/prefs';
import type { Entry } from '@/files/entries';
import { addOpenMenuItems } from '@/files/menus';
import { inArchive, join } from '@/files/paths';
import { t, tl } from '@/i18n';
import { usePanels } from '@/stores/panels';
import { reachSystemMenu } from './index';
import PropsWell from './PropsWell.vue';
import { actionsSub, canAsk, iconUrl, loadShell, runAsAdmin, selection, shell, systemMenu, warn } from './shell';

// The selection of the row menu, or the folder itself for the menu of its empty space.
const pathsFor = (kind: 'row' | 'list') => (kind === 'list' ? [usePanels().T().path] : selection());

function moreOptions(kind: 'row' | 'list', x: number, y: number): MenuEntry | null {
  const paths = pathsFor(kind);
  if (!canAsk(paths)) return null;
  const row = { id: 'moreOptions', label: t('shellmenu.more'), g: 'IconMore' };
  return shell.menu === 'actions'
    ? { ...row, sub: actionsSub(paths) }
    : { ...row, run: () => systemMenu(paths, x, y, { background: kind === 'list' }) };
}

// programFor is the program the selection names, when Windows offers to start it as administrator.
function programFor(sel: Entry[], path: string): string | null {
  const one = sel.length === 1 ? sel[0]! : null;
  if (!one || one.dir || one.up || inArchive(path) || !shell.runAs.includes('.' + ext(one.name))) return null;
  return join(one.at || path, one.name);
}

// Shift+F10 opens the menu where the cursor row is, or that of the folder when no row is picked.
function fromKeyboard() {
  const at = document.querySelector('.panel.active .row.cursor')?.getBoundingClientRect();
  const x = at ? at.left + Math.min(48, at.width / 2) : innerWidth / 2, y = at ? at.bottom : innerHeight / 2;
  const paths = selection();
  void systemMenu(paths.length ? paths : pathsFor('list'), x, y, { extended: true, background: !paths.length });
}

export default function setup() {
  loadShell().catch(warn);
  reachSystemMenu((kind, items, { x, y, shift }) => {
    if (shift && canAsk(pathsFor(kind))) {
      void systemMenu(pathsFor(kind), x, y, { extended: true, background: kind === 'list' });
      return null;
    }
    const more = moreOptions(kind, x, y);
    return more ? [...items, '-', more] : items;
  });
  // An action of its own, so it can take a key like any other.
  const here = () => { const F = usePanels(); return programFor(F.sel(), F.T().path); };
  action('runAs', { label: tl('shellmenu.runAs'), g: 'IconShieldOn', when: () => !!here(), run: () => { const p = here(); if (p) void runAsAdmin(p); } });
  addOpenMenuItems((sel, path) => (programFor(sel, path) ? ['runAs'] : []));
  action('systemMenu', { label: tl('shellmenu.more'), g: 'IconMore', when: () => !!shell.menu, run: fromKeyboard });
  bindKey('Shift+F10', 'systemMenu');
  addTo('entryIcons', (k, folder, px) => iconUrl(k, folder, px, prefs.systemIcons === true));
  addTo('propsWells', (path) => (shell.properties ? h(PropsWell, { path }) : null));
}
