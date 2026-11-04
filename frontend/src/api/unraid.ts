// The array, its disks and shares, where a path lies, sleeping disks and the rights repair, as
// backend/internal/unraid/openapi.yaml describes them.
import { call, json, q } from './client';

export interface UnraidParity {
  running: boolean;
  paused: boolean;
  kind?: 'check' | 'rebuild' | 'clear';
  pos: number;
  size: number;
  percent: number;
  speed?: number;
  lastSync?: string;
  lastErrors: number;
  lastExit: number;
}
export interface UnraidDisk {
  name: string;
  kind: 'parity' | 'data' | 'pool' | 'flash';
  pool?: string;
  mount?: string;
  mounted: boolean;
  size: number;
  free: number;
  spundown: boolean;
  rotational: boolean;
  temp?: number;
}
export interface UnraidStorage { type: 'array' | 'pool'; name?: string }
export interface UnraidShare {
  name: string;
  primary: UnraidStorage;
  secondary?: UnraidStorage;
  mover?: 'toSecondary' | 'toPrimary';
  allocator: string;
  disks: string[];
  free: number;
  used: number;
}
export interface UnraidState {
  available: boolean;
  version?: string;
  parity: UnraidParity;
  disks: UnraidDisk[];
  shares: UnraidShare[];
  write: boolean;
  spindownDelay: number;
  mover?: { schedule?: string; active: boolean };
}
export interface UnraidOn { device: string; files: number; size: number; asleep: boolean }
export interface UnraidWhere {
  path: string;
  volume?: string;
  share?: string;
  dir: boolean;
  on: UnraidOn[];
  files: number;
  size: number;
  partial?: boolean;
  links?: { count: number; paths: string[]; complete: boolean };
}
export type KeptReason = 'appdata' | 'system' | 'domains' | 'flash';
export interface PermsScan {
  path: string;
  blocked?: KeptReason | 'archive';
  owner: string;
  files: number;
  dirs: number;
  wrong: number;
  kept: { path: string; reason: KeptReason; size?: number }[];
  partial?: boolean;
}
export interface PermsResult { entries: number; changed: number; failed: number; firstError?: string; kept: PermsScan['kept'] }
export interface SpinEvent { asleep: string[]; changed: string[] }

const body = async <T>(path: string, init?: RequestInit) => (await call<T>(path, init)).body!;

export const unraid = {
  state: () => body<UnraidState>('/unraid/state'),
  where: (path: string) => body<UnraidWhere>('/unraid/where' + q({ path })),
  spindown: async (disks?: string[]) => (await body<{ disks: string[] }>('/unraid/spindown', json('POST', disks ? { disks } : {}))).disks,
  permsScan: (path: string) => body<PermsScan>('/unraid/fixperms/scan', json('POST', { path })),
};
