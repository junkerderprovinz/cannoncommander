// The menu bar: every top-level menu with the action ids it lists. Ids no feature has registered yet drop out on their own.
import { tl } from '@/i18n';
import { menu, type MenuEntry, type MenuItem, menuState } from './menu';

export const MENUS: [string, MenuEntry[]][] = [
  ['file', ['newFolder', 'newFile', 'upload', 'download', '-', 'view', 'edit', 'share', 'props', '-', 'pack', 'unpack', 'checksum', 'checksumVerify', 'split', 'combine', 'symlink', '-', 'delete', 'openTrash', '-', 'logout']],
  ['edit', ['copy', 'move', 'rename', 'multiRename', '-', 'clipCut', 'clipCopy', 'clipPaste', '-', 'selectAll', 'invertSel', 'selectPattern', 'unselect', 'saveSel', 'restoreSel', '-', 'copyPath']],
  ['view', ['single', 'dual', 'sidebar', 'quickview', 'fkeys', 'hidden', 'extensions', 'syncBrowse', '-',
    { label: tl('menus.look'), g: 'IconTabApps', sub: ['view-details', 'view-compact', 'view-list', 'view-content', 'view-columns', 'view-tiles', 'view-large', 'view-gallery', 'view-drives'] },
    { label: tl('menus.group'), g: 'IconFile', sub: ['group-none', 'group-kind', 'group-date', 'group-size'] },
    'flat', 'columns', { head: tl('menus.sort') }, 'sortName', 'sortSize', 'sortTime', 'sortType', '-', 'swapPanels', 'equalPanels', 'profiles', 'refresh']],
  ['go', ['back', 'forward', 'up', 'home', 'gotoPath', 'history', '-', 'BOOKMARKS', '-', 'openTrash', 'openShares', 'openActivity']],
  ['tools', ['search', 'quickFilter', '-', 'compareDirs', 'sync', 'diff', 'duplicates', 'diskUsage', '-', 'jobs', 'mounts']],
  ['help', ['palette', 'shortcuts', 'whatsNew', 'docs', 'glyphs', '-', 'about']],
];

export const menuName = (i: number) => tl(`menus.${MENUS[i]![0]}`)();

let bookmarkEntries: () => MenuItem[] = () => [];
export const setBookmarkEntries = (fn: () => MenuItem[]) => { bookmarkEntries = fn; };

export function menuItems(i: number, bookmarks: MenuItem[] = bookmarkEntries()): MenuEntry[] {
  return MENUS[i]![1].flatMap((id) => (id === 'BOOKMARKS' ? [{ head: tl('menus.bookmarks') }, ...bookmarks] : [id]));
}

// Without room for the menu bar, every menu stacks into one list under its own heading.
export function allMenuItems(bookmarks: MenuItem[] = bookmarkEntries()): MenuEntry[] {
  return MENUS.flatMap((_, i) => [
    { head: menuName(i) },
    ...menuItems(i, bookmarks).filter((x) => typeof x !== 'object' || !('head' in x)),
  ]);
}

export function openMenuBar(i: number) {
  const btn = document.querySelectorAll<HTMLElement>('.menubar button')[i];
  if (!btn) return;
  const r = btn.getBoundingClientRect();
  menu(r.left, r.bottom + 6, menuItems(i), { anchor: btn, bar: i });
}

export const stepMenu = (d: number) => openMenuBar((menuState.bar + d + MENUS.length) % MENUS.length);
