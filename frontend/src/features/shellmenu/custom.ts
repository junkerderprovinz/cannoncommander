// The person's own shape of the app's context menus: rows hidden, and rows moved within the group they stand in.
// Hidden rows are kept as menu:id, such as row:copy, the order as every id of a menu in the order its groups show.
import { actions, labelOf } from '@/core/actions';
import { host } from '@/core/host';
import type { MenuEntry } from '@/core/menu';
import { prefs, setPref } from '@/core/prefs';
import { t } from '@/i18n';

export type MenuKind = 'row' | 'list';

// The rows each menu can show, in its groups and in the order it shows them. A row a feature adds without an id here
// keeps its place and cannot be hidden.
export const CATALOGUE: Record<MenuKind, string[][]> = {
  row: [
    ['folderOpen', 'folderLink', 'folderOther', 'folderTab', 'folderBookmark', 'view', 'edit', 'openWith', 'runAs', 'unpack', 'testArchive'],
    ['copy', 'move', 'rename', 'clipCut', 'clipCopy'],
    ['share', 'collect', 'download', 'pack', 'checksum', 'symlink', 'diff'],
    ['labelColour', 'labelTags'],
    ['props', 'fixPerms', 'copyPath'],
    ['delete'],
    ['moreOptions'],
  ],
  list: [
    ['newFolder', 'newFile', 'upload', 'clipPaste'],
    ['selectAll', 'selectPattern'],
    ['refresh', 'props', 'fixPerms', 'terminal'],
    ['moreOptions'],
  ],
};

// Rows that are no action of their own, by the text and glyph the menu gives them.
const ROWS: Record<string, { label: () => string; g: string; desktop?: boolean }> = {
  folderOpen: { label: () => t('ctx.open'), g: 'IconFolderOpen' },
  folderLink: { label: () => t('ctx.internalLink'), g: 'IconLink' },
  folderOther: { label: () => t('ctx.openOther'), g: 'IconPaneSend' },
  folderTab: { label: () => t('ctx.openTab'), g: 'IconAdd' },
  folderBookmark: { label: () => t('ctx.addBookmark'), g: 'IconStar' },
  openWith: { label: () => t('openwith.menu'), g: 'IconExternalLink', desktop: true },
  runAs: { label: () => t('shellmenu.runAs'), g: 'IconShieldOn', desktop: true },
  labelColour: { label: () => t('labels.colour'), g: 'IconTabLook' },
  labelTags: { label: () => t('labels.tags'), g: 'IconHash' },
  moreOptions: { label: () => t('shellmenu.more'), g: 'IconMore', desktop: true },
};

export interface CatalogueRow { id: string; label: string; g: string }

// rowOf names a row for the settings, or null for one this app does not show here.
export function rowOf(id: string): CatalogueRow | null {
  const own = ROWS[id];
  if (own) return own.desktop && !host.desktop ? null : { id, label: own.label(), g: own.g };
  const a = actions[id];
  return a && (!a.when || a.when()) ? { id, label: labelOf(a), g: a.g ?? 'IconFile' } : null;
}

export const idOf = (it: MenuEntry) => (typeof it === 'string' ? (it === '-' ? '' : it) : 'head' in it ? '' : it.id ?? '');

const hiddenList = () => (Array.isArray(prefs.menuHidden) ? (prefs.menuHidden as string[]) : []);
const orders = () => (prefs.menuOrder && typeof prefs.menuOrder === 'object' ? (prefs.menuOrder as Partial<Record<MenuKind, string[]>>) : {});

export const isHidden = (kind: MenuKind, id: string) => hiddenList().includes(`${kind}:${id}`);
export const orderOf = (kind: MenuKind) => orders()[kind] ?? [];

export function setHidden(kind: MenuKind, id: string, hidden: boolean) {
  const key = `${kind}:${id}`;
  setPref('menuHidden', hidden ? [...new Set([...hiddenList(), key])] : hiddenList().filter((k) => k !== key));
}

// reorder puts the rows of list that order names into the order it gives them, each into a place one of them held,
// so the rows it does not name stay where they are.
export function reorder<T>(list: T[], order: string[], id: (it: T) => string): T[] {
  const rank = (it: T) => order.indexOf(id(it));
  const slots = list.flatMap((it, i) => (rank(it) >= 0 ? [i] : []));
  const moved = slots.map((i) => list[i]!).sort((a, b) => rank(a) - rank(b));
  const out = [...list];
  slots.forEach((slot, n) => { out[slot] = moved[n]!; });
  return out;
}

// groups is the menu's catalogue as the person ordered it.
export const groups = (kind: MenuKind, order = orderOf(kind)) => CATALOGUE[kind].map((g) => reorder(g, order, (id) => id));

// move swaps a row with its neighbour in its group, by step -1 up or 1 down, and keeps the whole menu's new order.
export function move(kind: MenuKind, id: string, step: -1 | 1) {
  const now = groups(kind);
  const g = now.find((list) => list.includes(id));
  if (!g) return;
  const i = g.indexOf(id), j = i + step;
  if (j < 0 || j >= g.length) return;
  [g[i], g[j]] = [g[j]!, g[i]!];
  setPref('menuOrder', { ...orders(), [kind]: now.flat() });
}

export function resetMenu(kind: MenuKind) {
  setPref('menuHidden', hiddenList().filter((k) => !k.startsWith(kind + ':')));
  const { [kind]: _, ...rest } = orders();
  setPref('menuOrder', rest);
}

export const customised = (kind: MenuKind) => orderOf(kind).length > 0 || hiddenList().some((k) => k.startsWith(kind + ':'));

// shape gives a menu the person's shape: hidden rows out, moved rows in their places, and no separator left over at
// an end or next to another.
export function shape(kind: MenuKind, items: MenuEntry[], hidden: (id: string) => boolean = (id) => isHidden(kind, id), order = orderOf(kind)): MenuEntry[] {
  const parts: MenuEntry[][] = [[]];
  for (const it of items) {
    if (it === '-') parts.push([]);
    else parts.at(-1)!.push(it);
  }
  const out: MenuEntry[] = [];
  for (const part of parts) {
    const kept = reorder(part.filter((it) => !idOf(it) || !hidden(idOf(it))), order, idOf);
    if (!kept.length) continue;
    if (out.length) out.push('-');
    out.push(...kept);
  }
  return out;
}
