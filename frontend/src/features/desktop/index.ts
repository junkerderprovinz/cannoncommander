// What only the desktop app does, where CannonCommander stands in for the computer's own file manager: files open in
// their programs, and folders handed over from outside open in a tab.
import './desktop.css';
import { watch, watchEffect } from 'vue';
import { on } from '@/api/sse';
import { action, actions } from '@/core/actions';
import { host } from '@/core/host';
import { join } from '@/files/paths';
import { tl } from '@/i18n';
import { usePanels } from '@/stores/panels';
import { launch } from './launch';
import { hostKnown, type Opening, openHandedOver, takeStartFolders } from './open';

export default function setup() {
  const F = usePanels();
  watchEffect(() => {
    if (!host.desktop) {
      delete actions.launch;
      return;
    }
    action('launch', {
      label: tl('desktop.launch'), g: 'IconExternalLink', when: () => F.sel().some((k) => !k.dir),
      run: (path?: string) => {
        const k = F.sel().find((e) => !e.dir);
        const target = path ?? (k && join(k.at || F.T().path, k.name));
        return target ? launch(target) : undefined;
      },
    });
  });

  on('desktop.open', (e) => { void openHandedOver((e.data as Opening[] | undefined) ?? []); });
  const start = takeStartFolders();
  if (start.length && hostKnown()) void openHandedOver(start);
  else if (start.length) {
    const stop = watch(hostKnown, () => {
      stop();
      void openHandedOver(start);
    });
  }
}
