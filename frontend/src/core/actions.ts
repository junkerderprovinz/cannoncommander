// An action is anything the menu bar, the context menu, a key or a button can run.
import { shallowReactive } from 'vue';
import { t } from '@/i18n';
import { extend } from './extend';
import { prefs } from './prefs';
import { toast } from './toast';

export type Need = 'sel' | 'file' | 'dual' | 'writable';
export interface Action {
  id: string;
  label: string | (() => string);
  g?: string;
  key?: string;
  menu?: string;
  order?: number;
  group?: string;
  ctx?: boolean;
  needs?: Need | Need[];
  radio?: boolean;
  run: (...args: never[]) => unknown;
  when?: () => boolean;
  checked?: () => boolean;
  // Kept out of the command search: it needs what its caller hands it, or a menu of its own already leads there.
  hidden?: boolean;
}
export type ActionDef = Omit<Action, 'id'>;

export const actions = shallowReactive<Record<string, Action>>({});

export const action = (id: string, def: ActionDef) => { actions[id] = { id, ...def }; };
export const labelOf = (a: { label?: string | (() => string) }) => (typeof a.label === 'function' ? a.label() : a.label ?? '');

export interface SelItem { name: string; dir?: boolean; at?: string }
export interface ActionContext { sel: () => SelItem[]; here: () => string }
let context: ActionContext = { sel: () => [], here: () => '/' };
export const setActionContext = (c: ActionContext) => { context = c; };

export const inArchive = (path: string) => path.includes('::');

export function blocked(a: Pick<Action, 'id' | 'needs'>): string | null {
  const need = ([] as Need[]).concat(a.needs || []);
  const sel = context.sel();
  if (need.includes('sel') && !sel.length) return t('core.needSel');
  if (need.length && !['copy', 'move', 'delete', 'clipCut', 'clipCopy'].includes(a.id) && sel.some((k) => k.at && k.at !== context.here())) return t('core.ownFolder');
  if (need.includes('file') && !sel.some((k) => !k.dir)) return t('core.needFile');
  if (need.includes('dual') && !prefs.dual) return t('core.needDual');
  if (need.includes('writable') && inArchive(context.here())) return t('core.archiveOnly');
  return null;
}

export function run(id: string, ...args: unknown[]) {
  const a = actions[id];
  if (!a) return toast(t('core.notBuilt', { id }), 'warn');
  const why = blocked(a);
  if (why) return toast(why, 'warn');
  const go = extend.actionWrappers.reduce((fn, wrap) => wrap(a, fn), a.run) as (...x: unknown[]) => unknown;
  return go(...args);
}
