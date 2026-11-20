import { createPinia, setActivePinia } from 'pinia';
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { type Job, ops } from '@/api/ops';
import { prefs } from '@/core/prefs';
import { win, winStack } from '@/core/windows';
import { addMessages } from '@/i18n';
import { useJobs } from '@/stores/jobs';
import de from './de.json';
import en from './en.json';
import { followJob } from './follow';

beforeAll(() => addMessages('jobs', { de, en }));

const job = (over: Partial<Job> = {}): Job => ({
  id: 'j1', kind: 'copy', owner: 'jdp', state: 'running', position: 0, name: 'Urlaub', count: 1, dest: '/mnt/user/backups', totalBytes: 100, doneBytes: 40,
  totalFiles: 4, doneFiles: 1, rate: 10, left: 6, cap: 0, waitForParity: false, failedItems: 0, createdAt: '2026-11-20T09:30:00Z', ...over,
});
const top = () => winStack.at(-1)!;
const buttons = () => top().foot.filter((f) => f.label).map((f) => f.label);
function press(label: string) {
  const w = top(), f = w.foot.find((b) => b.label === label)!;
  if (!f.run || f.run(w.el!) !== false) w.close();
}
function started(over: Partial<Job> = {}) {
  const J = useJobs(), j = job(over);
  J.upsert(j);
  const w = win({ title: 'Kopieren', body: 'confirm', foot: [] });
  followJob(w, j);
  return { J, w, j };
}

beforeEach(() => {
  setActivePinia(createPinia());
  winStack.splice(0);
  vi.restoreAllMocks();
  prefs.keepJobWindow = false;
});

describe('a window that started a job', () => {
  it('stays and shows the job running, with a way to the background and a way to cancel', () => {
    const { w } = started();
    expect(winStack).toEqual([w]);
    expect(w.props).toMatchObject({ id: 'j1' });
    expect(buttons()).toEqual(['Im Hintergrund', 'Abbrechen']);
  });

  it('closes by itself once the job is done and the box is unticked', () => {
    const { J } = started();
    J.upsert(job({ state: 'done', doneBytes: 100 }));
    expect(winStack).toHaveLength(0);
  });

  it('stays open with the result once the job is done and the box is ticked', () => {
    prefs.keepJobWindow = true;
    const { J } = started();
    J.upsert(job({ state: 'done', doneBytes: 100 }));
    expect(winStack).toHaveLength(1);
    expect(buttons()).toEqual(['Schließen']);
  });

  it('stays open after an error, whatever the box says', () => {
    const { J } = started();
    J.upsert(job({ state: 'failed', error: 'kein Platz' }));
    expect(winStack).toHaveLength(1);
    expect(buttons()).toEqual(['Schließen']);
  });

  it('goes to the background and leaves the job running', () => {
    const step = vi.spyOn(ops, 'jobStep');
    started();
    press('Im Hintergrund');
    expect(winStack).toHaveLength(0);
    expect(step).not.toHaveBeenCalled();
  });

  it('cancels the job from its foot and stays to show it end', () => {
    const step = vi.spyOn(ops, 'jobStep').mockResolvedValue(job({ state: 'cancelled' }));
    started();
    press('Abbrechen');
    expect(step).toHaveBeenCalledWith('j1', 'cancel');
    expect(winStack).toHaveLength(1);
  });
});
