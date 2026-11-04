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
    { label: t('ctx.openOther'), g: 'IconCompare', run: () => { prefs.dual = true; F.go(path, F.other()); } },
    { label: t('ctx.openTab'), g: 'IconAdd', run: () => { const p = F.P(); p.tabs.push(newTab(path)); p.cur = p.tabs.length - 1; } },
    { label: t('ctx.addBookmark'), g: 'IconStar', run: () => addBookmark(path) },
  ];
}

// Features add their own rows for the selection here, such as colours and tags, between sharing and properties.
const rowExtras: ((sel: ReturnType<ReturnType<typeof usePanels>['sel']>, path: string) => MenuEntry[])[] = [];
export const addRowMenuItems = (fn: (typeof rowExtras)[number]) => rowExtras.push(fn);

export function rowMenu(x: number, y: number) {
  const F = usePanels();
  const tab = F.T(), sel = F.sel(), file = sel.find((k) => !k.dir);
  const oneFolder = sel.length === 1 && sel[0]!.dir && !sel[0]!.up && !inArchive(tab.path);
  const items: MenuEntry[] = [
    ...(oneFolder ? folderItems(join(tab.path, sel[0]!.name)) : ['view', 'edit', ...(file?.archive || sel[0]?.kind === 'arch' ? ['unpack', 'testArchive'] : [])]), '-',
    'copy', 'move', 'rename', 'clipCut', 'clipCopy', '-',
    'share', 'collect', 'download', 'pack', 'checksum', 'symlink', ...(sel.length === 2 && sel.every((k) => !k.dir) ? ['diff'] : []), '-',
    ...rowExtras.flatMap((fn) => fn(sel, tab.path)), '-', 'props', 'fixPerms', 'copyPath', '-', 'delete',
  ];
  menu(x, y, items);
}

export const listMenu = (x: number, y: number) => menu(x, y, ['newFolder', 'newFile', 'upload', 'clipPaste', '-', 'selectAll', 'selectPattern', '-', 'refresh', 'props', 'fixPerms']);

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
