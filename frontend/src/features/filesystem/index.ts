// What the desktop app keeps of the file system it runs on, the way the system's own file manager shows it.
import { watch } from 'vue';
import { action } from '@/core/actions';
import { host } from '@/core/host';
import { prefs, setPref } from '@/core/prefs';
import { tl } from '@/i18n';

export default function setup() {
  // Only Windows has protected system files, and the server says where it runs after the start.
  watch(() => host.os, (os) => {
    if (os !== 'windows') return;
    action('systemFiles', { label: tl('filesystem.systemFiles'), g: 'IconShieldOn', checked: () => !!prefs.showSystem, run: () => setPref('showSystem', !prefs.showSystem) });
  }, { immediate: true });
}
