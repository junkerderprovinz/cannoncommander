// Keys of your own: a settings tile and the F1 window let every action take the key you press, on top of a layout to
// start from. Both are prefs, so they follow the person to every browser.
import './keys.css';
import { watch } from 'vue';
import { action, actions } from '@/core/actions';
import { keymap } from '@/core/keymap';
import { prefs } from '@/core/prefs';
import { setTipSource } from '@/core/tips';
import { toast } from '@/core/toast';
import { settingsTab } from '@/core/views';
import { win } from '@/core/windows';
import { language, t, tl } from '@/i18n';
import { type KeyMap, keyLabel, type Preset } from './keys';
import KeysTab from './KeysTab.vue';
import KeysWin from './KeysWin.vue';
import { applyKeys, commit, declaredKeys, startKeys } from './state';

export { commit, layout, layoutKeys, useLayout } from './state';

function open() {
  win({
    title: t('keys.title'), wide: true, cls: 'kb-win', body: KeysWin,
    props: {
      apply: (map: KeyMap, which: Preset) => {
        commit(map, which);
        toast(prefs.keys ? t('keys.ownNow') : t('keys.asShipped'));
      },
    },
  });
}

export default function setup() {
  settingsTab({ id: 'keys', label: tl('keys.title'), g: 'IconKeyboard', order: 45, component: KeysTab });
  queueMicrotask(() => {
    startKeys();
    watch(language, () => applyKeys({ ...keymap }));
    action('shortcuts', { ...actions.shortcuts!, label: () => t('keys.title'), g: 'IconKeyboard', run: open });
  });
  // A tooltip names its key in brackets, and with a key of your own it names yours.
  setTipSource((el) => {
    const run = el.dataset.run, tip = el.dataset.tip || '', own = run && declaredKeys()[run], was = own && keyLabel(own), now = run && actions[run]?.key;
    if (!was || was === now) return tip;
    return now ? tip.split(was).join(now) : tip.replace(` (${was})`, '');
  });
}
