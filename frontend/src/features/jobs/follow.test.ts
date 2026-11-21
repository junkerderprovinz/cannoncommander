import { createPinia, setActivePinia } from 'pinia';
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { type Job, ops } from '@/api/ops';
import { prefs } from '@/core/prefs';
import { win, winStack } from '@/core/windows';
import { addMessages } from '@/i18n';
import { useJobs } from '@/stores/jobs';
import { usePanels } from '@/stores/panels';
import de from './de.json';
import en from './en.json';
import { followJob, toggleDetails } from './follow';

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
  followJob(w, j, '/mnt/user/fotos');
  return { J, w, j };
}

beforeEach(() => {
  setActivePinia(createPinia());
  winStack.splice(0);
  vi.restoreAllMocks();
  prefs.keepJobWindow = false;
  prefs.jobDetails = false;
});

describe('a window that started a job', () => {
  it('stays and shows the job running, with the background, a pause and a cancel, and the details switch at the start', () => {
    const { w } = started();
    expect(winStack).toEqual([w]);
    expect(w.props).toMatchObject({ id: 'j1', from: '/mnt/user/fotos' });
    expect(buttons()).toEqual(['Im Hintergrund', 'Anhalten', 'Abbrechen']);
    expect(w.startBody).toBeTruthy();
  });

  it('offers to go on while the job is paused', () => {
    const { J } = started();
    J.upsert(job({ state: 'paused' }));
    expect(buttons()).toEqual(['Im Hintergrund', 'Abbrechen', 'Weiter']);
  });

  it('closes by itself once the job is done and the box is unticked', () => {
    const { J } = started();
    J.upsert(job({ state: 'done', doneBytes: 100 }));
    expect(winStack).toHaveLength(0);
  });

  it('stays open with the result and the way to the target once the job is done and the box is ticked', () => {
    prefs.keepJobWindow = true;
    const { J } = started();
    J.upsert(job({ state: 'done', doneBytes: 100 }));
    expect(winStack).toHaveLength(1);
    expect(buttons()).toEqual(['Ziel öffnen', 'Schließen']);
    const go = vi.spyOn(usePanels(), 'go').mockResolvedValue(true);
    press('Ziel öffnen');
    expect(go).toHaveBeenCalledWith('/mnt/user/backups');
    expect(winStack).toHaveLength(0);
  });

  it('has nothing to open after a delete', () => {
    prefs.keepJobWindow = true;
    const { J } = started({ kind: 'trash', dest: undefined });
    J.upsert(job({ kind: 'trash', dest: undefined, state: 'done' }));
    expect(buttons()).toEqual(['Schließen']);
  });

  it('stays open after an error, whatever the box says, and can try again', async () => {
    const { J } = started();
    J.upsert(job({ state: 'failed', error: 'kein Platz' }));
    expect(winStack).toHaveLength(1);
    expect(buttons()).toEqual(['Wiederholen', 'Schließen']);
    vi.spyOn(ops, 'jobStep').mockResolvedValue(job({ state: 'running' }));
    press('Wiederholen');
    await vi.waitFor(() => expect(buttons()).toEqual(['Im Hintergrund', 'Anhalten', 'Abbrechen']));
  });

  it('can take up a cancelled job again', () => {
    const { J } = started();
    J.upsert(job({ state: 'cancelled' }));
    expect(buttons()).toEqual(['Wiederholen', 'Schließen']);
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

  it('pauses the job from its foot', () => {
    const step = vi.spyOn(ops, 'jobStep').mockResolvedValue(job({ state: 'paused' }));
    started();
    press('Anhalten');
    expect(step).toHaveBeenCalledWith('j1', 'pause');
    expect(winStack).toHaveLength(1);
  });

  it('remembers whether the window is opened up', () => {
    toggleDetails();
    expect(prefs.jobDetails).toBe(true);
    toggleDetails();
    expect(prefs.jobDetails).toBe(false);
  });
});
