// The keys in force: the layout chosen, own keys on top of it, and the labels menus and tooltips show for them.
import { actions } from '@/core/actions';
import { host } from '@/core/host';
import { keymap } from '@/core/keymap';
import { prefs, setPref } from '@/core/prefs';
import { type KeyMap, keyLabel, mainCombo, type OwnKeys, ownKeys, type Preset, preset, withOwn } from './keys';

interface Saved { preset: Preset; own: OwnKeys }
let defaults: KeyMap = {};
// The key each action names for itself, which also covers keys the panels handle outside the map, such as Backspace.
let declared: Record<string, string | undefined> = {};
export const shippedKeys = () => defaults;
export const declaredKeys = () => declared;

export const layoutKeys = (which: Preset) => preset(which, defaults, (a) => !!actions[a]);
// Explorer's layout is for the desktop app, which stands in for Explorer.
export const layoutItems = (): [Preset, string][] => [['cc', 'CannonCommander'], ['tc', 'Total Commander'], ...(host.desktop ? [['explorer', 'Explorer'] as [Preset, string]] : [])];

// Older settings hold the whole map, which turns into what it changes in its layout.
function saved(): Saved | null {
  const s = prefs.keys as { preset?: Preset; own?: OwnKeys; map?: KeyMap } | null | undefined;
  if (!s || typeof s !== 'object') return null;
  const which = s.preset ?? 'cc';
  if (s.own && typeof s.own === 'object') return { preset: which, own: s.own };
  if (s.map && typeof s.map === 'object') return { preset: which, own: ownKeys(layoutKeys(which), s.map) };
  return null;
}
export const layout = (): Preset => saved()?.preset ?? 'cc';
export const hasOwnKeys = () => Object.keys(saved()?.own ?? {}).length > 0;

// The label in menus, tooltips and the command search follows the first key the action has, in the words of the
// interface's language.
export function applyKeys(map: KeyMap) {
  for (const c of Object.keys(keymap)) delete keymap[c];
  Object.assign(keymap, map);
  for (const a of Object.values(actions)) {
    const own = declared[a.id];
    // As shipped, an action that names no key of its own shows none, as the prototype has it.
    const main = !own && !saved() ? undefined : mainCombo(map, a.id, own) ?? (own && !(own in map) && !(own in defaults) ? own : undefined);
    if (main) a.key = keyLabel(main);
    else delete a.key;
  }
}

// Taken once every feature has added its keys, so the defaults are the whole set.
export function startKeys() {
  defaults = { ...keymap };
  declared = Object.fromEntries(Object.values(actions).map((a) => [a.id, a.key]));
  const s = saved();
  applyKeys(s ? withOwn(layoutKeys(s.preset), s.own) : defaults);
}

function store(map: KeyMap, which: Preset) {
  const own = ownKeys(layoutKeys(which), map);
  setPref('keys', which === 'cc' && !Object.keys(own).length ? null : { preset: which, own });
  applyKeys(map);
}

// A layout comes with its way of selecting: Explorer's selects with a click, the commanders' only move the cursor.
export function useLayout(which: Preset) {
  store(withOwn(layoutKeys(which), saved()?.own ?? {}), which);
  setPref('clickSelect', which === 'explorer');
}

export function commit(map: KeyMap, which: Preset) {
  const chosen = which !== layout();
  store(map, which);
  if (chosen) setPref('clickSelect', which === 'explorer');
}
