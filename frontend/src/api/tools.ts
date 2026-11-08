// Search and the tools of the file and tools menus, as backend/internal/{search,prefs,tools,fileops,fsapi,shares}
// describe them: comparing and syncing folders, comparing files, duplicates, disk usage, batch rename, archives,
// checksums, clean up, share links and the hand over to ArrowLoop. The long runs start as jobs.
import { serverQuery, serverSize } from '@/files/fields';
import { BASE, call, type Item, json, q, toItem } from './client';
import type { Job } from './ops';
import type { FsItem } from './types';

// The search window's criteria as it keeps them and as a saved search stores them.
export interface SearchQuery { name: string; text: string; in: string; hidden: boolean; min: string; max: string; when: string; type: string }
// The attributes of Windows the properties turn on and off; pinned keeps a cloud file on the device.
export type WinFlag = 'readOnly' | 'hidden' | 'archive' | 'pinned';
export interface SavedSearch { id: number; name: string; query: SearchQuery }

export type Hit = Item & { at: string; line?: number };
export interface WireHit extends FsItem { at: string; line?: number }
export interface Found { path: string; items: Hit[]; total: number; more: boolean; scanned: number; pending: number; unknown: string[]; indexed: boolean }
export interface ContentHits { job: string; items: WireHit[]; total: number; scanned: number; done?: boolean }

export const toHit = (h: WireHit): Hit => ({ ...toItem(h), at: h.at, ...(h.line ? { line: h.line } : {}) });

export type CompareState = 'same' | 'newerL' | 'newerR' | 'differ' | 'onlyL' | 'onlyR';
export interface CompareSide { dir: boolean; size: number; mtime: string }
export interface Compared { rel: string; name: string; state: CompareState; left?: CompareSide; right?: CompareSide }
export interface Comparison { items: Compared[]; counts: Partial<Record<CompareState, number>> }

export type SyncDirection = 'right' | 'left' | 'both';
export interface SyncStep { key: string; rel: string; act: 'copy' | 'replace' | 'delete'; to: 'L' | 'R'; dir?: boolean; off?: boolean; why?: 'rightNewer' | 'leftNewer' | 'sameAge'; size: number }
export interface SyncRequest { left: string; right: string; direction: SyncDirection; content: boolean; recursive: boolean; mirror: boolean; hidden: boolean }
export interface ArrowLoopJob { name: string; left: string; right: string; direction: SyncDirection; mirror?: boolean; every: 'hourly' | 'daily' | 'weekly'; at?: string }

export interface DiffSide { path: string; size: number; mtime: string; lines: string[] }
export type DiffRowKind = 'eq' | 'chg' | 'del' | 'add';
export interface DiffRow { t: DiffRowKind; l?: number | null; r?: number | null }
export interface Diff { left: DiffSide; right: DiffSide; binary: boolean; hunks: number; rows: DiffRow[] }

export interface DupCopy { path: string; at: string; mtime: string }
export interface DupGroup { name: string; size: number; sha256: string; wasted: number; copies: DupCopy[] }
export interface Duplicates { groups: DupGroup[]; more?: boolean; wasted: number; files: number }

export interface UsageTotals { size: number; files: number; dirs: number; mtime: string }
export interface UsageEntry extends UsageTotals { name: string; dir: boolean }
export interface UsageKind { kind: string; name: string; files: number; size: number; exts: string[] }
export interface UsageFile { name: string; at: string; size: number; mtime: string }
export interface DiskUsage extends UsageTotals { path: string; items: UsageEntry[]; types: UsageKind[]; largest: UsageFile[]; indexed: boolean }

export type Algo = 'md5' | 'sha1' | 'sha256' | 'sha512' | 'b3';
export type CaseRule = 'keep' | 'lower' | 'upper' | 'title';
export interface RenameMask { name: string; ext: string; start: number; step: number; find: string; repl: string; regex: boolean; case: CaseRule }
export type RenameError = 'empty' | 'invalid' | 'exists' | 'duplicate';
export interface RenameRow { path: string; old: string; new: string; error?: RenameError; missing?: string[] }
export interface RenameBatchResult { rows: RenameRow[]; changed: number; errors: number; done: boolean; undo?: string }
// A saved layout of both panels: the folders of their tabs, the active tab of each and the columns of the list.
export interface ProfileInput { name: string; dual: boolean; columns: string[]; panels: { cur: number; tabs: string[] }[] }
export interface Profile extends ProfileInput { id: number }

