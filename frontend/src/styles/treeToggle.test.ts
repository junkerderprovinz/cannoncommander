import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import { SRC } from '@/testing/motionCss';

const sheets = readdirSync(SRC, { recursive: true, encoding: 'utf8' }).filter((f) => f.endsWith('.css'));
const rules = sheets.flatMap((file) =>
  [...readFileSync(join(SRC, file), 'utf8').matchAll(/([^{}]+)\{([^{}]*)\}/g)].map((m) => ({ file, selector: m[1]!.trim(), body: m[2]! })),
);
const hovers = rules.filter((r) => /\.tree-tog[^,]*:hover/.test(r.selector));

describe('the arrows that open a folder in a tree', () => {
  it('light only their glyph under the pointer, in the accent glyph colour', () => {
    const own = hovers.find((r) => r.selector === '.tree-tog:hover');
    expect(own?.body).toMatch(/color: var\(--accent-glyph\)/);
  });

  it('get no ground of their own under the pointer, in any sheet', () => {
    expect(hovers.filter((r) => /background/.test(r.body)).map((r) => `${r.file}: ${r.selector}`)).toEqual([]);
  });

  it('keep the contrast colour on the open row, where the accent is the ground', () => {
    const app = rules.filter((r) => r.file.endsWith('app.css'));
    const hover = app.findIndex((r) => r.selector === '.tree-tog:hover');
    const lit = app.findIndex((r) => r.selector === '.glim-active .tree-tog');
    expect(lit).toBeGreaterThan(hover);
    expect(app[lit]!.body).toMatch(/color: var\(--accent-contrast\)/);
  });
});
