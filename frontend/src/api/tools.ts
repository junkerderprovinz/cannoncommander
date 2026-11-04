// Search, saved searches and the folder tools, as backend/internal/{search,prefs,tools} describe them: comparing and
// syncing two folders, comparing two files, duplicates and disk usage, and the hand over to ArrowLoop.
import { call, type Item, json, q, toItem } from './client';
import type { Job } from './ops';
import type { FsItem } from './types';

// The search window's criteria as it keeps them and as a saved search stores them.
export interface SearchQuery { name: string; text: string; in: string; hidden: boolean; min: string; max: string; when: string; type: string }
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
export interface ArrowLoopRequest { name: string; left: string; right: string; direction: SyncDirection; mirror: boolean; every: 'hourly' | 'daily' | 'weekly'; at: string }

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

const body = async <T>(path: string, init?: RequestInit) => (await call<T>(path, init)).body!;
const id = encodeURIComponent;

// The criteria GET /search and the job contentSearch take, from what the window keeps.
export const searchParams = (s: SearchQuery) => ({
  path: s.in.trim() || '/', q: s.name, hidden: String(!!s.hidden), type: s.type || 'all', when: s.when || 'any', ...(s.min ? { min: s.min } : {}), ...(s.max ? { max: s.max } : {}),
});

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
  sync: (left: string, right: string, steps: Pick<SyncStep, 'rel' | 'act' | 'to'>[]) => body<Job>('/jobs', json('POST', { kind: 'sync', params: { left, right, steps } })),
  arrowLoop: (req: ArrowLoopRequest) => body<{ job: unknown; url: string }>('/tools/arrowloop', json('POST', req)),

  diff: (left: string, right: string) => body<Diff>('/tools/diff', json('POST', { left, right })),
  duplicates: (path: string, by: 'name' | 'size', hidden: boolean) => body<Job>('/tools/duplicates', json('POST', { path, by, hidden })),
  diskUsage: (path: string, hidden: boolean, kind?: string) => body<DiskUsage>('/tools/diskusage' + q({ path, hidden: String(hidden), ...(kind ? { kind } : {}) })),

  job: (job: string) => body<Job>(`/jobs/${id(job)}`),
};
