// The file view's context menus. Ids no feature has registered yet drop out of the menu on their own.
import { run } from '@/core/actions';
import { menu, type MenuEntry, type MenuItem } from '@/core/menu';
import { prefs } from '@/core/prefs';
import { t } from '@/i18n';
import { newTab, usePanels } from '@/stores/panels';
import { addBookmark, copyInternalLink } from './bookmarks';
import { COLUMN_GROUPS, COLUMNS, columnLabel, Columns } from './columns';
import { inArchive, join } from './paths';

export function folderItems(path: string): MenuItem[] {
  const F = usePanels();
  return [
    { label: t('ctx.open'), g: 'IconFolderOpen', run: () => F.go(path) },
    { label: t('ctx.internalLink'), g: 'IconLink', run: () => copyInternalLink(path) },
    { label: t('ctx.openOther'), g: 'IconPaneSend', run: () => { prefs.dual = true; F.go(path, F.other()); } },
    { label: t('ctx.openTab'), g: 'IconAdd', run: () => { const p = F.P(); p.tabs.push(newTab(path)); p.cur = p.tabs.length - 1; } },
    { label: t('ctx.addBookmark'), g: 'IconStar', run: () => addBookmark(path) },
  ];
}

type RowExtra = (sel: ReturnType<ReturnType<typeof usePanels>['sel']>, path: string) => MenuEntry[];
// Features add their own rows for the selection here, such as colours and tags, between sharing and properties.
const rowExtras: RowExtra[] = [];
export const addRowMenuItems = (fn: RowExtra) => rowExtras.push(fn);
// Rows that open files another way stand below Ansehen and Bearbeiten, where Explorer keeps Öffnen mit.
const openExtras: RowExtra[] = [];
export const addOpenMenuItems = (fn: RowExtra) => openExtras.push(fn);

export function rowMenu(x: number, y: number) {
  const F = usePanels();
  const tab = F.T(), sel = F.sel(), file = sel.find((k) => !k.dir);
  const oneFolder = sel.length === 1 && sel[0]!.dir && !sel[0]!.up && !inArchive(tab.path);
  const items: MenuEntry[] = [
    ...(oneFolder ? folderItems(join(tab.path, sel[0]!.name)) : ['view', 'edit', ...openExtras.flatMap((fn) => fn(sel, tab.path)), ...(file?.archive || sel[0]?.kind === 'arch' ? ['unpack', 'testArchive'] : [])]), '-',
    'copy', 'move', 'rename', 'clipCut', 'clipCopy', '-',
    'share', 'collect', 'download', 'pack', 'checksum', 'symlink', ...(sel.length === 2 && sel.every((k) => !k.dir) ? ['diff'] : []), '-',
    ...rowExtras.flatMap((fn) => fn(sel, tab.path)), '-', 'props', 'fixPerms', 'copyPath', '-', 'delete',
  ];
  menu(x, y, items);
}

// Features add their own rows for the folder itself here, after the ones that make something new in it.
const listExtras: ((path: string) => MenuEntry[])[] = [];
export const addListMenuItems = (fn: (typeof listExtras)[number]) => listExtras.push(fn);

export const listMenu = (x: number, y: number) => menu(x, y, [
  'newFolder', 'newFile', 'upload', 'clipPaste', ...listExtras.flatMap((fn) => fn(usePanels().T().path)), '-', 'selectAll', 'selectPattern', '-', 'refresh', 'props', 'fixPerms', 'terminal',
]);

export const tabMenu = (x: number, y: number) => menu(x, y, ['newTab', 'dupTab', 'lockTab', '-', 'closeTab', 'closeOtherTabs']);

export function columnMenu(x: number, y: number) {
  menu(x, y, [
    ...COLUMN_GROUPS.map(([gr, g]) => ({
      label: t(`columnGroups.${gr}`), g,
      sub: COLUMNS.filter((c) => c.group === gr && !c.fixed).map((c) => ({ label: columnLabel(c.id), checked: () => Columns.shown().includes(c.id), run: () => Columns.toggle(c.id) })),
    })),
    '-',
    { label: t('ctx.chooseColumns'), g: 'IconSliders', run: () => run('columns') },
    { label: t('ctx.resetColumns'), g: 'IconRefresh', run: () => Columns.reset() },
  ]);
}
