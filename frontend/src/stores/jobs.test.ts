import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { type Job, ops } from '@/api/ops';
import { emit } from '@/api/sse';
import { shell } from '@/shell/state';
import { useJobs } from './jobs';

const job = (id: string, over: Partial<Job> = {}): Job => ({
  id, kind: 'copy', owner: 'jdp', state: 'queued', position: 0, name: 'a.txt', count: 1, dest: '/mnt/user/b', totalBytes: 100, doneBytes: 0,
  totalFiles: 1, doneFiles: 0, rate: 0, left: null, cap: 0, waitForParity: false, failedItems: 0, createdAt: '2026-11-02T09:30:00Z', ...over,
});

beforeEach(() => {
  setActivePinia(createPinia());
  vi.restoreAllMocks();
});

describe('job list', () => {
  it('keeps open jobs in queue order before the ended ones, newest first', async () => {
    vi.spyOn(ops, 'jobs').mockResolvedValue([
      job('old', { state: 'done', finishedAt: '2026-11-02T08:00:00Z' }), job('b', { position: 1 }), job('new', { state: 'failed', finishedAt: '2026-11-02T09:00:00Z' }), job('a', { position: 0, state: 'running' }),
    ]);
    const J = useJobs();
    await J.load();
    expect(J.list.map((j) => j.id)).toEqual(['a', 'b', 'new', 'old']);
    expect(shell.jobCount).toBe(2);
  });

  it('follows created, progress and state events', () => {
    const J = useJobs();
    emit({ type: 'job.created', data: job('x') });
    emit({ type: 'job.progress', data: { id: 'x', totalBytes: 100, doneBytes: 40, totalFiles: 1, doneFiles: 0, rate: 10, left: 6 } });
    expect(J.list[0]).toMatchObject({ id: 'x', doneBytes: 40, rate: 10 });
    emit({ type: 'job.state', data: job('x', { state: 'paused', doneBytes: 40 }) });
    expect(J.list[0]!.state).toBe('paused');
  });

  it('tells whoever waits once a job has ended, also when it ended before', () => {
    const J = useJobs(), seen: string[] = [];
    emit({ type: 'job.created', data: job('x') });
    J.whenDone(J.list[0]!, (j) => seen.push(`${j.id} ${j.state}`));
    emit({ type: 'job.done', data: job('x', { state: 'done', undo: 'u1' }) });
    J.whenDone(J.list[0]!, (j) => seen.push(`late ${j.undo}`));
    expect(seen).toEqual(['x done', 'late u1']);
  });

  it('hands a conflict to the ones who answer it and marks the job as asking', () => {
    const J = useJobs(), asked = vi.fn();
    J.onAsked(asked);
    emit({ type: 'job.created', data: job('x', { state: 'running' }) });
    const conflict = { key: 'a.txt', source: { path: '/a/a.txt', dir: false, size: 1, mtime: '' }, target: { path: '/b/a.txt', dir: false, size: 2, mtime: '' } };
    emit({ type: 'job.conflict', data: { id: 'x', conflict } });
    expect(J.list[0]!.state).toBe('asking');
    expect(asked).toHaveBeenCalledWith(expect.objectContaining({ id: 'x' }), conflict);
  });

  it('moves a job in the queue at once and lets the server settle the order', async () => {
    vi.spyOn(ops, 'jobs').mockResolvedValue([job('a', { position: 0 }), job('b', { position: 1 }), job('c', { position: 2 })]);
    const change = vi.spyOn(ops, 'changeJob').mockImplementation(async (id) => job(id, { position: 0 }));
    const J = useJobs();
    await J.load();
    const moving = J.move(J.list[2]!, 0);
    expect(J.running.map((j) => j.id)).toEqual(['c', 'a', 'b']);
    await moving;
    expect(change).toHaveBeenCalledWith('c', { position: 0 });
  });

  it('runs a job of its own, such as an upload, through its own steps', () => {
    const J = useJobs(), control = { pause: vi.fn(), resume: vi.fn(), cancel: vi.fn() };
    const own = J.addLocal(job('upload-1', { kind: 'upload', state: 'running', position: 99 }), control);
    J.pause(own);
    J.cancel(own);
    expect(control.pause).toHaveBeenCalledOnce();
    expect(control.cancel).toHaveBeenCalledOnce();
    const ended = vi.fn();
    J.onEnded(ended);
    own.state = 'done';
    J.localChanged(own, 'running');
    expect(ended).toHaveBeenCalledOnce();
  });
});
