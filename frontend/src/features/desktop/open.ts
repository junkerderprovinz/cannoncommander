// Folders handed over from outside, by the command line, a second start of the app, Win+E or another program that
// shows a file in its folder. Each opens in a new tab of the active panel, as Explorer opens a new window for it.
import { windowsPaths } from '@/core/host';
import { prefs } from '@/core/prefs';
import { go as goView, view } from '@/core/views';
import { typedPath } from '@/files/paths';
import { useListing } from '@/stores/listing';
import { newTab, usePanels } from '@/stores/panels';
import { useUser } from '@/stores/user';

// A folder as the computer writes it, with the file to put the cursor on. Without a folder it is the home folder.
export interface Opening { folder?: string; select?: string }

export async function openHandedOver(list: Opening[]) {
  const F = usePanels(), listing = useListing(), home = useUser().me.home || '/';
  if (view.id !== 'files') goView('files');
  for (const o of list) {
    const path = o.folder ? typedPath(o.folder, { windows: windowsPaths(), home }) : home;
    const p = F.P();
    p.tabs.push(newTab(path));
    p.cur = p.tabs.length - 1;
    if (!o.select) continue;
    await listing.load(path).catch(() => null);
    const at = F.entries().findIndex((k) => k.name === o.select);
    if (at < 0) continue;
    F.T().cursor = at;
    if (prefs.clickSelect) F.T().marked = new Set([o.select]);
  }
}

// The folders the window started with stand in its address once, and leave it again so a reload does not open them twice.
export function takeStartFolders(): Opening[] {
  const params = new URLSearchParams(location.search);
  const raw = params.get('open');
  if (!raw) return [];
  params.delete('open');
  const rest = params.toString();
  history.replaceState(history.state, '', location.pathname + (rest ? '?' + rest : '') + location.hash);
  try {
    const list = JSON.parse(raw);
    return Array.isArray(list) ? list : [];
  } catch {
    return [];
  }
}
