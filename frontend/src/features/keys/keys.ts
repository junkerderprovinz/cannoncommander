// Keys of your own: which combo runs which action, how a combo reads, the Total Commander layout to start from and the
// rules a new key has to pass. Pure, so the window and the tests share it.
import { t } from '@/i18n';

export type KeyMap = Record<string, string>;

const NAMED = ['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Delete', 'Insert', 'Escape', ' ', 'Home', 'End', 'PageUp', 'PageDown', 'Backspace', 'Enter', 'Tab'];
const MODS: Record<string, string> = { Ctrl: 'ctrl', Shift: 'shift', Alt: 'alt' };

// A combo ending in the plus key itself, such as Ctrl++, keeps that plus as its last part.
export function keyLabel(combo: string): string {
  const parts = combo.endsWith('++') ? [...combo.slice(0, -2).split('+'), '+'] : combo.split('+');
  return parts.map((p, i) => {
    if (i < parts.length - 1) return MODS[p] ? t(`keys.mod.${MODS[p]}`) : p;
    if (NAMED.includes(p)) return t(`keys.named.${p === ' ' ? 'Space' : p}`);
    return p.length === 1 ? p.toUpperCase() : p;
  }).join('+');
}

export const combosOf = (map: KeyMap, id: string) => Object.keys(map).filter((c) => map[c] === id);

// The key a menu shows for an action: the one it came with, if it still has it, else its first.
export function mainCombo(map: KeyMap, id: string, declared?: string) {
  const own = combosOf(map, id);
  return own.find((c) => c === declared) ?? own[0];
}

// Total Commander's own keys for the actions both programs share.
export const TOTAL_COMMANDER: KeyMap = { 'Alt+F5': 'pack', 'Alt+F9': 'unpack', 'Ctrl+b': 'flat', 'Shift+F6': 'rename', F2: 'refresh', 'Shift+F2': 'compareDirs', 'Ctrl+d': 'addBookmark', 'Alt+F8': 'history' };

// Explorer's keys where they differ from a commander's. They come on top, so Ctrl+R still refreshes beside F5, and
// F3 stays with the viewer, as Space does.
export const EXPLORER: KeyMap = { F5: 'refresh', 'Alt+d': 'gotoPath', 'Ctrl+Shift+c': 'copyPath', Backspace: 'back', 'Alt+p': 'sidePane', 'Ctrl+d': 'delete' };

export type Preset = 'cc' | 'tc' | 'explorer';

export function preset(id: Preset, defaults: KeyMap, known: (action: string) => boolean): KeyMap {
  const map = { ...defaults };
  if (id === 'explorer') {
    for (const [combo, act] of Object.entries(EXPLORER)) if (known(act)) map[combo] = act;
    return map;
  }
  if (id !== 'tc') return map;
  for (const [combo, act] of Object.entries(TOTAL_COMMANDER)) {
    if (!known(act)) continue;
    for (const c of combosOf(map, act)) delete map[c];
    map[combo] = act;
  }
  return map;
}

export const FIXED = ['Tab', 'Enter', 'F10'];
export const MODIFIER_KEYS = ['Control', 'Shift', 'Alt', 'Meta', 'AltGraph', 'CapsLock'];

// A browser never hands these to a page, so in a tab of the container they would do nothing. The desktop app's web
// view has no tabs and lets the app have them.
const BROWSER = ['Ctrl+n', 'Ctrl+Shift+n', 'Ctrl+t', 'Ctrl+Shift+t', 'Ctrl+w', 'Ctrl+Shift+w', 'Ctrl+Tab', 'Ctrl+Shift+Tab', 'Ctrl+PageUp', 'Ctrl+PageDown', 'Ctrl+Shift+p'];
const SYSTEM = ['Alt+Tab', 'Alt+Shift+Tab', 'Alt+F4', 'Alt+ ', 'Ctrl+Escape', 'Ctrl+Alt+Delete'];

export type Reserved = 'browser' | 'system';
export function reservedBy(combo: string, desktop: boolean): Reserved | undefined {
  if (SYSTEM.includes(combo)) return 'system';
  return !desktop && BROWSER.includes(combo) ? 'browser' : undefined;
}

export type Verdict = { ok: true } | { ok: false; reason: 'fixed' | 'letter' | Reserved } | { ok: false; reason: 'taken'; other: string };

export function check(map: KeyMap, combo: string, action: string, desktop: boolean): Verdict {
  if (FIXED.includes(combo)) return { ok: false, reason: 'fixed' };
  // Single letters and digits belong to typing to jump in the list.
  if (/^[\p{L}\d]$/u.test(combo)) return { ok: false, reason: 'letter' };
  const reserved = reservedBy(combo, desktop);
  if (reserved) return { ok: false, reason: reserved };
  const other = map[combo];
  if (other && other !== action) return { ok: false, reason: 'taken', other };
  return { ok: true };
}

export function assign(map: KeyMap, combo: string, action: string): KeyMap {
  const next = { ...map };
  for (const c of combosOf(next, action)) delete next[c];
  next[combo] = action;
  return next;
}

// The action takes the key, and the action that had it gets the keys this one had.
export function swap(map: KeyMap, combo: string, action: string): KeyMap {
  const other = map[combo], had = combosOf(map, action);
  const next = assign(map, combo, action);
  if (other) for (const c of had) next[c] = other;
  return next;
}

export function reset(map: KeyMap, base: KeyMap, action: string): KeyMap {
  const next = { ...map };
  for (const c of combosOf(next, action)) delete next[c];
  for (const c of combosOf(base, action)) if (!next[c]) next[c] = action;
  return next;
}

// The layout's keys of an action that another action has taken.
export const heldBack = (map: KeyMap, base: KeyMap, action: string) => combosOf(base, action).filter((c) => map[c] && map[c] !== action);

// A reset that takes the layout's keys back. With swap the action that held them gets the keys this one had instead.
export function restore(map: KeyMap, base: KeyMap, action: string, swapped: boolean): KeyMap {
  const holder = map[heldBack(map, base, action)[0] ?? ''], had = combosOf(map, action);
  const next = { ...map };
  for (const c of had) delete next[c];
  for (const c of combosOf(base, action)) next[c] = action;
  if (swapped && holder) for (const c of had) if (!next[c]) next[c] = holder;
  return next;
}

export const changed = (map: KeyMap, base: KeyMap, action: string) => combosOf(base, action).sort().join() !== combosOf(map, action).sort().join();

// Own keys are kept as what they change in a layout, an empty action for a key taken away, so a layout chosen later
// and keys a newer version ships still reach someone who has keys of their own.
export type OwnKeys = Record<string, string>;

export function ownKeys(base: KeyMap, map: KeyMap): OwnKeys {
  const own: OwnKeys = {};
  for (const c of new Set([...Object.keys(base), ...Object.keys(map)])) if (base[c] !== map[c]) own[c] = map[c] ?? '';
  return own;
}

export function withOwn(base: KeyMap, own: OwnKeys): KeyMap {
  const map = { ...base };
  for (const [c, a] of Object.entries(own)) {
    if (a) map[c] = a;
    else delete map[c];
  }
  return map;
}
