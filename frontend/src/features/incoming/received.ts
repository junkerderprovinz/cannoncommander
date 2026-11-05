// What sw.js kept from a share: the files, or for a shared link or text only that, written into a small text file.
export const BOX = 'cannoncommander-shared';

interface Note { titel?: string; text?: string; adresse?: string }

// A name the server takes, from the title of what was shared.
export const noteName = (title: string | undefined, fallback: string) =>
  (title?.replace(/[\\/:*?"<>|\u0000-\u001f]+/g, ' ').replace(/\s+/g, ' ').trim().slice(0, 80) || fallback) + '.txt';

export function noteFile(note: Note, fallback: string, at: number): File {
  const lines = [note.titel, note.text, note.adresse].filter((x): x is string => !!x?.trim());
  return new File([lines.join('\n') + '\n'], noteName(note.titel, fallback), { type: 'text/plain', lastModified: at });
}

// Takes everything out of the box and empties it, so a reload does not offer the same files again.
export async function takeReceived(fallback: string, at: number): Promise<File[]> {
  if (!('caches' in globalThis)) return [];
  const box = await caches.open(BOX);
  const files: File[] = [];
  let note: Note | null = null;
  // The keys end in the place each file had in the share, which the box does not keep in order.
  const keys = [...(await box.keys())].sort((a, b) => Number(a.url.split('/').pop()) - Number(b.url.split('/').pop()));
  for (const req of keys) {
    const res = await box.match(req);
    if (!res) continue;
    if (req.url.endsWith('/notiz')) { note = await res.json(); continue; }
    const name = decodeURIComponent(res.headers.get('X-Name') || '') || req.url.split('/').pop()!;
    files.push(new File([await res.blob()], name, { type: res.headers.get('Content-Type') || '', lastModified: Number(res.headers.get('X-Modified')) || at }));
  }
  await caches.delete(BOX);
  if (!files.length && note) files.push(noteFile(note, fallback, at));
  return files;
}
