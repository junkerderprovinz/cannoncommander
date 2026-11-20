import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { type Job, ops, type Preflight } from '@/api/ops';
import type { Entry } from '@/files/entries';
import { winStack } from '@/core/windows';
import { addMessages } from '@/i18n';
import jobsDe from '@/features/jobs/de.json';
import jobsEn from '@/features/jobs/en.json';
import de from './de.json';
import en from './en.json';
import { transfer } from './index';

addMessages('transfer', { de, en });
addMessages('jobs', { de: jobsDe, en: jobsEn });

const preflight = (more: Partial<Preflight>): Preflight => ({ dest: '/mnt/user/backups', count: 1, bytes: 10, files: 2, conflicts: [], same: [], sameDevice: false, big: false, parityBusy: false, ...more });
const job = { id: 'j1', kind: 'copy', state: 'queued', name: 'album', count: 1, dest: '/mnt/user/backups' } as Job;
const album = { name: 'album', dir: true } as Entry;
const conflict = { key: 'album', source: { path: '', dir: true, size: 0, mtime: '' }, target: { path: '', dir: true, size: 0, mtime: '' } };

beforeEach(() => {
  setActivePinia(createPinia());
  winStack.splice(0);
  vi.restoreAllMocks();
});

describe('the same copy again after a cancel', () => {
  it('takes the cancelled job up where it stopped, without asking about what it left', async () => {
    vi.spyOn(ops, 'preflight').mockResolvedValue(preflight({ resume: 'j1' }));
    const retried = vi.spyOn(ops, 'jobStep').mockResolvedValue(job);
    const started = vi.spyOn(ops, 'startJob').mockResolvedValue({ ...job, id: 'j2' });
    await transfer({ kind: 'copy', items: [album], dest: '/mnt/user/backups', from: '/mnt/user/fotos' });
    expect(retried).toHaveBeenCalledWith('j1', 'retry');
    expect(started).not.toHaveBeenCalled();
    expect(winStack).toHaveLength(0);
  });

  it('starts a job of its own when there is nothing to take up', async () => {
    vi.spyOn(ops, 'preflight').mockResolvedValue(preflight({}));
    const retried = vi.spyOn(ops, 'jobStep').mockResolvedValue(job);
    const started = vi.spyOn(ops, 'startJob').mockResolvedValue({ ...job, id: 'j2' });
    await transfer({ kind: 'copy', items: [album], dest: '/mnt/user/backups', from: '/mnt/user/fotos' });
    expect(started).toHaveBeenCalledTimes(1);
    expect(retried).not.toHaveBeenCalled();
  });

  it('still asks about a name that exists when there is nothing to take up', async () => {
    vi.spyOn(ops, 'preflight').mockResolvedValue(preflight({ conflicts: [conflict] }));
    void transfer({ kind: 'copy', items: [album], dest: '/mnt/user/backups', from: '/mnt/user/fotos' });
    await new Promise((r) => setTimeout(r, 0));
    expect(winStack.map((w) => w.cls)).toContain('cf-win');
  });
});
