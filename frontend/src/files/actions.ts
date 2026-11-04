// The everyday actions of the file view. Feature modules add the rest under the ids the menu bar lists.
import { action } from '@/core/actions';
import { menu } from '@/core/menu';
import { openMenuBar } from '@/core/menus';
import { prefs, setPref } from '@/core/prefs';
import { toast } from '@/core/toast';
import { go as goView, view } from '@/core/views';
import { tl, t } from '@/i18n';
import { shell } from '@/shell/state';
import { newTab, usePanels } from '@/stores/panels';
import { useListing } from '@/stores/listing';
import { join } from './paths';
import { VIEWS, viewOf } from './views';

const toggle = (k: string) => () => setPref(k, !prefs[k]);

export function registerFileActions() {
  const F = usePanels();
  const listing = useListing();

  action('single', { label: tl('actions.single'), g: 'IconViewSimple', key: 'Strg+1', checked: () => !prefs.dual, run: () => setPref('dual', false) });
  action('dual', { label: tl('actions.dual'), g: 'IconCompare', key: 'Strg+2', checked: () => prefs.dual, run: () => setPref('dual', true) });
  action('sidebar', { label: tl('actions.sidebar'), g: 'IconMenu', key: 'Strg+B', checked: () => prefs.side,
    run: () => (innerWidth <= 760 ? (shell.sideOpen = !shell.sideOpen) : setPref('side', !prefs.side)) });
  action('fkeys', { label: tl('actions.fkeys'), g: 'IconKeyboard', checked: () => prefs.fkeys, run: toggle('fkeys') });
  action('hidden', { label: tl('actions.hidden'), g: 'IconEye', key: 'Strg+H', checked: () => prefs.showHidden, run: toggle('showHidden') });
  action('extensions', { label: tl('actions.extensions'), g: 'IconFileText', checked: () => prefs.showExt, run: toggle('showExt') });
  action('syncBrowse', { label: tl('actions.syncBrowse'), g: 'IconSync', checked: () => prefs.syncBrowse, run: () => {
    if (!prefs.dual) prefs.dual = true;
    toggle('syncBrowse')();
    toast(prefs.syncBrowse ? t('files.syncOn') : t('files.syncOff'));
  } });
  action('history', { label: tl('actions.history'), g: 'IconLog', key: 'Alt+Runter', run: () => {
    const r = document.querySelector('.panel.active .pathbar')?.getBoundingClientRect() || { left: 300, bottom: 120 };
    const list = F.visited.filter((p) => p !== F.T().path);
    menu(r.left, r.bottom + 6, list.length
      ? [{ head: t('files.recent') }, ...list.map((p) => ({ label: p, g: 'IconFolder', run: () => F.go(p) }))]
      : [{ head: t('files.noHistory') }]);
  } });

  for (const [id, s] of [['sortName', 'name'], ['sortSize', 'size'], ['sortTime', 'time'], ['sortType', 'type']] as const) {
    action(id, { label: tl(`actions.${id}`), checked: () => F.T().sort === s, run: () => {
      const tab = F.T();
      if (tab.sort === s) tab.dir *= -1;
      else Object.assign(tab, { sort: s, dir: 1 });
    } });
  }
  action('swapPanels', { label: tl('actions.swapPanels'), g: 'IconSync', key: 'Strg+U', needs: 'dual', run: () => { F.panels.reverse(); } });
  action('equalPanels', { label: tl('actions.equalPanels'), g: 'IconCompare', key: 'Strg+.', needs: 'dual', run: () => F.go(F.T().path, F.other()) });
  action('refresh', { label: tl('actions.refresh'), g: 'IconRefresh', key: 'Strg+R', run: async () => {
    listing.invalidate(F.T().path);
    await listing.load(F.T().path);
    toast(t('files.refreshed'));
  } });

  action('newTab', { label: tl('actions.newTab'), g: 'IconAdd', key: 'Strg+T', run: () => { const p = F.P(); p.tabs.push(newTab(F.T().path)); p.cur = p.tabs.length - 1; } });
  action('closeTab', { label: tl('actions.closeTab'), g: 'IconClose', key: 'Strg+W', run: () => { const p = F.P(); if (p.tabs.length > 1) { p.tabs.splice(p.cur, 1); p.cur = Math.max(0, p.cur - 1); } } });
  action('dupTab', { label: tl('actions.dupTab'), g: 'IconCopy', run: () => { const p = F.P(), tab = F.T(); p.tabs.splice(p.cur + 1, 0, { ...newTab(tab.path), sort: tab.sort, dir: tab.dir }); p.cur += 1; } });
  action('lockTab', { label: tl('actions.lockTab'), g: 'IconPin', checked: () => !!F.T().locked, run: () => { F.T().locked = !F.T().locked; } });
  action('closeOtherTabs', { label: tl('actions.closeOtherTabs'), g: 'IconClose', when: () => F.P().tabs.length > 1, run: () => { const p = F.P(); p.tabs = [p.tabs[p.cur]!]; p.cur = 0; } });

  // Only Details and Kompakt have rows yet, and they share one layout.
  VIEWS.filter(([id]) => id === 'details' || id === 'compact').forEach(([id, glyph]) => action('view-' + id, {
    label: tl(`views.${id}`), g: glyph, radio: true, checked: () => viewOf(F.T()) === id,
    run: () => setPref('folderViews', { ...prefs.folderViews, [F.T().path]: id }),
  }));

  action('back', { label: tl('actions.back'), g: 'IconBack', key: 'Alt+Links', run: () => { const tab = F.T(); if (!tab.hist.length) return; tab.fwd.push(tab.path); F.go(tab.hist.pop()!, F.active, false); } });
  action('forward', { label: tl('actions.forward'), g: 'IconForward', key: 'Alt+Rechts', run: () => { const tab = F.T(); if (!tab.fwd.length) return; tab.hist.push(tab.path); F.go(tab.fwd.pop()!, F.active, false); } });
  action('up', { label: tl('actions.up'), g: 'IconFolderUp', key: 'Rücktaste', run: () => F.T().path !== '/' && F.open(F.active, { up: true }) });
  action('home', { label: tl('actions.home'), g: 'IconFolder', run: () => F.go('/mnt/user') });
  action('gotoPath', { label: tl('actions.gotoPath'), g: 'IconPencil', key: 'Strg+L', run: () => {
    if (view.id !== 'files') goView('files');
    F.T().editPath = true;
  } });

  action('selectAll', { label: tl('actions.selectAll'), g: 'IconSelectAll', key: 'Strg+A', run: () => { const tab = F.T(); F.entries().filter((k) => !k.up).forEach((k) => tab.marked.add(k.name)); } });
  action('invertSel', { label: tl('actions.invertSel'), g: 'IconSync', key: '*', run: () => {
    const tab = F.T();
    F.entries().filter((k) => !k.up).forEach((k) => (tab.marked.has(k.name) ? tab.marked.delete(k.name) : tab.marked.add(k.name)));
  } });
  action('unselect', { label: tl('actions.unselect'), g: 'IconClearSelection', key: '-', run: () => { F.T().marked = new Set(); } });
  action('copyPath', { label: tl('actions.copyPath'), g: 'IconLink', needs: 'sel', run: () => {
    navigator.clipboard?.writeText(F.sel().map((k) => join(F.T().path, k.name)).join('\n'));
    toast(t('files.pathCopied'));
  } });

  action('settings', { label: tl('actions.settings'), g: 'IconGear', run: () => goView('settings') });
  action('menuBar', { label: tl('actions.menuBar'), g: 'IconMenu', key: 'F10', run: () => openMenuBar(0) });
}

