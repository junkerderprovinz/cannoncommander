// Named extension points. Feature modules add to them instead of patching the core, so two features never fight over one function.
import { shallowReactive, type VNodeChild } from 'vue';
import type { Entry } from '@/files/entries';
import type { Tab } from '@/stores/panels';
import type { Action } from './actions';
import type { Toast } from './toast';

export interface ExtensionPoints {
  entryFilters: Array<(items: Entry[], path: string, panel: number) => Entry[]>;
  // A search word starting with # asks these whether an entry carries that colour or tag.
  tagMatchers: Array<(item: Entry, word: string, path: string) => boolean>;
  // Returning false from beforeGo stops the change of folder.
  beforeGo: Array<(path: string, panel: number) => boolean | void | Promise<boolean | void>>;
  afterGo: Array<(path: string, panel: number) => void>;
  // Text typed into the path field that one of these takes as its own, by returning true, goes nowhere else.
  pathWords: Array<(typed: string, panel: number) => boolean>;
  transferPipeline: Array<(job: Record<string, unknown>) => Record<string, unknown> | null>;
  footerSlots: Array<(panel: number) => VNodeChild>;
  // Inside the search field of the path row, such as its menu, and as bars between the path row and the list.
  filterSlots: Array<(panel: number) => VNodeChild>;
  panelBars: Array<(panel: number) => VNodeChild>;
  // A tab that shows something other than its folder names it here.
  tabLabels: Array<(tab: Tab) => { name: string; g?: string } | null>;
  nameBadges: Array<(item: Entry, path: string) => VNodeChild>;
  thumbProviders: Array<(item: Entry, path: string) => VNodeChild | null>;
  // The address of a picture that stands in for an entry's glyph, such as a program's own icon, px pixels square.
  entryIcons: Array<(item: Entry, path: string, px: number) => string | null>;
  // The sidebar's first card holds sideTop above the pinned folders and sideRows below them; sideCards come next.
  sideTop: Array<() => VNodeChild>;
  sideRows: Array<() => VNodeChild>;
  sideCards: Array<() => VNodeChild>;
  treeDecorators: Array<(path: string) => VNodeChild>;
  // Eigenschaften shows these below its head, for the one entry or folder it describes.
  propsWells: Array<(path: string) => VNodeChild>;
  actionWrappers: Array<(action: Action, run: Action['run']) => Action['run']>;
  onRender: Array<() => void>;
  toastHistory: Array<(toast: Toast) => void>;
}

export const extend: ExtensionPoints = {
  entryFilters: shallowReactive([]), tagMatchers: shallowReactive([]), beforeGo: shallowReactive([]), afterGo: shallowReactive([]), pathWords: shallowReactive([]), transferPipeline: shallowReactive([]),
  footerSlots: shallowReactive([]), filterSlots: shallowReactive([]), panelBars: shallowReactive([]), tabLabels: shallowReactive([]), nameBadges: shallowReactive([]), thumbProviders: shallowReactive([]), entryIcons: shallowReactive([]), sideTop: shallowReactive([]), sideRows: shallowReactive([]), sideCards: shallowReactive([]),
  treeDecorators: shallowReactive([]), propsWells: shallowReactive([]), actionWrappers: shallowReactive([]), onRender: shallowReactive([]), toastHistory: shallowReactive([]),
};

const orders = new WeakMap<object, number>();

// The rows below the pinned folders, top to bottom.
export const SIDE_ROWS = { recent: 10, collection: 20, parity: 30 };

// A lower order comes first, and the same order keeps the order of adding, so a part that loads later, as the desktop
// app's do, still takes the place it names.
export function addTo<K extends keyof ExtensionPoints>(point: K, fn: ExtensionPoints[K][number], order = 0) {
  const list = extend[point] as object[];
  orders.set(fn, order);
  const after = list.findIndex((f) => (orders.get(f) ?? 0) > order);
  list.splice(after < 0 ? list.length : after, 0, fn);
  return () => {
    const i = list.indexOf(fn);
    if (i >= 0) list.splice(i, 1);
  };
}
