// The context menu as the app calls it: menu(x, y, items) anywhere, drawn by GlimStone's ContextMenu in MenuHost.vue.
// Items are action ids, '-' for a separator, {head} for a heading, a plain {label, g, run} or {label, g, sub: [...]},
// which opens its items to the side.
import type { MenuEntry as GlimEntry } from '@glimstone/vue';
import { menuIsOpen } from '@glimstone/core';
import { reactive } from 'vue';
import { actions, blocked, labelOf, run, type Need } from './actions';
import { glyphComponent } from './glyphs';
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

export interface MenuOptions { above?: boolean; anchor?: HTMLElement | null }

export const menuState = reactive({
  list: null as Resolved[] | null,
  x: 0, y: 0, above: false, seq: 0,
  anchor: null as HTMLElement | null,
});

// A menu of the bar is GlimStone's own and not in menuState, so both count.
export const isMenuOpen = () => !!menuState.list || menuIsOpen();

export function menu(x: number, y: number, items: MenuEntry[], { above = false, anchor = null }: MenuOptions = {}) {
  hideTip();
  Object.assign(menuState, { list: resolveItems(items), x, y, above, anchor, seq: menuState.seq + 1 });
}

export function closeMenu() {
  Object.assign(menuState, { list: null, anchor: null });
}

const text = (v: string | (() => string)) => (typeof v === 'function' ? v() : v);

// An action that switches something on or off closes the menu like any action; only a plain box, such as a column
// in the column menu, keeps it open so several can be ticked in a row.
function pick(it: MenuItem) {
  if (it.id && actions[it.id]) {
    run(it.id);
    if (it.checked && !it.radio) closeMenu();
  } else (it.run as (() => unknown) | undefined)?.();
}

export function toGlim(list: Resolved[]): GlimEntry[] {
  return list.map((it): GlimEntry => {
    if (it === '-') return '-';
    if (isHead(it)) {
      const run = it.run;
      return { head: text(it.head), ...(run ? { edit: { tip: it.tip ?? '', glyph: it.g ? glyphComponent(it.g) : undefined, onSelect: () => { run(); } } } : {}) };
    }
    return {
      label: labelOf(it),
      ...(it.g && !it.color ? { glyph: glyphComponent(it.g) } : {}),
      ...(it.color ? { color: it.color } : {}),
      ...(it.key ? { shortcut: it.key } : {}),
      ...(it.count != null ? { count: it.count } : {}),
      ...(it.checked ? { checked: it.checked() } : {}),
      ...(it.radio ? { radio: true } : {}),
      ...(it.sub ? { sub: toGlim(resolveItems(it.sub)) } : {}),
      disabled: itemOff(it),
      onSelect: () => pick(it),
    };
  });
}
