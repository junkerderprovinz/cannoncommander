// What the server no longer uses and the storeroom it waits in, as backend/internal/storeroom/openapi.yaml describes it.
import { call, json } from './client';

export type FindKind = 'template' | 'stack' | 'appdata' | 'container' | 'network' | 'volume' | 'image' | 'cache';
export interface StoreroomFind { kind: FindKind; id: string; name: string; path?: string; size: number; forGood: boolean }
export interface StoreroomEntry { id: string; kind: Exclude<FindKind, 'image' | 'cache'>; name: string; from: string; held: string; at: string; size: number }
export interface StoreroomScan { docker: boolean; write: boolean; finds: StoreroomFind[]; room: StoreroomEntry[] }
export type StoreroomOp = 'store' | 'restore' | 'discard';
export interface StoreroomRef { kind?: string; id: string }
export interface StoreroomStep { kind: string; name: string; from: string; to?: string; forGood: boolean; done: boolean; problem?: 'gone' | 'inUse' | 'taken' | 'failed'; error?: string }

const steps = async (path: string, op: StoreroomOp, refs: StoreroomRef[]) => (await call<{ steps: StoreroomStep[] }>(path, json('POST', { op, refs }))).body!.steps;

export const storeroom = {
  scan: async () => (await call<StoreroomScan>('/storeroom/scan')).body!,
  plan: (op: StoreroomOp, refs: StoreroomRef[]) => steps('/storeroom/plan', op, refs),
  run: (op: StoreroomOp, refs: StoreroomRef[]) => steps('/storeroom/run', op, refs),
};
