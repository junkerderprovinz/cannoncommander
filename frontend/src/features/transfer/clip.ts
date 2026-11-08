// The app's own clipboard, which both builds have. It holds names in one folder, or full paths for rows from several,
// as a search or the flat view shows them.
import { type Entry, toEntry } from '@/files/entries';
import { join, parent, split } from '@/files/paths';
import { useClipboard } from '@/stores/clipboard';
import { useListing } from '@/stores/listing';

// keep puts rows on the app's clipboard and returns their full paths.
export function keep(cut: boolean, from: string, rows: Pick<Entry, 'name' | 'at'>[]): string[] {
  const paths = rows.map((k) => join(k.at || from, k.name));
  useClipboard().put(cut, from, rows.map((k) => k.name), rows.some((k) => k.at && k.at !== from) ? paths : undefined);
  return paths;
}

// The rows of files from anywhere, each with the folder it lies in, as a search result carries them.
export async function entriesOf(files: string[]): Promise<Entry[]> {
  const byFolder = new Map<string, Set<string>>();
  for (const f of files) {
    const dir = parent(f);
    if (!byFolder.has(dir)) byFolder.set(dir, new Set());
    byFolder.get(dir)!.add(split(f).at(-1)!);
  }
  const listing = useListing(), out: Entry[] = [];
  for (const [dir, names] of byFolder) {
    const got = await listing.load(dir);
    out.push(...got.items.filter((k) => names.has(k.name)).map((k) => ({ ...toEntry(k), at: dir })));
  }
  return out;
}
