// Stand-in for GlimStone's context menu until @glimstone/vue ships one; MenuHost.vue draws this state with the prototype's DOM.
// Items are action ids, '-' for a separator, {head} for a heading, a plain {label, g, run} or {label, g, sub: [...]},
// which opens its items to the side.
import { reactive } from 'vue';
import { actions, blocked, run, type Need } from './actions';
import { hideTip } from './tips';

export interface MenuItem {
  id?: string;
  label?: string | (() => string);
  g?: string;
  key?: string;
  run?: (...args: never[]) => unknown;
  sub?: MenuEntry[];
  checked?: () => boolean;
  radio?: boolean;
  color?: string;
  count?: number;
  needs?: Need | Need[];
  when?: () => boolean;
}
export interface MenuHead { head: string | (() => string); run?: () => unknown; g?: string; tip?: string }
export type MenuEntry = string | MenuItem | MenuHead;
export type Resolved = '-' | MenuItem | MenuHead;

export const isHead = (it: Resolved | undefined): it is MenuHead => typeof it === 'object' && 'head' in it;

export const resolveItems = (items: MenuEntry[]): Resolved[] => items
  .map((it) => (typeof it === 'string' && it !== '-' ? actions[it] : it))
  .filter((it): it is Resolved => !!it)
  .filter((it, k, a) => it !== '-' || (k > 0 && k < a.length - 1 && a[k - 1] !== '-' && !isHead(a[k - 1])));

export const itemOff = (it: MenuItem) => !!(it.id && actions[it.id] && (blocked({ id: it.id, needs: it.needs }) || (it.when && !it.when())));

export interface MenuOptions { above?: boolean; anchor?: HTMLElement | null; bar?: number }
export interface Placed { left: number; top: number }

export const menuState = reactive({
  list: null as Resolved[] | null,
  items: [] as MenuEntry[],
  x: 0, y: 0, above: false, bar: -1,
  anchor: null as HTMLElement | null,
  at: null as Placed | null,
  focusK: -1,
  sub: null as { host: number; list: Resolved[]; focus: boolean; focusK: number } | null,
});

export const isMenuOpen = () => !!menuState.list;

export function menu(x: number, y: number, items: MenuEntry[], { above = false, anchor = null, bar = -1 }: MenuOptions = {}) {
  closeMenu();
  hideTip();
  Object.assign(menuState, { list: resolveItems(items), items, x, y, above, anchor, bar, at: null, focusK: -1, sub: null });
}

export function closeMenu() {
  Object.assign(menuState, { list: null, sub: null, bar: -1, anchor: null, at: null });
}

export function openSub(host: number, focus = false) {
  const it = menuState.list?.[host];
  if (!it || it === '-' || isHead(it) || !it.sub) return;
  menuState.sub = { host, list: resolveItems(it.sub), focus, focusK: -1 };
}

export const closeSub = () => { menuState.sub = null; };

// A checkbox item leaves the menu open where it was, so several boxes can be ticked in a row.
export function choose(it: MenuItem, k: number, placed: Placed, fromSub = false) {
  const { items, anchor, bar } = menuState;
  const host = menuState.sub?.host ?? -1;
  closeMenu();
  if (it.id && actions[it.id]) run(it.id);
  else (it.run as (() => unknown) | undefined)?.();
  if (it.checked && !it.id && !it.radio) {
    menu(placed.left, placed.top, items, { anchor, bar });
    menuState.at = placed;
    if (fromSub) {
      openSub(host);
      menuState.sub!.focusK = k;
    } else menuState.focusK = k;
  }
}
