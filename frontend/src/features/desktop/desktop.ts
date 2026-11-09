// What only the desktop app does, where CannonCommander stands in for the computer's own file manager: files open in
// their programs, folders handed over from outside open in a tab, the first start asks how the app should handle,
// and the computer may send all its folders here.
import './desktop.css';
import { on } from '@/api/sse';
import { action, actions } from '@/core/actions';
import { prefs } from '@/core/prefs';
import { addOpenMenuItems } from '@/files/menus';
import { inArchive, join } from '@/files/paths';
import { tl } from '@/i18n';
import { usePanels } from '@/stores/panels';
import { followFileManager } from './filemanager';
import { launch } from './launch';
import { type Opening, openHandedOver, takeStartFolders } from './open';
import { askStyle } from './style';

// The window's own menu offers Back, Forward and Reload in the language of the system's web view, which mean nothing
// here. Fields keep theirs for cut, copy and paste.
function quietMenu(e: MouseEvent) {
  if (!(e.target instanceof Element) || !e.target.closest('input, textarea, [contenteditable]:not([contenteditable="false"])')) e.preventDefault();
}

export default function setup() {
  const F = usePanels();
  document.addEventListener('contextmenu', quietMenu);
  action('launch', {
    label: tl('desktop.launch'), g: 'IconExternalLink', when: () => F.sel().some((k) => !k.dir),
    run: (path?: string) => {
      const k = F.sel().find((e) => !e.dir);
      const target = path ?? (k && join(k.at || F.T().path, k.name));
      return target ? launch(target) : undefined;
    },
  });
  // A download only copies into the window's own download folder and an upload copies over HTTP, where every file
  // is on this computer already.
  for (const id of ['download', 'upload']) delete actions[id];
  void followFileManager();
  if (!prefs.desktopStyle) askStyle();
  // Öffnen hands the file to its program, beside Öffnen mit, as in Explorer's menu.
  addOpenMenuItems((sel, path) => (sel.some((k) => !k.dir && !k.up) && !inArchive(path) ? ['launch'] : []));
  on('desktop.open', (e) => { void openHandedOver((e.data as Opening[] | undefined) ?? []); });
  const start = takeStartFolders();
  if (start.length) void openHandedOver(start);
}
