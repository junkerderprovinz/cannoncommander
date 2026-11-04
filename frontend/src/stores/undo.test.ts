import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ops } from '@/api/ops';
import { emit } from '@/api/sse';
import { setClock } from '@/core/format';
import { useJobs } from './jobs';
import { useTrash } from './trash';
import { useUndo } from './undo';

const settle = () => new Promise((r) => setTimeout(r, 0));

beforeEach(() => {
  setActivePinia(createPinia());
  vi.restoreAllMocks();
});

describe('undo', () => {
  it('lists the server steps and the local ones together, newest first', async () => {
    setClock(() => Date.parse('2026-11-02T09:31:00Z'));
    vi.spyOn(ops, 'undoSteps').mockResolvedValue([{ id: 's1', kind: 'mkdir', at: '2026-11-02T09:30:00Z', name: 'Neu', count: 1 }]);
    const U = useUndo();
    await U.load();
    U.push('„x“ vom Schnellzugriff gelöst', () => {});
    expect(U.list.map((s) => ('local' in s ? 'local' : s.server.id))).toEqual(['local', 's1']);
  });

  it('takes a local step only once', async () => {
    const run = vi.fn(), U = useUndo();
    const step = U.push('gelöst', run);
    expect(await U.take({ at: step.at, local: step })).toBe(true);
    expect(await U.take({ at: step.at, local: step })).toBe(false);
    expect(run).toHaveBeenCalledOnce();
  });

  it('asks the server to take a step back and follows the job that does it', async () => {
    const job = { id: 'j1', kind: 'undo', state: 'queued' } as never;
    const undo = vi.spyOn(ops, 'undo').mockResolvedValue({ step: { id: 's1', kind: 'move', at: '', name: 'a', count: 1 }, job });
    const U = useUndo();
    U.steps = [{ id: 's1', kind: 'move', at: '', name: 'a', count: 1 }];
    await U.take(U.list[0]!);
    expect(undo).toHaveBeenCalledWith('s1');
    expect(U.steps).toEqual([]);
    expect(useJobs().list.map((j) => j.id)).toEqual(['j1']);
  });

  it('reads the list again when the server says it changed', async () => {
    const steps = vi.spyOn(ops, 'undoSteps').mockResolvedValue([]);
    useUndo();
    steps.mockClear();
    emit({ type: 'undo.changed', data: { count: 0 } });
    await settle();
    expect(steps).toHaveBeenCalled();
  });
});

describe('trash', () => {
  it('reads the trash with its keeping time and again on trash.changed', async () => {
    const list = vi.spyOn(ops, 'trash').mockResolvedValue({ items: [], count: 0, bytes: 0, days: 14 });
    const T = useTrash();
    await T.load();
    expect(T.days).toBe(14);
    list.mockClear();
    emit({ type: 'trash.changed', data: {} });
    await settle();
    expect(list).toHaveBeenCalled();
  });
});
