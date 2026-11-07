import { createPinia, setActivePinia } from 'pinia';
import { beforeAll, beforeEach, describe, expect, it } from 'vitest';
import type { Job } from '@/api/ops';
import { useJobs } from '@/stores/jobs';
import setup from '.';

const job = (state: Job['state']) => ({ id: 'j1', kind: 'copy', state, position: 0, createdAt: '2026-11-07T10:00:00Z' }) as Job;

let avatar: HTMLElement;
beforeAll(() => {
  setActivePinia(createPinia());
  setup();
});
beforeEach(() => {
  document.body.innerHTML = '<header class="head"><button class="avatar" data-account>JD</button></header>';
  avatar = document.querySelector('[data-account]')!;
  avatar.getClientRects = () => [avatar.getBoundingClientRect()] as unknown as DOMRectList;
  delete document.documentElement.dataset.motion;
});

describe('a job coming home', () => {
  it('nods on the avatar, whose menu keeps the jobs, when it is done', () => {
    const jobs = useJobs();
    jobs.upsert(job('running'));
    jobs.upsert(job('done'));
    expect(avatar.classList).toContain('glim-confirm');
  });

  it('shakes the avatar when it failed', () => {
    const jobs = useJobs();
    jobs.upsert({ ...job('running'), id: 'j2' });
    jobs.upsert({ ...job('failed'), id: 'j2' });
    expect(avatar.classList).toContain('glim-shake');
  });
});
