// Named extension points. Feature modules add to them instead of patching the core, so two features never fight over one function.
import { shallowReactive, type VNodeChild } from 'vue';
import type { Entry } from '@/files/entries';
import type { Action } from './actions';
import type { Toast } from './toast';

export interface ExtensionPoints {
  entryFilters: Array<(items: Entry[], path: string, panel: number) => Entry[]>;
  // A search word starting with # asks these whether an entry carries that colour or tag.
  tagMatchers: Array<(item: Entry, word: string, path: string) => boolean>;
  // Returning false from beforeGo stops the change of folder.
  beforeGo: Array<(path: string, panel: number) => boolean | void | Promise<boolean | void>>;
  afterGo: Array<(path: string, panel: number) => void>;
  transferPipeline: Array<(job: Record<string, unknown>) => Record<string, unknown> | null>;
  footerSlots: Array<(panel: number) => VNodeChild>;
  nameBadges: Array<(item: Entry, path: string) => VNodeChild>;
  thumbProviders: Array<(item: Entry, path: string) => VNodeChild | null>;
  sideRows: Array<() => VNodeChild>;
  treeDecorators: Array<(path: string) => VNodeChild>;
  actionWrappers: Array<(action: Action, run: Action['run']) => Action['run']>;
  onRender: Array<() => void>;
  toastHistory: Array<(toast: Toast) => void>;
}

export const extend: ExtensionPoints = {
  entryFilters: shallowReactive([]), tagMatchers: shallowReactive([]), beforeGo: shallowReactive([]), afterGo: shallowReactive([]), transferPipeline: shallowReactive([]),
  footerSlots: shallowReactive([]), nameBadges: shallowReactive([]), thumbProviders: shallowReactive([]), sideRows: shallowReactive([]),
  treeDecorators: shallowReactive([]), actionWrappers: shallowReactive([]), onRender: shallowReactive([]), toastHistory: shallowReactive([]),
};

export function addTo<K extends keyof ExtensionPoints>(point: K, fn: ExtensionPoints[K][number]) {
  (extend[point] as unknown[]).push(fn);
  return () => {
    const list = extend[point] as unknown[];
    const i = list.indexOf(fn);
    if (i >= 0) list.splice(i, 1);
  };
}
