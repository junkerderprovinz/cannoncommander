// A file without an extension, or with one no format claims, is text when the server reads its start as text, which
// is how Kate and the file command tell.
import { call, q } from '@/api/client';
import { ext, FORMAT_ID, kindOf } from '@/core/glyphs';

export const nameTellsNothing = (k: { name: string; dir?: boolean }) => !k.dir && !FORMAT_ID[ext(k.name)] && kindOf(k) === 'file';

const seen = new Map<string, Promise<boolean>>();
export function textInside(path: string, k: { size?: number; time?: number }): Promise<boolean> {
  const key = `${path}\n${k.size}\n${k.time}`;
  let got = seen.get(key);
  if (!got) {
    got = call('/fs/text' + q({ path, max: '4096' })).then(() => true, () => false);
    seen.set(key, got);
    if (seen.size > 200) seen.delete(seen.keys().next().value!);
  }
  return got;
}
