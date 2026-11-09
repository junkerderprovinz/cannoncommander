import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Job, JobItem } from '@/api/ops';
import { host } from '@/core/host';
import { elevationAnswers } from './answers';

const { job } = vi.hoisted(() => ({ job: vi.fn() }));
vi.mock('@/api/ops', async (original) => {
  const real = await original<typeof import('@/api/ops')>();
  return { ...real, ops: { ...real.ops, job } };
});

const failed = (kind: string, items: Partial<JobItem>[], errorCode?: string) => {
  job.mockResolvedValue({ items: items.map((i, seq) => ({ seq, path: '/C/x', state: 'failed', ...i })) });
  return { id: 'j1', kind, state: 'failed', errorCode } as Job;
};

afterEach(() => Object.assign(host, { desktop: false, os: '' }));

describe('a failed job the offer to go on as administrator answers', () => {
  it('keeps the failure toast away when the offer or its answer speaks', async () => {
    Object.assign(host, { desktop: true, os: 'linux' });
    expect(await elevationAnswers(failed('copy', [{ errorCode: 'denied' }]))).toBe(true);
    expect(await elevationAnswers(failed('move', [{ state: 'pending' }], 'denied'))).toBe(true);
    expect(await elevationAnswers(failed('delete', [{ errorCode: 'elevationDeclined' }]))).toBe(true);
  });

  it('leaves every other failure to its own toast', async () => {
    Object.assign(host, { desktop: true, os: 'linux' });
    expect(await elevationAnswers(failed('copy', [{ errorCode: 'noSpace' }]))).toBe(false);
    expect(await elevationAnswers(failed('pack', [{ errorCode: 'denied' }]))).toBe(false);
    Object.assign(host, { desktop: false });
    expect(await elevationAnswers(failed('copy', [{ errorCode: 'denied' }]))).toBe(false);
  });
});