export interface RenamePreset { name: string; options: Partial<Omit<RenameMask, 'start' | 'step'>> }

export interface CleanupFind { path: string; note?: string; size?: number; to?: string }
export type CleanupGroup = 'emptyDir' | 'emptyFile' | 'temp' | 'link' | 'ext' | 'name';
export interface CleanupScan { path: string; scanned: number; capped?: boolean; groups: Partial<Record<CleanupGroup, CleanupFind[]>> }

export interface SumLine { name: string; hash?: string; algo?: Algo; bad?: string }
export interface ArchiveInfo { path: string; format: string; entries: number; unpacked: number; encrypted: boolean }

export interface PackResult { path: string }
export interface UnpackResult { paths: string[]; skipped: number }
export interface TestedArchive { path: string; members: number; ok: boolean; damage?: { entry?: string; reason: string }; password?: boolean }
export interface ChecksumResult { algo: Algo; files: { path: string; name: string; hash: string }[]; saved?: string }
export interface VerifyResult { lines: { name: string; path?: string; algo?: Algo; state: 'ok' | 'mismatch' | 'missing' | 'bad' }[] }
export interface SplitResult { parts: number; first: string }
export interface CombineResult { path: string; checked: boolean }

export interface Share {
  id: string;
  slug: string;
  kind: 'file' | 'folder' | 'collection';
  name: string;
  paths: string[];
  owner: string;
  created: string;
  expires: string | null;
  password: boolean;
  limit: number;
  downloads: number;
  views: number;
  uploads: boolean;
  uploadLimit: number;
  uploaded: number;
  notify: boolean;
  message: string;
  state: 'active' | 'expired' | 'exhausted';
}
export interface ShareCreate { paths: string[]; name?: string; message?: string; slug?: string; expires?: string | null; password?: string; limit?: number; uploads?: boolean; notify?: boolean }
export type ShareUpdate = Partial<Omit<ShareCreate, 'paths'>> & { reroll?: boolean };

const body = async <T>(path: string, init?: RequestInit) => (await call<T>(path, init)).body!;
const id = encodeURIComponent;

// The criteria in the words, dates and numbers the server reads, which is also how a saved search keeps them, so it
// finds the same whatever language the interface speaks later.
export const serverSearch = (s: SearchQuery): SearchQuery => ({ ...s, name: serverQuery(s.name), min: serverSize(s.min), max: serverSize(s.max) });

// The criteria GET /search and the job contentSearch take, from what the window keeps.
export const searchParams = (search: SearchQuery) => {
  const s = serverSearch(search);
  return { path: s.in.trim() || '/', q: s.name, hidden: String(!!s.hidden), type: s.type || 'all', when: s.when || 'any', ...(s.min ? { min: s.min } : {}), ...(s.max ? { max: s.max } : {}) };
};

