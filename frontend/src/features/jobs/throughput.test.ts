import { beforeEach, describe, expect, it } from 'vitest';
import { forget, record, samplesOf } from './throughput';

beforeEach(() => forget('j1'));

describe('the speed a job had along its way', () => {
  it('keeps one sample per half percent, the latest rate winning', () => {
    record('j1', 10, 100, 1000);
    record('j1', 10.2, 120, 1500);
    record('j1', 11, 90, 2000);
    expect(samplesOf('j1')).toEqual([{ pct: 10, rate: 120, at: 1500 }, { pct: 11, rate: 90, at: 2000 }]);
  });

  it('passes over a step back and a job that has not moved yet', () => {
    record('j1', 0, 0, 0);
    record('j1', 20, 100, 1000);
    record('j1', 15, 100, 1200);
    expect(samplesOf('j1').map((s) => s.pct)).toEqual([20]);
  });

  it('keeps no more than two hundred samples', () => {
    for (let k = 1; k <= 300; k++) record('j1', k / 3, 50, k);
    expect(samplesOf('j1').length).toBeLessThanOrEqual(200);
    expect(samplesOf('j1').at(-1)!.pct).toBeGreaterThanOrEqual(99.5);
  });

  it('knows nothing of a job it never saw', () => {
    expect(samplesOf('other')).toEqual([]);
  });
});
