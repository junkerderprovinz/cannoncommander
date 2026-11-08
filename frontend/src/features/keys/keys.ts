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

export type Verdict = { ok: true } | { ok: false; reason: 'fixed' | 'letter' } | { ok: false; reason: 'taken'; other: string };

// Whether `combo` may go to `action`. A key another action has asks first, and pressing it again (`confirmed`) takes it.
export function check(map: KeyMap, combo: string, action: string, confirmed: boolean): Verdict {
  if (FIXED.includes(combo)) return { ok: false, reason: 'fixed' };
  // Single letters and digits belong to typing to jump in the list.
  if (/^[\p{L}\d]$/u.test(combo)) return { ok: false, reason: 'letter' };
  const other = map[combo];
  if (other && other !== action && !confirmed) return { ok: false, reason: 'taken', other };
  return { ok: true };
}

export function assign(map: KeyMap, combo: string, action: string): KeyMap {
  const next = { ...map };
  for (const c of combosOf(next, action)) delete next[c];
  next[combo] = action;
  return next;
}

export function reset(map: KeyMap, defaults: KeyMap, action: string): KeyMap {
  const next = { ...map };
  for (const c of combosOf(next, action)) delete next[c];
  for (const c of combosOf(defaults, action)) if (!next[c]) next[c] = action;
  return next;
}

export const changed = (map: KeyMap, defaults: KeyMap, action: string) => combosOf(defaults, action).sort().join() !== combosOf(map, action).sort().join();
