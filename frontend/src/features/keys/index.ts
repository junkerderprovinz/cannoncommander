// Keys of your own: the F1 window lets every action take the key you press, with a Total Commander layout to start
// from. The choice is a pref, so it follows the person to every browser.
import './keys.css';
import { action, actions } from '@/core/actions';
import { keymap } from '@/core/keymap';
import { prefs, setPref } from '@/core/prefs';
import { setTipSource } from '@/core/tips';
import { toast } from '@/core/toast';
import { win } from '@/core/windows';
import { t } from '@/i18n';
import { type KeyMap, keyLabel, mainCombo } from './keys';
import KeysWin from './KeysWin.vue';

interface Saved { map: KeyMap; preset: 'cc' | 'tc' }
let defaults: KeyMap = {};
let defaultLabel: Record<string, string | undefined> = {};
const saved = () => {
  const s = prefs.keys as Saved | null | undefined;
  return s && typeof s.map === 'object' ? s : null;
};

// The label in menus, tooltips and the command search follows the first key the action has.
function apply(map: KeyMap) {
  for (const c of Object.keys(keymap)) delete keymap[c];
  Object.assign(keymap, map);
  for (const a of Object.values(actions)) {
    const main = mainCombo(map, a.id, defaultLabel[a.id]);
    if (main) a.key = keyLabel(main);
    else delete a.key;
  }
}

function open() {
  win({
    title: t('keys.title'), wide: true, cls: 'kb-win', body: KeysWin,
    props: {
      current: { ...keymap }, defaults, defaultLabel, which: saved()?.preset ?? 'cc',
      apply: (map: KeyMap, which: 'cc' | 'tc') => {
        apply(map);
        const own = JSON.stringify(map) !== JSON.stringify(defaults);
        setPref('keys', own ? { map, preset: which } : null);
        toast(own ? t('keys.ownNow') : t('keys.asShipped'));
      },
    },
  });
}

export default function setup() {
  // Taken after every feature has added its keys, so the defaults are the whole set.
  queueMicrotask(() => {
    defaults = { ...keymap };
    defaultLabel = Object.fromEntries(Object.values(actions).map((a) => [a.id, a.key]));
    const s = saved();
    if (s) apply(s.map);
    action('shortcuts', { ...actions.shortcuts!, label: () => t('keys.title'), g: 'IconKeyboard', run: open });
  });
  // A tooltip names its key in brackets, and with a key of your own it names yours.
  setTipSource((el) => {
    const tip = el.dataset.tip || '', id = el.dataset.run, was = id && defaultLabel[id], now = id && actions[id]?.key;
    if (!was || was === now) return tip;
    return now ? tip.split(was).join(now) : tip.replace(` (${was})`, '');
  });
}
