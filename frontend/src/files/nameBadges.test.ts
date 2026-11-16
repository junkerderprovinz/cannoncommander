import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import { SRC } from '@/testing/motionCss';

const source = (file: string) => readFileSync(join(SRC, file), 'utf8');
const pill = (file: string, inner: RegExp) => source(file).match(new RegExp(`<Pill\\b[^>]*>(?=[^<]*${inner.source})`))?.[0] ?? '';
const rules = readdirSync(SRC, { recursive: true, encoding: 'utf8' })
  .filter((f) => f.endsWith('.css'))
  .flatMap((file) => [...source(file).replace(/\/\*[\s\S]*?\*\//g, '').matchAll(/([^{}]+)\{([^{}]*)\}/g)].map((m) => ({ file, selector: m[1]!.trim(), body: m[2]! })));
const grounds = rules.filter((r) => /\.u-cache\b|\.glim-pill\.tag\b/.test(r.selector) && /background/.test(r.body));

const tokens = readFileSync(join(SRC, '..', 'node_modules', '@glimstone', 'core', 'tokens.css'), 'utf8');
const pairs = (token: string) => [...tokens.matchAll(new RegExp(`${token}: light-dark\\((#[0-9a-f]{6}), (#[0-9a-f]{6})\\)`, 'gi'))].map((m) => [m[1]!, m[2]!] as const);
const luminance = (hex: string) => {
  const [r, g, b] = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255).map((c) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4));
  return 0.2126 * r! + 0.7152 * g! + 0.0722 * b!;
};
const contrast = (a: string, b: string) => {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi! + 0.05) / (lo! + 0.05);
};

describe('the badges beside a name', () => {
  it('draw the archive bit in the neutral pill', () => {
    const archive = pill('files/ListItem.vue', /files\.archive/);
    expect(archive).toMatch(/v-if="k\.archive"/);
    expect(archive).not.toMatch(/tone=/);
  });

  it('draw a file waiting for the mover in the neutral pill', () => {
    const cache = pill('features/unraid/CacheBadge.vue', /pool === 'cache'/);
    expect(cache).toMatch(/class="u-cache"/);
    expect(cache).not.toMatch(/tone=/);
  });

  it('draw tags in the neutral pill', () => {
    expect(pill('features/labels/Marks.vue', /#\{\{ tag \}\}/)).not.toMatch(/tone=/);
  });

  it('get no ground of their own over the pill', () => {
    expect(grounds.map((r) => `${r.file}: ${r.selector}`)).toEqual([]);
  });

  it('sit on the third surface in the light theme and on the second in the dark one', () => {
    const name = rules.find((r) => r.file.endsWith('app.css') && r.selector === '.name' && /--pill-ground/.test(r.body));
    expect(name?.body).toMatch(/--pill-ground: light-dark\(var\(--carbon-surface3\), var\(--carbon-surface2\)\)/);
  });

  it('keep their words at 4.5:1 on that ground in every theme', () => {
    const ink = pairs('--carbon-text-sub'), light = pairs('--carbon-surface3'), dark = pairs('--carbon-surface2');
    expect(ink.length && light.length && dark.length).toBeTruthy();
    for (const [inkLight, inkDark] of ink) {
      for (const [ground] of light) expect(contrast(inkLight, ground), `${inkLight} on ${ground}`).toBeGreaterThanOrEqual(4.5);
      for (const [, ground] of dark) expect(contrast(inkDark, ground), `${inkDark} on ${ground}`).toBeGreaterThanOrEqual(4.5);
    }
  });
});
