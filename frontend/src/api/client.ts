// Every call to /api/v1 goes through here, so the rest of the app never sees a URL, a header or the wire format.
import { serverQuery } from '@/files/fields';
import { serverText } from '@/i18n';
import type { Bookmark, FsItem, FsList, FsMeta, FsText, FsTree, Me, Problem, SearchResult, Volume } from './types';

export const BASE = '/api/v1';

export class ApiError extends Error {
  // retryAfter is in seconds, for a request the server wants to see again later.
  constructor(readonly status: number, readonly problem?: Problem, readonly retryAfter?: number) {
    super(serverText(problem?.code, problem?.params) ?? (problem?.detail || (status >= 500 && serverText('broken')) || problem?.title || `HTTP ${status}`));
  }
}

// A 401 outside the sign in means the session ended, and the app shows the sign in again.
let signedOut = () => {};
export const onSignedOut = (fn: () => void) => { signedOut = fn; };

let token = '';
export const setToken = (t: string) => { token = t; };

interface Got<T> { body: T | null; etag?: string }

// When this page last asked the server to change something on disk, so a change the server reports can be told apart
// from one another program made. Its settings, the folders it watches and a test note change nothing there.
export const writes = { at: 0 };

export async function call<T>(path: string, init: RequestInit & { etag?: string } = {}): Promise<Got<T>> {
  if (init.method && init.method !== 'GET' && !/^\/(me|events|jobs\/settings|unraid\/notify)\b/.test(path)) writes.at = Date.now();
  const headers = new Headers(init.headers);
  headers.set('Accept', 'application/json');
  if (token) headers.set('Authorization', `Bearer ${token}`);
  if (init.etag) headers.set('If-None-Match', init.etag);
  if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');
  const res = await fetch(BASE + path, { ...init, headers });
  if (res.status === 304) return { body: null, etag: init.etag };
  if (!res.ok) {
    const problem = res.headers.get('Content-Type')?.includes('json') ? await res.json().catch(() => undefined) : undefined;
    if (res.status === 401 && !path.startsWith('/auth/')) signedOut();
    throw new ApiError(res.status, problem, Number(res.headers.get('Retry-After')) || undefined);
  }
  return { body: res.status === 204 ? null : await res.json(), etag: res.headers.get('ETag') ?? undefined };
}

export const q = (params: Record<string, string>) => '?' + new URLSearchParams(params).toString();
export const json = (method: string, body: unknown): RequestInit => ({ method, body: JSON.stringify(body) });

// Some settings follow the screen, so every browser keeps an id of its own for them.
const DEVICE_KEY = 'cannoncommander-device';
function device(): Record<string, string> {
  try {
    let id = localStorage.getItem(DEVICE_KEY);
    if (!id) {
      // randomUUID needs a secure context, which a server on the home network often lacks.
      id = Array.from(crypto.getRandomValues(new Uint8Array(12)), (b) => b.toString(16).padStart(2, '0')).join('');
      localStorage.setItem(DEVICE_KEY, id);
    }
    return { device: id };
  } catch {
    return {};
  }
}

// What the interface works with: times as milliseconds, nothing optional that it reads on every row.
export interface Item {
  name: string;
  dir: boolean;
  size: number;
  sizeUnknown?: boolean;
  time: number;
  mode: string;
  owner: string;
  group: string;
  hidden?: boolean;
  link?: string;
  items?: number;
  archive?: boolean;
  at?: string;
  btime?: number;
  ino?: number;
  nlink?: number;
  uid?: number;
  gid?: number;
  picture?: string;
}
export interface Listing { path: string; items: Item[]; volume?: Volume; etag?: string }

export const toItem = (i: FsItem): Item => ({
  name: i.name, dir: i.dir, size: i.size, ...(i.sizeUnknown ? { sizeUnknown: true } : {}), time: Date.parse(i.mtime), mode: i.mode, owner: i.owner ?? '', group: i.group ?? '',
  ...(i.hidden ? { hidden: true } : {}), ...(i.link ? { link: i.link } : {}), ...(i.items != null ? { items: i.items } : {}),
  ...(i.archive ? { archive: true } : {}), ...(i.btime ? { btime: Date.parse(i.btime) } : {}),
  ...(i.ino != null ? { ino: i.ino } : {}), ...(i.nlink != null ? { nlink: i.nlink } : {}),
  ...(i.uid != null ? { uid: i.uid } : {}), ...(i.gid != null ? { gid: i.gid } : {}), ...(i.picture ? { picture: i.picture } : {}),
});

export const api = {
  // Resolves to null when the copy behind `etag` is still current.
  async fsList(path: string, etag?: string, signal?: AbortSignal): Promise<Listing | null> {
    const got = await call<FsList>('/fs/list' + q({ path }), { etag, signal });
    return got.body && { path: got.body.path, items: got.body.items.map(toItem), volume: got.body.volume, etag: got.etag };
  },
  async fsTree(path: string): Promise<FsTree> {
    return (await call<FsTree>('/fs/tree' + q({ path }))).body!;
  },
  async fsMeta(paths: string[], columns: string[]): Promise<FsMeta['items']> {
    return (await call<FsMeta>('/fs/meta', json('POST', { paths, columns }))).body?.items ?? {};
  },
  // The first lines of a text file, read from its first few kilobytes only.
  async fsText(path: string, lines: number): Promise<string> {
    const text = (await call<FsText>('/fs/text' + q({ path, max: String(lines * 256) }))).body?.text ?? '';
    return text.split('\n').slice(0, lines).join('\n');
  },
  thumbUrl: (path: string, size: 'small' | 'large' = 'small', v?: number) => BASE + '/fs/thumb' + q({ path, size, ...(v ? { v: String(v) } : {}) }),
  // Names below `path` that match `query`, hidden ones only when they are shown.
  async search(path: string, query: string, hidden: boolean, signal?: AbortSignal): Promise<{ items: Item[]; more: boolean; unknown?: string[] }> {
    const body = (await call<SearchResult>('/search' + q({ path, q: serverQuery(query), hidden: String(hidden) }), { signal })).body!;
    return { items: body.items.map((h) => ({ ...toItem(h), at: h.at })), more: !!body.more, unknown: body.unknown ?? [] };
  },
  async watch(clientId: string, paths: string[]): Promise<void> {
    await call('/events/watch', json('PUT', { clientId, paths }));
  },
  async me(): Promise<Me> {
    return (await call<Me>('/me')).body!;
  },
  async bookmarks(): Promise<Bookmark[]> {
    return (await call<{ items: Bookmark[] }>('/me/bookmarks')).body!.items;
  },
  async saveBookmarks(items: Bookmark[]): Promise<void> {
    await call('/me/bookmarks', json('PUT', { items }));
  },
  async prefs(): Promise<Record<string, unknown>> {
    return (await call<Record<string, unknown>>('/me/prefs' + q(device()))).body ?? {};
  },
  async savePrefs(prefs: Record<string, unknown>): Promise<void> {
    await call('/me/prefs' + q(device()), json('PUT', prefs));
  },
};
