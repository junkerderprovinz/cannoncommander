import { createPinia, setActivePinia } from 'pinia';
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { type Job, ops } from '@/api/ops';
import { prefs } from '@/core/prefs';
import { winStack } from '@/core/windows';
import type { Entry } from '@/files/entries';
import { addMessages } from '@/i18n';
import jobsDe from '@/features/jobs/de.json';
import jobsEn from '@/features/jobs/en.json';
import de from './de.json';
import en from './en.json';
import { deleteItems } from './trash';

beforeAll(() => {
  addMessages('trash', { de, en });
  addMessages('jobs', { de: jobsDe, en: jobsEn });
});

const running: Job = {
  id: 't1', kind: 'trash', owner: 'jdp', state: 'running', position: 0, name: 'a.txt', count: 1, totalBytes: 0, doneBytes: 0,
  totalFiles: 1, doneFiles: 0, rate: 0, left: null, cap: 0, waitForParity: false, failedItems: 0, createdAt: '2026-11-20T09:30:00Z',
};

beforeEach(() => {
  setActivePinia(createPinia());
  winStack.splice(0);
  vi.restoreAllMocks();
  Object.assign(prefs, { confirmDelete: true, trash: true });
});

describe('the delete window', () => {
  it('stays after the confirm and shows the job it started', async () => {
    vi.spyOn(ops, 'startJob').mockResolvedValue(running);
    vi.spyOn(ops, 'trash').mockResolvedValue({ items: [], days: 30 } as never);
    deleteItems([{ name: 'a.txt' } as Entry], '/mnt/user/dokumente');
    const w = winStack.at(-1)!, go = w.foot.find((f) => f.primary)!;
    expect(go.run!(w.el!)).toBe(false);
    await new Promise((r) => setTimeout(r, 0));
    expect(winStack).toEqual([w]);
    expect(w.props).toMatchObject({ id: 't1' });
  });
});
