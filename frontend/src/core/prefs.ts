// Settings per user. The copy in localStorage paints the first frame right; the server's copy follows and wins.
import { ACCENTS, applyAccent, applyDisco, applyMotion, applyRainbow, applyShape, BOTTOM_BAR_AXIS, DEFAULT_ACCENT, DEFAULT_LABEL_MODE, labelModeChanged, RAINBOW, setLabelMode } from '@glimstone/core';
import { reactive } from 'vue';

export const PREFS_KEY = 'cannoncommander-prefs';
// An empty language has not been picked yet, and the browser's language stands for it until someone picks one.

export const DEFAULT_PREFS = {
  theme: 'system', accent: DEFAULT_ACCENT, shape: 'round', motion: 'subtle', quietToasts: false,
  labelsButtons: DEFAULT_LABEL_MODE, labelsSidebar: DEFAULT_LABEL_MODE, labelsTabs: DEFAULT_LABEL_MODE, labelsBottombar: DEFAULT_LABEL_MODE,
  rainbow: false, reactive: false, rotate: false, rotateSeed: 0, disco: false, tabOrder: [] as string[],
  accentPresets: ACCENTS.map((a) => a.hex), palette: [...RAINBOW], jobMode: 'serial', newsSeen: [] as string[], stayDays: 30,
  speedCap: 0, parityWait: true, notifyDone: false, notifyUnraid: false, notifyAfter: 60,
  columns: ['name', 'size', 'modified'], colWidths: {} as Record<string, number>,
  density: 'medium', showHidden: false, showSystem: false, showExt: true, syncBrowse: false, folderSizes: true, foldersFirst: true, liveRefresh: true, dblclick: 'open', clickSelect: false, desktopStyle: '', fkeys: false, side: true, dual: true,
  quickview: false, sidePane: false, confirmDelete: true, trash: true, trashDays: 30, language: '', dateFormat: 'short', startFolder: '',
  folderViews: {} as Record<string, string>, folderGroups: {} as Record<string, string>, folderTiles: {} as Record<string, number>,
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
  // Without a theme of its own the page is left to GlimStone's stylesheet, which follows the system as it changes.
  if (prefs.theme === 'system') delete r.dataset.theme;
  else r.dataset.theme = prefs.theme;
  applyAccent(prefs.accent);
  applyShape(prefs.shape);
  applyMotion(prefs.motion);
  r.dataset.density = prefs.density;
  setLabelMode('buttons', prefs.labelsButtons);
  setLabelMode('sidebar', prefs.labelsSidebar);
  setLabelMode('tabs', prefs.labelsTabs);
  setLabelMode(BOTTOM_BAR_AXIS, prefs.labelsBottombar);
  labelModeChanged();
  // Reactive is a way of showing the rainbow, so it does nothing without it, and rotation shifts the palette instead of spinning it.
  applyRainbow({ on: prefs.rainbow, reactive: prefs.reactive, rotate: prefs.rotate, seed: prefs.rotateSeed, palette: prefs.palette });
  r.toggleAttribute('data-reactive', prefs.rainbow && prefs.reactive);
  applyDisco(prefs.disco);
}
