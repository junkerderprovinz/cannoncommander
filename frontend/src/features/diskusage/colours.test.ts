import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { ACCENTS } from '@glimstone/core';
import { describe, expect, it } from 'vitest';
import { ageBase, AGES, ageKey, dimmed, fillOf, inkOf, legend, typeKey, typeMix, TYPES } from './colours';

const MB = 1 << 20;
const film = { name: 'Klassiker', dir: true, size: 100 * MB, files: 9, dirs: 1, mtime: '', kinds: { video: 90 * MB, img: 6 * MB, text: 4 * MB }, ages: [0, 0, 0, 0, 100 * MB, 0] };
const photos = { name: '2026', dir: true, size: 50 * MB, files: 90, dirs: 0, mtime: '', kinds: { img: 40 * MB, video: 10 * MB }, ages: [30 * MB, 20 * MB, 0, 0, 0, 0] };
const iso = { name: 'debian.iso', dir: false, size: 20 * MB, files: 1, dirs: 0, mtime: '', kinds: { disk: 20 * MB }, ages: [0, 0, 0, 0, 0, 20 * MB] };

describe('colour by file type', () => {
  it('gives a tile the colour of its largest type', () => {
    expect(typeKey(film)).toBe('video');
    expect(typeKey(photos)).toBe('photo');
    expect(typeKey(iso)).toBe('other');
    expect(fillOf('type', 'video')).toBe('var(--du-type-video)');
  });

  it('folds documents and text into one type and lists the types largest first', () => {
    const doc = { ...film, kinds: { doc: 3 * MB, text: 2 * MB, audio: 4 * MB } };
    expect(typeMix(doc.kinds)).toEqual([['docs', 5 * MB], ['music', 4 * MB]]);
  });

  it('totals the legend by the bytes of each type and leaves out the empty ones', () => {
    const keys = legend('type', [film, photos, iso]);
    expect(keys.map((k) => k.key)).toEqual(['video', 'photo', 'docs', 'other']);
    expect(keys.find((k) => k.key === 'photo')!.v).toBe(46 * MB);
    expect(keys.reduce((a, k) => a + k.v, 0)).toBe(170 * MB);
  });

  it('has six types, each with its own colour token', () => {
    expect(TYPES).toEqual(['video', 'photo', 'music', 'docs', 'archive', 'other']);
    expect(new Set(TYPES.map((k) => fillOf('type', k))).size).toBe(6);
  });
});

describe('colour by age', () => {
  it('gives a tile the colour of the age that holds most of its bytes', () => {
    expect(ageKey(film)).toBe('4');
    expect(ageKey(photos)).toBe('0');
    expect(ageKey({ ...iso, ages: undefined })).toBe(String(AGES.length - 1));
  });

  it('ramps from the accent to grey without any blue', () => {
    const fills = AGES.map((_, i) => fillOf('age', String(i)));
    expect(fills[0]).toBe('var(--du-age-0)');
    expect(new Set(fills).size).toBe(6);
  });

  it('totals the legend by the bytes in each age and keeps every step', () => {
    const keys = legend('age', [film, photos, iso]);
    expect(keys.map((k) => k.v)).toEqual([30 * MB, 20 * MB, 0, 0, 100 * MB, 20 * MB]);
  });
});

describe('the legend highlight', () => {
  it('dims every tile of another colour and leaves the matching ones', () => {
    expect(dimmed('type', film, 'video')).toBe(false);
    expect(dimmed('type', photos, 'video')).toBe(true);
    expect(dimmed('age', photos, '0')).toBe(false);
    expect(dimmed('age', film, '0')).toBe(true);
    expect(dimmed('type', photos, null)).toBe(false);
  });

  it('writes on each colour in the ink that stays readable on it', () => {
    expect(inkOf('type', 'video')).toBe('var(--du-ink-type-video)');
    expect(inkOf('age', '5')).toBe('var(--du-ink-age-5)');
  });
});

describe('the colour tokens', () => {
  const css = readFileSync(join(__dirname, 'diskusage.css'), 'utf8');
  const token = (name: string) => css.match(new RegExp(`--${name}:\\s*([^;]+);`))?.[1];

  it('defines a fill and an ink for every type and every age', () => {
    for (const k of TYPES) expect([k, token(`du-type-${k}`), token(`du-ink-type-${k}`)]).toEqual([k, expect.any(String), expect.any(String)]);
    AGES.forEach((_, i) => expect([i, token(`du-age-${i}`), token(`du-ink-age-${i}`)]).toEqual([i, expect.any(String), expect.any(String)]));
  });

  it('takes the five type colours from the GlimStone chart series and leaves the rest grey', () => {
    const hues = TYPES.filter((k) => k !== 'other').map((k) => token(`du-type-${k}`)!.match(/^var\(--chart-(\d)\)$/)?.[1]);
    expect(new Set(hues).size).toBe(5);
    expect(hues).not.toContain(undefined);
    expect(token('du-type-other')).toBe('var(--carbon-surface3)');
  });

  it('ramps the ages from the accent to grey without any blue', () => {
    expect(token('du-age-from')).toBe('var(--accent)');
    expect(token('du-age-0')).toBe('var(--du-age-from)');
    expect(token(`du-age-${AGES.length - 1}`)).toBe('var(--carbon-surface3)');
    AGES.forEach((_, i) => expect(token(`du-age-${i}`)).not.toMatch(/chart|#|oklch|rgb/));
  });
});

describe('the start of the age ramp', () => {
  it('is the accent unless the accent is blue, then sunflower', () => {
    const want = { Sunflower: 'accent', Blue: 'warm', Green: 'accent', Red: 'accent', Purple: 'accent', Orange: 'accent', Teal: 'warm', Magenta: 'accent' };
    expect(Object.fromEntries(ACCENTS.map((a) => [a.name, ageBase(a.hex)]))).toEqual(want);
  });

  it('takes a picked blue, a grey and a broken value as they come', () => {
    expect(ageBase('#2f6feb')).toBe('warm');
    expect(ageBase('#888888')).toBe('accent');
    expect(ageBase('nonsense')).toBe('accent');
  });

  it('turns warm in the stylesheet for every theme', () => {
    const css = readFileSync(join(__dirname, 'diskusage.css'), 'utf8');
    const warm = css.match(/:root\[data-du-age="warm"\][^{]*\{([^}]*)\}/)?.[1] ?? '';
    const from = warm.match(/--du-age-from:\s*(#[0-9a-fA-F]{6});/)?.[1];
    expect(from && ageBase(from)).toBe('accent');
    expect(warm).toMatch(/--du-age-ink:/);
    expect(css).toMatch(/:root\[data-du-age="warm"\] \[data-theme\]/);
  });
});

describe('the scrolled tabs', () => {
  it('keep room above the badge on the first card', () => {
    const css = readFileSync(join(__dirname, 'diskusage.css'), 'utf8');
    const rule = css.match(/\.du-drives-scroll \{([^}]*)\}/)?.[1] ?? '';
    expect(rule).toMatch(/overflow: auto/);
    // GlimStone's heading badge is 22px high and centred on the card's top edge, and --space-6 is 12px.
    expect(rule).toMatch(/padding-top: var\(--space-6\)/);
  });
});
