// What the viewer, the editor and the cards read of a file: its bytes, a picture of it, its text, a page of it.
// Pictures and media go straight into src attributes, so they are plain URLs; the rest comes as JSON.
import { ApiError, BASE, call } from '@/api/client';

const q = (params: Record<string, string | number | undefined>) =>
  '?' + new URLSearchParams(Object.entries(params).filter(([, v]) => v !== undefined).map(([k, v]) => [k, String(v)])).toString();

export const rawUrl = (path: string, download = false) => BASE + '/fs/raw' + q({ path, download: download ? 'true' : undefined });
// The file's time in v lets the browser keep a thumbnail for good and still notice a new version.
export const thumbUrl = (path: string, size: 'small' | 'large' = 'small', v?: number) => BASE + '/fs/thumb' + q({ path, size, v });
export const pageUrl = (path: string, page: number, width: number, v?: number) => BASE + '/fs/page' + q({ path, page, width, v });

export interface TextFile {
  path: string;
  size: number;
  mtime: string;
  encoding: string;
  eol?: string;
  text: string;
  truncated?: boolean;
  etag?: string;
}

// Texts are asked for often while the cursor walks a folder, so the last few stay at hand.
const texts = new Map<string, Promise<TextFile>>();
export function fsText(path: string, max?: number, fresh = false): Promise<TextFile> {
  const key = `${path}\n${max ?? ''}`;
  if (!fresh && texts.has(key)) return texts.get(key)!;
  const got = call<TextFile>('/fs/text' + q({ path, max })).then((r) => ({ ...r.body!, etag: r.etag }));
  got.catch(() => texts.delete(key));
  texts.set(key, got);
  if (texts.size > 40) texts.delete(texts.keys().next().value!);
  return got;
}
export const forgetText = (path: string) => {
  for (const key of [...texts.keys()]) if (key.startsWith(path + '\n')) texts.delete(key);
};

export interface HexBytes { size: number; offset: number; bytes: Uint8Array }
export async function fsHex(path: string, offset = 0, length = 512): Promise<HexBytes> {
  const r = (await call<{ size: number; offset: number; data: string }>('/fs/hex' + q({ path, offset, length }))).body!;
  return { size: r.size, offset: r.offset, bytes: Uint8Array.from(atob(r.data), (c) => c.charCodeAt(0)) };
}

// The first page answers with the page count, so one request tells the pager how many pages follow.
export interface FirstPage { url: string; count: number; width: number; height: number }
export async function fsFirstPage(path: string, width: number, v?: number): Promise<FirstPage> {
  const res = await fetch(pageUrl(path, 1, width, v));
  if (!res.ok) throw new ApiError(res.status);
  const url = URL.createObjectURL(await res.blob());
  const img = new Image();
  img.src = url;
  // onload rather than decode(), which waits for a frame and so for a page clock the tests may have stopped.
  await new Promise((done) => { img.onload = img.onerror = done; });
  return { url, count: Math.max(1, Number(res.headers.get('X-Page-Count')) || 1), width: img.naturalWidth, height: img.naturalHeight };
}

export interface ImageInfo { format?: string; label?: string; boot?: string; used?: number; parts?: { label: string; size: number }[] }
export interface Stat { path: string; size: number; mtime: string; image?: ImageInfo }
export const fsStat = async (path: string) => (await call<Stat>('/fs/stat' + q({ path }))).body!;

export async function saveText(path: string, text: string, how: { encoding?: string; eol?: string; etag?: string }) {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (how.etag) headers['If-Match'] = how.etag;
  const r = await call<{ size: number; mtime: string }>('/fs/file' + q({ path }), {
    method: 'PUT', headers, body: JSON.stringify({ text, encoding: how.encoding, eol: how.eol }),
  });
  forgetText(path);
  return { ...r.body, etag: r.etag };
}

export const touch = (path: string) => call('/fs/touch', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ path }) });

// Which office suite the server works with; "none" keeps the built-in previews.
export type OfficeKind = 'none' | 'onlyoffice' | 'eurooffice' | 'collabora';
export interface OfficeSetup { kind: OfficeKind; url?: string }
let officeSetup: Promise<OfficeSetup> | null = null;
export function office(): Promise<OfficeSetup> {
  officeSetup ??= call<{ features?: { office?: OfficeSetup } }>('/system')
    .then((r) => r.body?.features?.office ?? { kind: 'none' as const })
    .catch(() => ({ kind: 'none' as const }));
  return officeSetup;
}

// What an office suite needs to open one file: for OnlyOffice and Euro-Office the signed editor config, for
// Collabora the WOPI address and its access token.
export interface OfficeConfig {
  kind: Exclude<OfficeKind, 'none'>;
  url: string;
  config?: Record<string, unknown>;
  token?: string;
  ttl?: number;
}
// A preview asks for view mode, as the server signs the config and the browser cannot change it afterwards.
export const officeConfig = async (path: string, mode: 'edit' | 'view') => (await call<OfficeConfig>('/office/config' + q({ path, mode }))).body!;
