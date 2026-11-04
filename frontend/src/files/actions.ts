// The everyday actions of the file view. Feature modules add the rest under the ids the menu bar lists.
import { reactive } from 'vue';
import { action } from '@/core/actions';
import { menu } from '@/core/menu';
import { openMenuBar } from '@/core/menus';
import { prefs, setPref } from '@/core/prefs';
import { toast } from '@/core/toast';
import { go as goView, view } from '@/core/views';
import { win } from '@/core/windows';
import { t, tl } from '@/i18n';
import { shell } from '@/shell/state';
import { useListing } from '@/stores/listing';
import { newTab, usePanels } from '@/stores/panels';
import { copyText } from '@/ui/kit';
import { addBookmark, copyInternalLink } from './bookmarks';
import { Columns } from './columns';
import ColumnsBody from './ColumnsBody.vue';
import { GROUPS } from './entries';
import { topFolders } from './frequent';
import { join } from './paths';
import { VIEWS } from './views';

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
    const here = F.T().path, top = topFolders(5, here);
    const recent = F.visited.filter((p) => p !== here && !top.includes(p));
    const row = (p: string) => ({ label: p, g: 'IconFolder', run: () => F.go(p) });
    menu(r.left, r.bottom + 6, top.length || recent.length
      ? [...(top.length ? [{ head: t('files.frequent') }, ...top.map(row)] : []), ...(recent.length ? [{ head: t('files.recent') }, ...recent.map(row)] : [])]
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
    listing.changed(F.T().path);
    await listing.load(F.T().path);
    toast(t('files.refreshed'));
  } });

  action('newTab', { label: tl('actions.newTab'), g: 'IconAdd', key: 'Strg+T', run: () => { const p = F.P(); p.tabs.push(newTab(F.T().path)); p.cur = p.tabs.length - 1; } });
  action('closeTab', { label: tl('actions.closeTab'), g: 'IconClose', key: 'Strg+W', run: () => { const p = F.P(); if (p.tabs.length > 1) { p.tabs.splice(p.cur, 1); p.cur = Math.max(0, p.cur - 1); } } });
  action('dupTab', { label: tl('actions.dupTab'), g: 'IconCopy', run: () => { const p = F.P(), tab = F.T(); p.tabs.splice(p.cur + 1, 0, { ...newTab(tab.path), sort: tab.sort, dir: tab.dir }); p.cur += 1; } });
  action('lockTab', { label: tl('actions.lockTab'), g: 'IconPin', checked: () => !!F.T().locked, run: () => { F.T().locked = !F.T().locked; } });
  action('closeOtherTabs', { label: tl('actions.closeOtherTabs'), g: 'IconClose', when: () => F.P().tabs.length > 1, run: () => { const p = F.P(); p.tabs = [p.tabs[p.cur]!]; p.cur = 0; } });

  VIEWS.forEach(([id, glyph]) => action('view-' + id, {
    label: tl(`views.${id}`), g: glyph, radio: true, checked: () => F.viewOf(F.T()) === id, run: () => F.setView(F.T(), id),
  }));
  action('nextView', { label: tl('actions.nextView'), g: 'IconTabApps', key: 'Strg+Umschalt+V', run: () => {
    const tab = F.T(), k = VIEWS.findIndex((v) => v[0] === F.viewOf(tab)), next = VIEWS[(k + 1) % VIEWS.length]![0];
    F.setView(tab, next);
    toast(t('files.viewNow', { name: t(`views.${next}`) }));
  } });
  action('group-none', { label: tl('actions.groupNone'), g: 'IconViewSimple', radio: true, checked: () => !F.groupOf(F.T()), run: () => F.setGroup(F.T(), '') });
  Object.entries(GROUPS).forEach(([id, gr]) => action('group-' + id, {
    label: tl(`actions.group-${id}`), g: gr.g, radio: true, checked: () => F.groupOf(F.T()) === id, run: () => F.setGroup(F.T(), id),
  }));
  action('flat', { label: tl('actions.flat'), g: 'IconLog', key: 'Strg+Umschalt+B', checked: () => !!F.T().flat, run: () => {
    const tab = F.T();
    Object.assign(tab, { flat: !tab.flat, cursor: 0, marked: new Set() });
  } });
  action('columns', { label: tl('actions.columns'), g: 'IconSliders', run: () => {
    const cur = Columns.shown(), on = reactive(new Set(cur));
    win({
      title: t('files.columnsTitle'), wide: true, body: ColumnsBody, props: { on },
      foot: [
        { start: t('files.columnsHint') },
        { label: t('ctx.resetColumns'), g: 'IconRefresh', run: () => Columns.reset() },
        { label: t('files.apply'), g: 'IconApply', primary: true, run: () => Columns.set([...cur.filter((id) => on.has(id)), ...[...on].filter((id) => !cur.includes(id))]) },
      ],
    });
  } });

  action('back', { label: tl('actions.back'), g: 'IconBack', key: 'Alt+Links', run: () => { const tab = F.T(); if (!tab.hist.length) return; tab.fwd.push(tab.path); F.go(tab.hist.pop()!, F.active, false); } });
  action('forward', { label: tl('actions.forward'), g: 'IconForward', key: 'Alt+Rechts', run: () => { const tab = F.T(); if (!tab.fwd.length) return; tab.hist.push(tab.path); F.go(tab.fwd.pop()!, F.active, false); } });
  action('up', { label: tl('actions.up'), g: 'IconFolderUp', key: 'Rücktaste', run: () => F.T().path !== '/' && F.open(F.active, { up: true }) });
  action('home', { label: tl('actions.home'), g: 'IconFolder', run: () => F.go('/mnt/user') });
  action('gotoPath', { label: tl('actions.gotoPath'), g: 'IconPencil', key: 'Strg+L', run: () => {
    if (view.id !== 'files') goView('files');
    F.T().editPath = true;
  } });
  action('addBookmark', { label: tl('actions.addBookmark'), g: 'IconStar', run: (path: string = F.T().path) => addBookmark(path) });
  action('internalLink', { label: tl('actions.internalLink'), g: 'IconLink', needs: 'sel', when: () => F.sel().length === 1 && !!F.sel()[0]!.dir, run: () => {
    const k = F.sel()[0]!;
    copyInternalLink(join(k.at || F.T().path, k.name));
  } });
  action('quickFilter', { label: tl('actions.quickFilter'), g: 'IconSearch', key: 'Strg+F', run: () => {
    if (view.id !== 'files') goView('files');
    F.T().qf = true;
  } });
  action('typeJump', { label: tl('actions.typeJump'), g: 'IconSearch', key: t('files.typeJumpKey'), run: () => toast(t('files.typeJumpTip')) });

  action('selectAll', { label: tl('actions.selectAll'), g: 'IconSelectAll', key: 'Strg+A', run: () => { const tab = F.T(); F.entries().filter((k) => !k.up).forEach((k) => tab.marked.add(k.name)); } });
  action('invertSel', { label: tl('actions.invertSel'), g: 'IconSync', key: '*', run: () => {
    const tab = F.T();
    F.entries().filter((k) => !k.up).forEach((k) => (tab.marked.has(k.name) ? tab.marked.delete(k.name) : tab.marked.add(k.name)));
  } });
  action('unselect', { label: tl('actions.unselect'), g: 'IconClearSelection', key: '-', run: () => { F.T().marked = new Set(); } });
  action('copyPath', { label: tl('actions.copyPath'), g: 'IconLink', needs: 'sel', run: () => copyText(F.sel().map((k) => join(k.at || F.T().path, k.name)).join('\n'), t('files.pathCopied')) });

  action('settings', { label: tl('actions.settings'), g: 'IconGear', run: () => goView('settings') });
  action('menuBar', { label: tl('actions.menuBar'), g: 'IconMenu', key: 'F10', run: () => openMenuBar(0) });
}
