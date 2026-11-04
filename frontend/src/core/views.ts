// A view fills the main area: 'files' is the file manager, others are full pages with the head row above them.
import { type Component, markRaw, reactive, shallowReactive } from 'vue';
import { closeMenu } from './menu';
import { prefs } from './prefs';

export interface ViewDef { page?: boolean; component: Component; key?: (e: KeyboardEvent, combo: string) => void }
export const views = shallowReactive<Record<string, ViewDef>>({});
export const view = reactive({ id: 'files', arg: undefined as unknown });

export const registerView = (id: string, def: ViewDef) => { views[id] = { ...def, component: markRaw(def.component) }; };
export function go(id: string, arg?: unknown) {
  view.id = id;
  view.arg = arg;
  closeMenu();
}
export const currentView = () => views[view.id] ?? views.files;

export interface SettingsTab { id: string; label: string | (() => string); g: string; order: number; component: Component }
export const settingsTabs = shallowReactive<SettingsTab[]>([]);
export function settingsTab(def: SettingsTab) {
  settingsTabs.push({ ...def, component: markRaw(def.component) });
  settingsTabs.sort((a, b) => a.order - b.order);
}

// The person's own order of the settings tiles; a tile the stored order does not know yet follows its predecessor.
export function orderedTabs<T extends { id: string }>(tabs: T[], order: string[] = (prefs.tabOrder as string[]) || []): T[] {
  const out = order.map((id) => tabs.find((t) => t.id === id)).filter((t): t is T => !!t);
  tabs.forEach((t, i) => {
    if (out.includes(t)) return;
    const prev = tabs[i - 1];
    out.splice(prev ? out.indexOf(prev) + 1 : 0, 0, t);
  });
  return out;
}
