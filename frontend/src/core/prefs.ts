// Settings per user. The copy in localStorage paints the first frame right; the server's copy follows and wins.
import { applyShape, DEFAULT_ACCENT } from '@glimstone/core';
import { reactive } from 'vue';

export const PREFS_KEY = 'cannoncommander-prefs';

export const DEFAULT_PREFS = {
  theme: 'dark', accent: DEFAULT_ACCENT, shape: 'round', motion: 'subtle', quietToasts: false,
  labelsButtons: 'textGlyph', labelsSidebar: 'textGlyph', labelsTabs: 'textGlyph', labelsBottombar: 'textGlyph',
  rainbow: false, reactive: false, rotate: false, tabOrder: [] as string[],
  columns: ['name', 'size', 'modified'], colWidths: {} as Record<string, number>,
  density: 'medium', showHidden: false, showExt: true, syncBrowse: false, folderSizes: true, foldersFirst: true, dblclick: 'open', fkeys: false, side: true, dual: true,
  quickview: false, confirmDelete: true, trash: true, trashDays: 30, language: 'de', dateFormat: 'short', startFolder: '/mnt/user/downloads',
  folderViews: {} as Record<string, string>, folderGroups: {} as Record<string, string>,
};
export type Prefs = typeof DEFAULT_PREFS & Record<string, unknown>;

// A stored single colour mode or label mode maps onto the split prefs.
export function migratePrefs(saved: Record<string, unknown>) {
  if (saved.colorMode) {
    saved.rainbow = saved.colorMode === 'rainbow';
    saved.reactive = saved.colorMode === 'reactive';
    delete saved.colorMode;
  }
  if (saved.labels) {
    for (const a of ['Buttons', 'Sidebar', 'Tabs', 'Bottombar']) saved['labels' + a] ??= saved.labels;
    delete saved.labels;
  }
  return saved;
}

function stored(): Record<string, unknown> {
  try { return migratePrefs(JSON.parse(localStorage.getItem(PREFS_KEY) || '{}')); } catch { return {}; }
}

export const prefs = reactive({ ...structuredClone(DEFAULT_PREFS), ...stored() }) as Prefs;

let saveToServer: ((p: Prefs) => void) | null = null;
export const onPrefsSaved = (fn: (p: Prefs) => void) => { saveToServer = fn; };

export function setPref(key: string, value: unknown) {
  prefs[key] = value;
  try { localStorage.setItem(PREFS_KEY, JSON.stringify(prefs)); } catch { /* private mode keeps the choice for this visit */ }
  applyPrefs();
  saveToServer?.(prefs);
}

export function mergeServerPrefs(server: Record<string, unknown>) {
  Object.assign(prefs, migratePrefs({ ...server }));
  applyPrefs();
}

export function applyPrefs(r: HTMLElement = document.documentElement) {
  r.dataset.theme = prefs.theme === 'system' ? (matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark') : prefs.theme;
  r.style.setProperty('--accent', prefs.accent);
  applyShape(prefs.shape);
  r.dataset.density = prefs.density;
  r.dataset.lButtons = prefs.labelsButtons;
  r.dataset.lSidebar = prefs.labelsSidebar;
  r.dataset.lTabs = prefs.labelsTabs;
  r.dataset.lBottombar = prefs.labelsBottombar;
  r.toggleAttribute('data-rainbow', prefs.rainbow);
  r.toggleAttribute('data-reactive', prefs.reactive);
  r.toggleAttribute('data-rotate', prefs.rainbow && prefs.rotate);
}
