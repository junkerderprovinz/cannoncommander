// What the computer's own file manager offers in the desktop app: Explorer's menu on Windows, the actions of the other
// file managers on Linux, its properties dialog, starting a program as administrator and its icons.
import { reactive } from 'vue';
import { BASE, call, json, q } from '@/api/client';
import { host } from '@/core/host';
import { menu, type MenuEntry } from '@/core/menu';
import { toast } from '@/core/toast';
import { ext } from '@/core/glyphs';
import type { Entry } from '@/files/entries';
import { inArchive, join } from '@/files/paths';
import { t } from '@/i18n';
import { usePanels } from '@/stores/panels';

export interface Offers { menu: '' | 'native' | 'actions'; properties: boolean; runAs: string[] }
export interface ShellAction { key: string; label: string; icon?: string; group?: string }
export interface ShellEntry { key: string; label: string; icon?: string; hidden: boolean }

export const shell = reactive<Offers>({ menu: '', properties: false, runAs: [] });

export async function loadShell() {
  if (!host.desktop) return;
  const { body } = await call<Offers>('/shell');
  if (body) Object.assign(shell, body);
}

export const warn = (e: unknown) => toast(e instanceof Error ? e.message : String(e), 'warn');

// Programs, shortcuts and icon files carry their own icon, so they show it even without the system's icons.
const OWN_ICON = new Set(['exe', 'lnk', 'url', 'ico', 'cur', 'scr', 'cpl', 'msc', 'desktop', 'appimage']);
// The sizes the system keeps its icons in.
const SIZES = [16, 32, 48, 256];

// iconUrl is the address of the system's icon for an entry, or null where the glyph stays.
export function iconUrl(k: Entry, folder: string, px: number, everywhere: boolean): string | null {
  if (!host.desktop || k.up) return null;
  const size = String(SIZES.find((s) => s >= px * (globalThis.devicePixelRatio || 1)) ?? 256);
  const e = ext(k.name);
  const own = !k.dir && OWN_ICON.has(e);
  if (!own && !everywhere) return null;
  // A folder may have a picture of its own, such as Documents or Downloads, and so is asked for by its path.
  if ((own || k.dir) && !inArchive(folder)) return BASE + '/shell/icon' + q({ path: join(k.at || folder, k.name), size, v: String(k.time) });
  // Every other file shows the icon of its type, which one address per type serves to the whole list.
  return BASE + '/shell/icon' + q({ name: k.dir ? 'folder' : e ? 'file.' + e : 'file', size, ...(k.dir ? { dir: 'true' } : {}) });
}

// selection is the marked entries of the active panel, or the one under the cursor, as API paths.
export function selection(): string[] {
  const F = usePanels();
  const here = F.T().path;
  return F.sel().filter((k) => !k.up).map((k) => join(k.at || here, k.name));
}

export const canAsk = (paths: string[]) => !!shell.menu && paths.length > 0 && !paths.some(inArchive) && !paths.includes('/');

export interface MenuWay { extended?: boolean; background?: boolean }

// Explorer's own menu opens at x and y of the page, which the server turns into a place on the screen. background asks
// for the menu of the folder's empty space, for the one folder in paths.
async function nativeMenu(paths: string[], x: number, y: number, way: MenuWay) {
  const scale = globalThis.devicePixelRatio || 1;
  try {
    await call('/shell/menu', json('POST', { paths, x: Math.round(x * scale), y: Math.round(y * scale), ...way }));
  } catch (e) {
    warn(e);
  }
}

async function runAction(paths: string[], key: string) {
  try {
    await call('/shell/actions', json('POST', { paths, key }));
  } catch (e) {
    warn(e);
  }
}

// actionRows lays out the actions of the other file managers, those of a submenu under its name.
export function actionRows(paths: string[], list: ShellAction[]): MenuEntry[] {
  if (!list.length) return [{ label: t('shellmenu.noActions'), g: 'IconInfo', disabled: true }];
  const groups = new Map<string, MenuEntry[]>();
  const rows: MenuEntry[] = [];
  for (const a of list) {
    const row = { label: a.label, ...(a.icon ? { img: a.icon } : {}), g: 'IconApp', run: () => runAction(paths, a.key) };
    if (!a.group) {
      rows.push(row);
      continue;
    }
    if (!groups.has(a.group)) {
      groups.set(a.group, []);
      rows.push({ label: a.group, g: 'IconFolder', sub: groups.get(a.group)! });
    }
    groups.get(a.group)!.push(row);
  }
  return rows;
}

export async function actionsFor(paths: string[]): Promise<MenuEntry[]> {
  const { body } = await call<{ actions: ShellAction[] }>('/shell/actions?' + paths.map((p) => 'path=' + encodeURIComponent(p)).join('&'));
  return actionRows(paths, body?.actions ?? []);
}

// actionsSub is a submenu that opens at once and fills in when the server has looked.
export function actionsSub(paths: string[]): MenuEntry[] {
  const sub = reactive<MenuEntry[]>([{ label: t('shellmenu.looking'), g: 'IconHourglass', disabled: true }]);
  actionsFor(paths).then((rows) => sub.splice(0, sub.length, ...rows), (e: unknown) => {
    sub.splice(0, sub.length, { label: e instanceof Error ? e.message : String(e), g: 'IconWarning', disabled: true });
  });
  return sub;
}

// systemMenu opens the computer's menu for paths at x and y: Explorer's own, or the actions as the app's menu.
export async function systemMenu(paths: string[], x: number, y: number, way: MenuWay = {}) {
  if (!canAsk(paths)) return;
  if (shell.menu === 'native') return nativeMenu(paths, x, y, way);
  try {
    menu(x, y, await actionsFor(paths));
  } catch (e) {
    warn(e);
  }
}

export async function runAsAdmin(path: string) {
  try {
    await call('/shell/runas', json('POST', { path }));
  } catch (e) {
    warn(e);
  }
}

export async function systemProperties(paths: string[]) {
  try {
    await call('/shell/properties', json('POST', { paths }));
  } catch (e) {
    warn(e);
  }
}

export async function loadEntries(): Promise<ShellEntry[]> {
  const { body } = await call<{ entries: ShellEntry[] }>('/shell/entries');
  return body?.entries ?? [];
}

export const hideEntries = (keys: string[]) => call('/shell/entries/hidden', json('PUT', { keys }));
