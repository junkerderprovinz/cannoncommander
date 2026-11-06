// Jobs, the check before a transfer, creating and renaming, the trash, undo, uploads and Unraid's gather plan, as the
// server's contracts in backend/internal/{jobs,fileops,trash,undo,unraid} describe them.
import { call, json } from './client';

export type JobState = 'queued' | 'running' | 'paused' | 'asking' | 'done' | 'failed' | 'cancelled';
export type Rule = 'ask' | 'skip' | 'replace' | 'both' | 'newer';
export type Choice = Exclude<Rule, 'ask'>;

export interface ConflictSide { path: string; dir: boolean; size: number; mtime: string; entries?: number }
export interface JobConflict { key: string; source: ConflictSide; target: ConflictSide; newer?: 'source' | 'target' }

export interface Job {
  id: string;
  kind: string;
  owner: string;
  state: JobState;
  reason?: 'parity';
  place?: number;
  position: number;
  name: string;
  count: number;
  dest?: string;
  totalBytes: number;
  doneBytes: number;
  totalFiles: number;
  doneFiles: number;
  rate: number;
  left: number | null;
  current?: string;
  cap: number;
  waitForParity: boolean;
  failedItems: number;
  error?: string;
  errorCode?: string;
  errorParams?: Record<string, unknown>;
  conflict?: JobConflict;
  undo?: string;
  result?: unknown;
  createdAt: string;
  startedAt?: string;
  finishedAt?: string;
}
export interface JobProgress { id: string; totalBytes: number; doneBytes: number; totalFiles: number; doneFiles: number; rate: number; left: number | null; current?: string }
export interface JobSettings { mode: 'serial' | 'parallel'; cap: number; parityWait: boolean; notifyUnraid: boolean; notifyAfter: number }

export interface TransferParams {
  sources: string[];
  dest: string;
  names?: string[];
  rule?: Rule;
  decisions?: Record<string, Choice>;
  verify?: boolean;
  keepAttrs?: boolean;
}
export interface JobCreate { kind: string; params: unknown; cap?: number; waitForParity?: boolean }

export interface UnraidTransferReport {
  trap?: { device: string; count: number; size: number; files: { path: string; device: string; size: number }[]; targets: { disk: string; free: number }[] };
  links?: { count: number; extra: number; files: { path: string; size: number; names: number; others: string[]; complete: boolean }[] };
  space?: {
    problem: 'noDisk' | 'tooMuch' | 'bigFile' | 'poolFull'; share: string; allocator: string; floor: number; total: number;
    biggest?: { path: string; size: number }; room: number; disk?: string; free: number; pool?: string; roomy?: string; roomyFree?: number;
  };
  linkable: boolean;
  wakes: string[];
  lands?: string;
  partial?: boolean;
}
export interface Preflight {
  dest: string;
  count: number;
  bytes: number;
  files: number;
  conflicts: JobConflict[];
  // Names at dest that are the same file as their source under another path; the job leaves them alone.
  same: string[];
  sameDevice: boolean;
  space?: { free: number; need: number; enough: boolean };
  big: boolean;
  parityBusy: boolean;
  unraid?: UnraidTransferReport;
}

export interface FsChanged { path: string; created?: boolean; undo?: string }

export interface TrashItem {
  id: string;
  name: string;
  origin: string;
  from: string;
  dir: boolean;
  size: number;
  files: number;
  by: string;
  deletedAt: string;
  expiresAt: string;
}
export interface TrashList { items: TrashItem[]; count: number; bytes: number; days: number }

export interface UndoStep { id: string; kind: 'rename' | 'renameBatch' | 'mkdir' | 'touch' | 'link' | 'copy' | 'move' | 'trash'; at: string; name: string; count: number; path?: string; to?: string }

export interface Upload { id: string; path: string; size: number; offset: number; state: 'open' | 'done' | 'skipped'; rule: string; mtime?: string; chunk: number }
export interface UploadRequest { path: string; size: number; mtime?: string; rule?: Rule }

export interface GatherDevice { device: string; now: number; files: number; after: number; free: number; asleep?: boolean }
export interface GatherPlan {
  path: string;
  mode: 'gather' | 'scatter';
  disk?: string;
  files: number;
  size: number;
  devices: GatherDevice[];
  options: { disk: string; move: number; short: number }[];
  moves: number;
  move: number;
  short: number;
  cached: number;
  partial?: boolean;
}

const body = async <T>(path: string, init?: RequestInit) => (await call<T>(path, init)).body!;
const id = encodeURIComponent;

export const ops = {
  jobs: async () => (await body<{ jobs: Job[] }>('/jobs')).jobs,
  startJob: (req: JobCreate) => body<Job>('/jobs', json('POST', req)),
  clearJobs: () => body<{ removed: number }>('/jobs', { method: 'DELETE' }),
  changeJob: (job: string, change: { position?: number; cap?: number }) => body<Job>(`/jobs/${id(job)}`, json('PATCH', change)),
  removeJob: (job: string) => call(`/jobs/${id(job)}`, { method: 'DELETE' }),
  jobStep: (job: string, step: 'pause' | 'resume' | 'cancel' | 'retry') => body<Job>(`/jobs/${id(job)}/${step}`, { method: 'POST' }),
  decide: (job: string, answer: { key?: string; choice: Choice; all?: boolean }) => body<Job>(`/jobs/${id(job)}/decide`, json('POST', answer)),
  jobSettings: () => body<JobSettings>('/jobs/settings'),
  saveJobSettings: (s: JobSettings) => body<JobSettings>('/jobs/settings', json('PUT', s)),

  preflight: (kind: 'copy' | 'move', params: TransferParams) => body<Preflight>('/ops/preflight', json('POST', { kind, ...params })),
  mkdir: (path: string) => body<FsChanged>('/fs/mkdir', json('POST', { path })),
  touch: (path: string) => body<FsChanged>('/fs/touch', json('POST', { path })),
  rename: (path: string, name: string) => body<FsChanged>('/fs/rename', json('POST', { path, name })),
  // A hard link is a second name for the same data; target is the existing file, path the new name.
  link: (target: string, path: string) => body<FsChanged>('/fs/link', json('POST', { target, path, hard: true })),

  trash: () => body<TrashList>('/trash'),
  restore: (ids: string[]) => body<Job>('/trash/restore', json('POST', { ids })),
  emptyTrash: (ids?: string[]) => body<Job>('/trash/empty', ids ? json('POST', { ids }) : { method: 'POST' }),

  undoSteps: async () => (await body<{ steps: UndoStep[] }>('/undo')).steps,
  undo: (step?: string) => body<{ step: UndoStep; job?: Job }>('/undo', step ? json('POST', { id: step }) : { method: 'POST' }),

  uploadStart: (req: UploadRequest) => call<Upload>('/uploads', json('POST', req)).then((got) => got.body!),
  uploadState: (upload: string) => body<Upload>(`/uploads/${id(upload)}`),
  uploadChunk: (upload: string, offset: number, chunk: Blob, signal?: AbortSignal) => body<Upload>(`/uploads/${id(upload)}`, {
    method: 'PUT', body: chunk, signal, headers: { 'Content-Type': 'application/octet-stream', 'Upload-Offset': String(offset) },
  }),
  uploadCancel: (upload: string) => call(`/uploads/${id(upload)}`, { method: 'DELETE' }),

  gatherPlan: (path: string, mode: 'gather' | 'scatter', disk?: string) => body<GatherPlan>('/unraid/gather/plan', json('POST', { path, mode, ...(disk ? { disk } : {}) })),
};
