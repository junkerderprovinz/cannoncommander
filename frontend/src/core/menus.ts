// The menu bar: every top-level menu with the action ids it lists. Ids no feature has registered yet drop out on their own.
import { tl } from '@/i18n';
import type { MenuEntry, MenuItem } from './menu';

export const MENUS: [string, MenuEntry[]][] = [
  ['file', ['newFolder', 'folderWithSel', 'newFile', 'upload', 'download', '-', 'view', 'edit', 'share', 'props', '-', 'pack', 'unpack', 'checksum', 'checksumVerify', 'split', 'combine', 'symlink', '-', 'delete', 'openTrash', '-', 'logout']],
  ['edit', ['copy', 'move', 'rename', 'duplicate', 'multiRename', '-', 'clipCut', 'clipCopy', 'clipPaste', '-', 'touchSelect', 'selectAll', 'invertSel', 'selectPattern', 'unselect', 'saveSel', 'restoreSel', '-', 'copyPath', 'internalLink']],
  ['view', ['single', 'dual', 'sidebar', 'quickview', 'sidePane', 'fkeys', 'hidden', 'extensions', 'syncBrowse', '-',
    { label: tl('menus.look'), g: 'IconTabApps', sub: ['view-details', 'view-compact', 'view-list', 'view-content', 'view-columns', 'view-tiles', 'view-large', 'view-gallery', 'view-drives'] },
    { label: tl('menus.group'), g: 'IconFile', sub: ['group-none', 'group-kind', 'group-date', 'group-size'] },
    'flat', 'columns', { head: tl('menus.sort') }, 'sortName', 'sortSize', 'sortTime', 'sortType', '-', 'swapPanels', 'equalPanels', 'profiles', 'refresh']],
  ['go', ['back', 'forward', 'up', 'home', 'gotoPath', 'history', '-', 'BOOKMARKS', '-', 'openTrash', 'openShares', 'openActivity']],
  ['tools', ['search', 'quickFilter', '-', 'compareDirs', 'sync', 'diff', 'duplicates', 'diskUsage', 'terminal', '-', 'jobs', 'mounts']],
  ['help', ['palette', 'shortcuts', 'whatsNew', 'docs', 'installApp', 'glyphs', '-', 'about']],
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

// The bar is GlimStone's MenuBar in the head row; F10 and the F-key bar open its menus through the opener it registers.
let opener: (i: number) => void = () => {};
export const setMenuBarOpener = (fn: (i: number) => void) => { opener = fn; };
export const openMenuBar = (i: number) => opener(i);
