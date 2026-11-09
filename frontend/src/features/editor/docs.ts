// The files open in the editor's tabs: how each was read, so it is written back the same way unless the person
// converts it, and whether it has changed since.
import { ApiError, call } from '@/api/client';
import { host } from '@/core/host';
import { saveText } from '@/features/viewer/content';
import { joinPath } from '@/features/viewer/kinds';

export type Eol = 'lf' | 'crlf' | 'cr';
export const EOLS: Eol[] = ['lf', 'crlf', 'cr'];
export const EOL_NAMES: Record<Eol, string> = { lf: 'LF', crlf: 'CRLF', cr: 'CR' };

// The encodings the server reads and writes, by the names people know them by. ASCII reads as UTF-8, which it is.
export const ENCODINGS: [id: string, name: string][] = [
  ['utf-8', 'UTF-8'], ['utf-8-bom', 'UTF-8 BOM'], ['utf-16le', 'UTF-16 LE'], ['utf-16be', 'UTF-16 BE'],
  ['windows-1252', 'Windows-1252'], ['iso-8859-1', 'ISO-8859-1'], ['iso-8859-15', 'ISO-8859-15'],
  ['windows-1250', 'Windows-1250'], ['iso-8859-2', 'ISO-8859-2'], ['windows-1251', 'Windows-1251'], ['koi8-r', 'KOI8-R'],
];
export const encodingName = (id: string) => ENCODINGS.find(([e]) => e === id)?.[1] ?? id.toUpperCase();

// Past this the editor reads only the start, and only to look at: a browser editor slows down long before 16 MiB of
// text, and the server refuses to hand more to it.
export const READ_START = 1 << 20;

export interface Indent { tabs: boolean; width: number }

export interface Doc {
  id: number;
  name: string;
  folder: string;
  path: string;
  // The text as it was read or last saved, with \n between lines, as the editor holds it.
  saved: string;
  encoding: string;
  eol: Eol;
  // How the file is on disk, so converting it counts as a change until it is saved.
  savedEncoding: string;
  savedEol: Eol;
  etag?: string;
  readOnly: boolean;
  // Only the start of a file too large for the editor was read.
  cut: boolean;
  dirty: boolean;
  lang: string;
  indent: Indent;
}

interface EditFile { text: string; encoding: string; eol?: string; etag: string; writable: boolean }
interface TextStart { text: string; encoding: string; eol?: string; truncated?: boolean }

// The line ending a file mostly has, as the editor turns every one into \n. A file of one line takes the system's.
export function eolOf(text: string, said?: string): Eol {
  if (said === 'lf' || said === 'crlf' || said === 'cr') return said;
  const crlf = text.split('\r\n').length - 1, lf = text.split('\n').length - 1 - crlf, cr = text.split('\r').length - 1 - crlf;
  if (crlf + lf + cr === 0) return host.desktop && host.os === 'windows' ? 'crlf' : 'lf';
  return crlf >= lf && crlf >= cr ? 'crlf' : cr > lf ? 'cr' : 'lf';
}

const plain = (text: string) => text.replace(/\r\n?/g, '\n');
let next = 1;

export interface Read { text: string; encoding: string; eol: Eol; etag?: string; readOnly: boolean; cut: boolean }

// The whole file as the editor needs it, in the encoding asked for or the one the server finds. A file too large for
// the editor comes as its start, to look at.
export async function readFile(path: string, encoding?: string): Promise<Read> {
  const p = new URLSearchParams({ path, ...(encoding ? { encoding } : {}) });
  try {
    const f = (await call<EditFile>('/fs/file?' + p)).body!;
    return { text: plain(f.text), encoding: f.encoding === 'ascii' ? 'utf-8' : f.encoding, eol: eolOf(f.text, f.eol), etag: f.etag, readOnly: !f.writable, cut: false };
  } catch (e) {
    if (!(e instanceof ApiError && e.status === 413)) throw e;
  }
  p.set('max', String(READ_START));
  const f = (await call<TextStart>('/fs/text?' + p)).body!;
  return { text: plain(f.text), encoding: f.encoding === 'ascii' ? 'utf-8' : f.encoding, eol: eolOf(f.text, f.eol), readOnly: true, cut: true };
}

// The language and the indent come from the text once the window has it.
export function makeDoc(name: string, folder: string, r: Read): Doc {
  return {
    id: next++, name, folder, path: joinPath(folder, name), saved: r.text, encoding: r.encoding, eol: r.eol,
    savedEncoding: r.encoding, savedEol: r.eol, etag: r.etag, readOnly: r.readOnly, cut: r.cut, dirty: false, lang: '', indent: { tabs: false, width: 4 },
  };
}

export const newFile = (): Read => ({ text: '', encoding: 'utf-8', eol: eolOf(''), readOnly: false, cut: false });

// Writes the text as the tab says, and only over the file as it was read, so a change made elsewhere meanwhile is
// never lost unseen.
export async function writeDoc(d: Doc, text: string) {
  const got = await saveText(d.path, text, { encoding: d.encoding, eol: d.eol, etag: d.etag });
  Object.assign(d, { etag: got.etag, saved: text, savedEncoding: d.encoding, savedEol: d.eol, dirty: false });
}
