import { afterEach, beforeAll, describe, expect, it } from 'vitest';
import { DAY, setClock } from '@/core/format';
import { addMessages } from '@/i18n';
import de from './de.json';
import { ago } from './shared';

const NOW = Date.UTC(2026, 10, 5, 22, 0);
beforeAll(() => addMessages('config', { de }));
afterEach(() => setClock(() => Date.now()));

describe('how long ago', () => {
  it('reads a time a few seconds ahead, as from a server whose clock runs fast, as just now', () => {
    setClock(() => NOW);
    expect(ago(NOW + 6000)).toBe('gerade eben');
    expect(ago(NOW - 30_000)).toBe('gerade eben');
  });

  it('still counts the days to a time that lies ahead', () => {
    setClock(() => NOW);
    expect(ago(NOW + DAY / 2)).toBe('morgen');
    expect(ago(NOW + 3 * DAY)).toBe('noch 3 Tage');
    expect(ago(NOW - 10 * 60_000)).toBe('vor 10 Min.');
  });
});
