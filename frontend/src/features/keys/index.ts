// Keys of your own: the F1 window lets every action take the key you press, with a Total Commander layout to start
// from. The choice is a pref, so it follows the person to every browser.
import './keys.css';
import { watch } from 'vue';
import { action, actions } from '@/core/actions';
import { keymap } from '@/core/keymap';
import { prefs, setPref } from '@/core/prefs';
import { setTipSource } from '@/core/tips';
import { toast } from '@/core/toast';
import { win } from '@/core/windows';
import { language, t } from '@/i18n';
import { type KeyMap, keyLabel, mainCombo, type Preset, preset } from './keys';
import KeysWin from './KeysWin.vue';

interface Saved { map: KeyMap; preset: Preset }
let defaults: KeyMap = {};
// The key each action names for itself, which also covers keys the panels handle outside the map, such as Backspace.
let declared: Record<string, string | undefined> = {};
const saved = () => {
  const s = prefs.keys as Saved | null | undefined;
  return s && typeof s.map === 'object' ? s : null;
};

// The label in menus, tooltips and the command search follows the first key the action has, in the words of the
// interface's language.
function apply(map: KeyMap) {
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

function open() {
  win({
    title: t('keys.title'), wide: true, cls: 'kb-win', body: KeysWin,
    props: {
      current: { ...keymap }, defaults, declared, which: saved()?.preset ?? 'cc',
      apply: (map: KeyMap, which: Preset) => {
        apply(map);
        const own = JSON.stringify(map) !== JSON.stringify(defaults);
        setPref('keys', own ? { map, preset: which } : null);
        toast(own ? t('keys.ownNow') : t('keys.asShipped'));
      },
    },
  });
}

// The layout a choice of style brings, as the F1 window would pick it.
export function choosePreset(id: Preset) {
  const map = preset(id, defaults, (a) => !!actions[a]);
  // Saved first, since a menu shows a key of a layout's own only once the layout is saved.
  setPref('keys', id === 'cc' ? null : { map, preset: id });
  apply(map);
}

export default function setup() {
  // Taken after every feature has added its keys, so the defaults are the whole set.
  queueMicrotask(() => {
    defaults = { ...keymap };
    declared = Object.fromEntries(Object.values(actions).map((a) => [a.id, a.key]));
    apply(saved()?.map ?? defaults);
    watch(language, () => apply({ ...keymap }));
    action('shortcuts', { ...actions.shortcuts!, label: () => t('keys.title'), g: 'IconKeyboard', run: open });
  });
  // A tooltip names its key in brackets, and with a key of your own it names yours.
  setTipSource((el) => {
    const tip = el.dataset.tip || '', own = el.dataset.run && declared[el.dataset.run], was = own && keyLabel(own), now = el.dataset.run && actions[el.dataset.run]?.key;
    if (!was || was === now) return tip;
    return now ? tip.split(was).join(now) : tip.replace(` (${was})`, '');
  });
}
