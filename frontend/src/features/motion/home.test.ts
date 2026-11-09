import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeAll, beforeEach, describe, expect, it } from 'vitest';
import type { Job } from '@/api/ops';
import { useJobs } from '@/stores/jobs';
import { usePanels } from '@/stores/panels';
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

describe('a job coming home at a lively level', () => {
  const head = () => document.querySelector('.head')!;
  beforeEach(() => {
    document.documentElement.style.setProperty('--motion-spark-count', '6');
    document.body.insertAdjacentHTML('beforeend', '<div class="panel" data-panel="0"><div class="list"></div></div>');
    const list = document.querySelector<HTMLElement>('.list')!;
    list.getClientRects = () => [list.getBoundingClientRect()] as unknown as DOMRectList;
  });
  afterEach(() => document.documentElement.removeAttribute('style'));

  it('throws sparks from the avatar as many as the level asks for', () => {
    const jobs = useJobs();
    jobs.upsert({ ...job('running'), id: 'j3' });
    jobs.upsert({ ...job('done'), id: 'j3' });
    expect(document.querySelectorAll('.fx-layer .fx-spark').length).toBe(6);
  });

  it('settles the panel a copy went to', () => {
    usePanels().T(0).path = '/mnt/user/films';
    const jobs = useJobs();
    jobs.upsert({ ...job('running'), id: 'j4', dest: '/mnt/user/films' });
    jobs.upsert({ ...job('done'), id: 'j4', dest: '/mnt/user/films' });
    expect(document.querySelector('.list')!.classList).toContain('fx-land');
  });

  it('jolts the head when a job failed', () => {
    const jobs = useJobs();
    jobs.upsert({ ...job('running'), id: 'j5' });
    jobs.upsert({ ...job('failed'), id: 'j5' });
    expect(head().classList).toContain('fx-jolt');
  });
});

describe('a menu opening', () => {
  it('numbers its rows, so they come in one after another', async () => {
    document.body.insertAdjacentHTML('beforeend', '<div class="glim-menu"><button></button><div class="glim-menu-sep"></div><button></button></div>');
    await Promise.resolve();
    expect([...document.querySelectorAll<HTMLElement>('.glim-menu > *')].map((r) => r.style.getPropertyValue('--row-i'))).toEqual(['0', '1', '2']);
  });
});
