// Names, sizes and folders as the transfer windows need them.
import { ops } from '@/api/ops';
import type { Entry } from '@/files/entries';
import { join, split } from '@/files/paths';
import { placeName } from '@/features/jobs/label';

export { placeName };
export const sizeOf = (k: Pick<Entry, 'size'>) => k.size || 0;
export const pathOf = (k: Pick<Entry, 'name' | 'at'>, from: string) => join(k.at || from, k.name);
// The part below /mnt/user or /mnt/diskN, which is the same folder seen through the other kind of share.
export const shareRel = (path: string) => path.replace(/^\/mnt\/[^/]+\/?/, '');

// A folder named on a disk share may not be there yet; the copy goes into it once it is. A level that exists answers
// with a conflict, which is fine.
export async function ensureDir(path: string) {
  const parts = split(path);
  for (let k = 3; k <= parts.length; k++) await ops.mkdir('/' + parts.slice(0, k).join('/')).catch(() => {});
}
