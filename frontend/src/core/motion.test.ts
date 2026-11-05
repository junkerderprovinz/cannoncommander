import { afterEach, describe, expect, it, vi } from 'vitest';
import { lively, moving, stormy, toMs } from './motion';

function reduce(on: boolean) {
  vi.stubGlobal('matchMedia', (q: string) => ({ matches: on && q.includes('reduce') }));
}

afterEach(() => vi.unstubAllGlobals());

describe('the motion level', () => {
  it('moves at every level but off', () => {
    reduce(false);
    expect(['off', 'subtle', 'wild', 'storm', ''].map(moving)).toEqual([false, true, true, true, true]);
  });

  it('counts every level above subtle as lively, also one added later or none set yet', () => {
    reduce(false);
    expect(['off', 'subtle', 'wild', 'storm', '', 'gale'].map(lively)).toEqual([false, false, true, true, true, true]);
  });

  it('knows the storm', () => {
    reduce(false);
    expect(['wild', 'storm'].map(stormy)).toEqual([false, true]);
  });

  it('stands still when the system asks for less motion, the storm included', () => {
    reduce(true);
    expect(['subtle', 'wild', 'storm'].flatMap((l) => [moving(l), lively(l), stormy(l)]).some(Boolean)).toBe(false);
  });
});

describe('a dial in milliseconds', () => {
  it('reads both units', () => {
    expect([toMs('480ms'), toMs(' .9s'), toMs('1.25s'), toMs('0s')]).toEqual([480, 900, 1250, 0]);
  });

  it('reads nothing as zero', () => {
    expect([toMs(''), toMs('fast')]).toEqual([0, 0]);
  });
});
