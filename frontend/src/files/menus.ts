// The file view's context menus. Ids no feature has registered yet drop out of the menu on their own.
import { run } from '@/core/actions';
import { menu, type MenuEntry, type MenuItem } from '@/core/menu';
import { prefs, setPref } from '@/core/prefs';
import { t } from '@/i18n';
import { newTab, usePanels } from '@/stores/panels';
import { COL, COLUMNS, Columns, DEFAULT_COLUMNS } from './columns';
import { inArchive, join } from './paths';

export function folderItems(path: string): MenuItem[] {
  const F = usePanels();
  return [
    { label: t('ctx.open'), g: 'IconFolderOpen', run: () => F.go(path) },
    { label: t('ctx.openOther'), g: 'IconCompare', run: () => { prefs.dual = true; F.go(path, F.other()); } },
    { label: t('ctx.openTab'), g: 'IconAdd', run: () => { const p = F.P(); p.tabs.push(newTab(path)); p.cur = p.tabs.length - 1; } },
  ];
}

export function rowMenu(x: number, y: number) {
  const F = usePanels();
  const tab = F.T(), sel = F.sel(), file = sel.find((k) => !k.dir);
  const oneFolder = sel.length === 1 && sel[0]!.dir && !sel[0]!.up && !inArchive(tab.path);
  const items: MenuEntry[] = [
    ...(oneFolder ? folderItems(join(tab.path, sel[0]!.name)) : ['view', 'edit', ...(file?.kind === 'arch' ? ['unpack', 'testArchive'] : [])]), '-',
    'copy', 'move', 'rename', 'clipCut', 'clipCopy', '-',
    'share', 'collect', 'download', 'pack', 'checksum', 'symlink', ...(sel.length === 2 && sel.every((k) => !k.dir) ? ['diff'] : []), '-',
    'props', 'fixPerms', 'copyPath', '-', 'delete',
  ];
  menu(x, y, items);
}

export const listMenu = (x: number, y: number) => menu(x, y, ['newFolder', 'newFile', 'upload', 'clipPaste', '-', 'selectAll', 'selectPattern', '-', 'refresh', 'props', 'fixPerms']);

export const tabMenu = (x: number, y: number) => menu(x, y, ['newTab', 'dupTab', 'lockTab', '-', 'closeTab', 'closeOtherTabs']);

const COLUMN_GROUPS: [string, string][] = [['general', 'IconFile'], ['rights', 'IconKey']];

function toggleColumn(id: string) {
  const cur = Columns.shown();
  if (COL[id]!.fixed) return;
  const next = cur.includes(id) ? cur.filter((x) => x !== id) : [...cur, id];
  setPref('columns', next.includes('name') ? next : ['name', ...next]);
}

export function columnMenu(x: number, y: number) {
  menu(x, y, [
    ...COLUMN_GROUPS.map(([gr, g]) => ({
      label: t(`columnGroups.${gr}`), g,
      sub: COLUMNS.filter((c) => c.group === gr && !c.fixed).map((c) => ({ label: c.label, checked: () => Columns.shown().includes(c.id), run: () => toggleColumn(c.id) })),
    })),
    '-',
    { label: t('ctx.chooseColumns'), g: 'IconSliders', run: () => run('columns') },
    { label: t('ctx.resetColumns'), g: 'IconRefresh', run: () => { prefs.colWidths = {}; setPref('columns', DEFAULT_COLUMNS); } },
  ]);
}
