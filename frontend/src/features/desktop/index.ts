// What only the desktop app does, where CannonCommander stands in for the computer's own file manager: files open in
// their programs.
import './desktop.css';
import { watchEffect } from 'vue';
import { action, actions } from '@/core/actions';
import { host } from '@/core/host';
import { join } from '@/files/paths';
import { tl } from '@/i18n';
import { usePanels } from '@/stores/panels';
import { launch } from './launch';

export default function setup() {
  const F = usePanels();
  watchEffect(() => {
    if (!host.desktop) {
      delete actions.launch;
      return;
    }
    action('launch', {
      label: tl('desktop.launch'), g: 'IconExternalLink', needs: 'sel', when: () => F.sel().some((k) => !k.dir),
      run: (path?: string) => {
        const k = F.sel().find((e) => !e.dir);
        const target = path ?? (k && join(k.at || F.T().path, k.name));
        if (target) return launch(target);
      },
    });
  });
}
