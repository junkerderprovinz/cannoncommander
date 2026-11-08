// Files shared with the other programs on the computer in the desktop app: its clipboard, files dropped on the window
// and files dragged out of it. Paths on disk only travel between the window and the server, which keeps to the
// sources, so a file outside all of them is left out and everything else takes the panels' own way with its undo.
import { ApiError, call, json } from '@/api/client';
import { toast } from '@/core/toast';
import type { Entry } from '@/files/entries';
import { parent } from '@/files/paths';
import { plural } from '@/i18n';
import { useClipboard } from '@/stores/clipboard';
import { usePanels } from '@/stores/panels';
import { entriesOf, keep } from './clip';

export interface Outside { files: string[]; cut: boolean; skipped: number }
type Dropped = Outside & { sameVolume: boolean };

const warn = (e: unknown) => toast(e instanceof ApiError ? e.message : String(e), 'warn');
const signature = (o: Outside | null) => (o ? (o.cut ? 'cut\n' : 'copy\n') + o.files.join('\n') : '');

// The files on the computer's clipboard, or null where the app does not reach it.
export async function readClipboard(): Promise<Outside | null> {
  const clip = useClipboard();
  try {
    const got = (await call<Outside>('/clipboard')).body!;
    clip.system = got.files.length;
    return got;
  } catch {
    clip.system = null;
    return null;
  }
}

// The window hears a burst of focus events as it comes to the front, and one look at the clipboard answers them all.
let looking: ReturnType<typeof setTimeout> | undefined;
export function lookSoon() {
  clearTimeout(looking);
  looking = setTimeout(() => { void readClipboard(); }, 500);
}

// Names cut or copied in a panel go onto the computer's clipboard as files. Those it cannot take, such as files inside
// an archive, stay in the app's own clipboard until another program puts something newer on the computer's.
export async function share(cut: boolean, from: string, rows: Pick<Entry, 'name' | 'at'>[]) {
  const clip = useClipboard();
  const paths = keep(cut, from, rows);
  try {
    await call('/clipboard', json('PUT', { paths, cut }));
    clip.clear();
    clip.system = rows.length;
  } catch {
    clip.under = signature(await readClipboard());
  }
}

// What a paste takes from the computer's clipboard, which may be no files at all, or null when the app's own
// clipboard goes first or there is no way to the computer's.
export async function clipboardWins(): Promise<Outside | null> {
  const clip = useClipboard();
  const got = await readClipboard();
  if (!got || (clip.clip && signature(got) === clip.under)) return null;
  clip.clear();
  return got;
}

async function bring(kind: 'copy' | 'move', files: string[], skipped: number, dest: string) {
  if (skipped) toast(plural(skipped, 'transfer.leftOut'), 'warn');
  if (!files.length) return undefined;
  try {
    return await usePanels().transfer(kind, await entriesOf(files), dest, parent(files[0]!));
  } catch (e) {
    warn(e);
    return undefined;
  }
}

// Files cut in another program move, and the clipboard is emptied as that program expects after a move.
export async function pasteOutside(got: Outside, dest: string) {
  const job = await bring(got.cut ? 'move' : 'copy', got.files, got.skipped, dest);
  if (!got.cut || !job) return;
  useClipboard().system = 0;
  await call('/clipboard/moved', json('POST', { files: got.files })).catch(() => {});
}

// Files dropped from another program go the way Explorer takes them: Shift moves and Ctrl copies, and otherwise they
// move on the same drive and are copied from another one.
export async function dropIn(paths: string[], dest: string, how: 'copy' | 'move' | 'auto') {
  let got: Dropped;
  try {
    got = (await call<Dropped>('/dragdrop/in', json('POST', { paths, to: dest }))).body!;
  } catch (e) {
    return warn(e);
  }
  await bring(how === 'auto' ? (got.sameVolume ? 'move' : 'copy') : how, got.files, got.skipped, dest);
}

// The drag goes on in the system's own drag loop and the answer comes when the files are dropped somewhere.
export async function dragOut(paths: string[]) {
  try {
    await call('/dragdrop/out', json('POST', { paths }));
  } catch (e) {
    warn(e);
  }
}