export const tools = {
  async find(s: SearchQuery, limit: number, signal?: AbortSignal): Promise<Found> {
    const got = await body<Omit<Found, 'items'> & { items: WireHit[] }>('/search' + q({ ...searchParams(s), limit: String(limit) }), { signal });
    return { ...got, items: got.items.map(toHit), more: !!got.more, pending: got.pending ?? 0, unknown: got.unknown ?? [] };
  },
  // A search for text inside files runs as a job; its hits come as search.hits events and in the job's result.
  findText: (s: SearchQuery) => body<Job>('/jobs', json('POST', { kind: 'contentSearch', params: { ...searchParams(s), hidden: !!s.hidden, text: s.text } })),

  saved: async () => (await body<{ items: SavedSearch[] }>('/search/saved')).items,
  saveSearch: (name: string, query: SearchQuery) => body<SavedSearch>('/search/saved', json('POST', { name, query })),
  changeSearch: (s: SavedSearch) => body<SavedSearch>(`/search/saved/${s.id}`, json('PUT', { name: s.name, query: s.query })),
  dropSearch: (s: SavedSearch) => call(`/search/saved/${s.id}`, { method: 'DELETE' }),

  compare: (left: string, right: string, opts: { content?: boolean; recursive?: boolean; top?: boolean; hidden?: boolean } = {}) =>
    body<Comparison>('/tools/compare', json('POST', { left, right, ...opts })),
  syncPlan: async (req: SyncRequest) => (await body<{ steps: SyncStep[] }>('/tools/sync/plan', json('POST', req))).steps,
  arrowLoop: (job: ArrowLoopJob) => body<{ job: unknown; url: string }>('/tools/arrowloop', json('POST', job)),

  diff: (left: string, right: string) => body<Diff>('/tools/diff', json('POST', { left, right })),
  duplicates: (path: string, by: 'name' | 'size', hidden: boolean) => body<Job>('/tools/duplicates', json('POST', { path, by, hidden })),
  // With largest at 0 only the totals and the entries come back, enough for a folder's size and count.
  diskUsage: (path: string, opts: { hidden?: boolean; kind?: string; largest?: number } = {}) => body<DiskUsage>('/tools/diskusage' + q({
    path, hidden: String(!!opts.hidden), ...(opts.kind ? { kind: opts.kind } : {}), ...(opts.largest != null ? { largest: String(opts.largest) } : {}),
  })),

  renameBatch: (req: { paths: string[]; mask?: RenameMask; names?: string[]; dryRun?: boolean }, signal?: AbortSignal) =>
    body<RenameBatchResult>('/fs/rename/batch', { ...json('POST', req), signal }),
  presets: async () => (await body<{ items: RenamePreset[] }>('/me/rename-presets')).items,
  savePresets: (items: RenamePreset[]) => call('/me/rename-presets', json('PUT', { items })),
  profiles: async () => (await body<{ items: Profile[] }>('/me/profiles')).items,
  saveProfile: (p: ProfileInput) => body<Profile>('/me/profiles', json('POST', p)),
  deleteProfile: (id: number) => call(`/me/profiles/${id}`, { method: 'DELETE' }),

  symlink: (target: string, path: string) => body<{ path: string; undo?: string }>('/fs/link', json('POST', { target, path })),
  shortcut: (target: string, path: string) => body<{ path: string; undo?: string }>('/fs/link', json('POST', { target, path, shortcut: true })),
  // Answers how many entries changed, or with recursive the job that changes them.
  attrs: (req: { paths: string[]; mode?: string; owner?: string; group?: string; flags?: Partial<Record<WinFlag, boolean>>; recursive?: boolean }) =>
    body<{ changed: number } | Job>('/fs/attrs', json('PATCH', req)),

  cleanupScan: (path: string, signal?: AbortSignal) => body<CleanupScan>('/tools/cleanup/scan', { ...json('POST', { path }), signal }),
  parseSums: (from: { path: string } | { text: string }) => body<{ lines: SumLine[] }>('/tools/checksum/parse', json('POST', from)).then((got) => got.lines),
  archiveFormats: () => body<{ read: string[]; pack: string[] }>('/tools/archives'),
  archiveInfo: (path: string, password?: string) =>
    body<ArchiveInfo>('/tools/archive' + q({ path }), password ? { headers: { 'X-Archive-Password': password } } : {}),

  shares: () => body<Share[]>('/shares'),
  allShares: () => body<Share[]>('/admin/shares'),
  share: (req: ShareCreate) => body<Share>('/shares', json('POST', req)),
  getShare: (share: string) => body<Share>(`/shares/${id(share)}`),
  changeShare: (share: string, change: ShareUpdate) => body<Share>(`/shares/${id(share)}`, json('PATCH', change)),
  dropShare: (share: string) => call(`/shares/${id(share)}`, { method: 'DELETE' }),
  address: (share: string) => body<{ url: string; qr: string }>(`/shares/${id(share)}/qr` + q({ origin: location.origin })),

  saveText: (path: string, text: string) => body<{ size: number; mtime: string }>('/fs/file' + q({ path }), json('PUT', { text })),
  rawUrl: (path: string) => BASE + '/fs/raw' + q({ path, download: 'true' }),
  // A form post lets the browser save the archive as it streams in, without holding it in memory first.
  archiveStream(paths: string[], format: 'zip' | 'tgz', name: string) {
    const form = document.createElement('form');
    Object.assign(form, { method: 'POST', action: BASE + '/fs/archive-stream', hidden: true });
    const add = (k: string, v: string) => form.append(Object.assign(document.createElement('input'), { type: 'hidden', name: k, value: v }));
    paths.forEach((p) => add('paths', p));
    add('format', format);
    add('name', name);
    document.body.append(form);
    form.submit();
    form.remove();
  },

  job: (job: string) => body<Job>(`/jobs/${id(job)}`),
};

// A link carries its slug, the address is the server's own one with /s/ in front.
export const shareUrl = (slug: string) => new URL(`s/${slug}`, location.origin + import.meta.env.BASE_URL).href;
