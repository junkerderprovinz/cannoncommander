// The tools of the file and tools menus, share links and rename presets, as the contracts in
// backend/internal/{tools,fileops,fsapi,shares,prefs} describe them. The long runs start as jobs through ops.startJob.
import { BASE, call, json, q } from './client';
import type { Job } from './ops';

export type Algo = 'md5' | 'sha1' | 'sha256' | 'sha512' | 'b3';
export type CaseRule = 'keep' | 'lower' | 'upper' | 'title';
export interface RenameMask { name: string; ext: string; start: number; step: number; find: string; repl: string; regex: boolean; case: CaseRule }
export type RenameError = 'empty' | 'invalid' | 'exists' | 'duplicate';
export interface RenameRow { path: string; old: string; new: string; error?: RenameError; missing?: string[] }
export interface RenameBatchResult { rows: RenameRow[]; changed: number; errors: number; done: boolean; undo?: string }
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
  state: 'active' | 'expired' | 'exhausted';
}
export interface ShareCreate { paths: string[]; name?: string; slug?: string; expires?: string | null; password?: string; limit?: number; uploads?: boolean; notify?: boolean }
export type ShareUpdate = Partial<Omit<ShareCreate, 'paths'>> & { reroll?: boolean };

export interface UnraidWhere {
  path: string; volume?: string; share?: string; dir: boolean; files: number; size: number; partial?: boolean;
  on: { device: string; files: number; size: number; asleep: boolean }[];
  links?: { count: number; paths: string[]; complete: boolean };
}
export interface ArrowLoopJob { name: string; left: string; right: string; direction: 'right' | 'left' | 'both'; mirror?: boolean; every: 'hourly' | 'daily' | 'weekly'; at?: string }

const body = async <T>(path: string, init?: RequestInit) => (await call<T>(path, init)).body!;
const id = encodeURIComponent;

export const tools = {
  renameBatch: (req: { paths: string[]; mask?: RenameMask; names?: string[]; dryRun?: boolean }, signal?: AbortSignal) =>
    body<RenameBatchResult>('/fs/rename/batch', { ...json('POST', req), signal }),
  presets: async () => (await body<{ items: RenamePreset[] }>('/me/rename-presets')).items,
  savePresets: (items: RenamePreset[]) => call('/me/rename-presets', json('PUT', { items })),

  symlink: (target: string, path: string) => body<{ path: string; undo?: string }>('/fs/link', json('POST', { target, path })),
  // Answers how many entries changed, or with recursive the job that changes them.
  attrs: (req: { paths: string[]; mode?: string; owner?: string; group?: string; recursive?: boolean }) =>
    body<{ changed: number } | Job>('/fs/attrs', json('PATCH', req)),

  // Totals of a folder from the index, for its size and how many files and folders it holds.
  usage: (path: string) => body<{ size: number; files: number; dirs: number; mtime: string }>('/tools/diskusage' + q({ path, largest: '0' })),
  cleanupScan: (path: string, signal?: AbortSignal) => body<CleanupScan>('/tools/cleanup/scan', { ...json('POST', { path }), signal }),
  parseSums: (from: { path: string } | { text: string }) => body<{ lines: SumLine[] }>('/tools/checksum/parse', json('POST', from)).then((got) => got.lines),
  archiveFormats: () => body<{ read: string[]; pack: string[] }>('/tools/archives'),
  archiveInfo: (path: string, password?: string) =>
    body<ArchiveInfo>('/tools/archive' + q({ path }), password ? { headers: { 'X-Archive-Password': password } } : {}),
  where: (path: string) => body<UnraidWhere>('/unraid/where' + q({ path })),
  arrowLoop: (job: ArrowLoopJob) => body<{ job: unknown; url: string }>('/tools/arrowloop', json('POST', job)),

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
};

// The address visitors open, for links whose QR code was not asked for.
export const shareUrl = (slug: string) => `${location.origin}/s/${slug}`;
