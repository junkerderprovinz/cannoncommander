import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import { SRC } from '@/testing/motionCss';

const source = (file: string) => readFileSync(join(SRC, file), 'utf8');
const pill = (file: string, inner: RegExp) => source(file).match(new RegExp(`<Pill\\b[^>]*>(?=[^<]*${inner.source})`))?.[0] ?? '';
const grounds = readdirSync(SRC, { recursive: true, encoding: 'utf8' })
  .filter((f) => f.endsWith('.css'))
  .flatMap((file) => [...source(file).matchAll(/([^{}]+)\{([^{}]*)\}/g)].map((m) => ({ file, selector: m[1]!.trim(), body: m[2]! })))
  .filter((r) => /\.u-cache\b|\.glim-pill\.tag\b/.test(r.selector) && /background/.test(r.body));

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
});
