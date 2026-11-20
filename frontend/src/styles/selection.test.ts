import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import { SRC } from '@/testing/motionCss';

const sheets = readdirSync(SRC, { recursive: true, encoding: 'utf8' }).filter((f) => f.endsWith('.css'));
const rules = sheets.flatMap((file) =>
  [...readFileSync(join(SRC, file), 'utf8').replace(/\/\*[\s\S]*?\*\//g, '').matchAll(/([^{}]+)\{([^{}]*)\}/g)].map((m) => ({ file, selector: m[1]!.trim(), body: m[2]! })),
);
const on = (selector: string) => rules.filter((r) => r.selector.split(',').map((s) => s.trim()).includes(selector));

describe('a selected row in the file list', () => {
  it("takes GlimStone's wash of the accent behind it", () => {
    expect(on('.row.marked').map((r) => r.body).join()).toMatch(/background: var\(--accent-soft\)/);
  });

  it('is the cursor row while nothing is marked', () => {
    expect(on('.list:not(.marking) > .row.cursor').map((r) => r.body).join()).toMatch(/background: var\(--accent-soft\)/);
  });

  it('gets no frame, and neither does the cursor row', () => {
    const framed = rules.filter((r) => /\.row\.(cursor|marked)/.test(r.selector) && /outline:(?!\s*none)|border(-\w+)?:/.test(r.body));
    expect(framed.map((r) => `${r.file}: ${r.selector}`)).toEqual([]);
  });

  it('marks the cursor row in the active panel by a stronger wash when it is selected, and a neutral lift when not', () => {
    expect(on('.panel.active .row.cursor.marked').map((r) => r.body).join()).toMatch(/background: var\(--row-cursor\)/);
    expect(on('.panel.active .list:not(.marking) > .row.cursor').map((r) => r.body).join()).toMatch(/background: var\(--row-cursor\)/);
    expect(on('.panel.active .list.marking > .row.cursor:not(.marked)').map((r) => r.body).join()).toMatch(/background: var\(--hover\)/);
  });

  it('draws no bar at the start of the cursor row', () => {
    expect(rules.filter((r) => /\.row\.cursor/.test(r.selector) && /box-shadow:\s*inset/.test(r.body)).map((r) => `${r.file}: ${r.selector}`)).toEqual([]);
  });
});

describe('a selected row anywhere in the file manager', () => {
  it('takes the same wash in the column view, the bookmarks and the folder tree', () => {
    expect(on('.mitem.on').map((r) => r.body).join()).toMatch(/background: var\(--accent-soft\)/);
    expect(on('#side .glim-nav-row.glim-active').map((r) => r.body).join()).toMatch(/background-color: var\(--accent-soft\)/);
  });

  it('keeps the text and glyphs of a selected side row as on any row', () => {
    const side = rules.filter((r) => /\.glim-active.*(\.meta|\.tree-tog|\.vmeter|\.u-moon|svg)|#side .*\.glim-active/.test(r.selector) && /accent-contrast/.test(r.body));
    expect(side.map((r) => `${r.file}: ${r.selector}`)).toEqual([]);
  });
});

describe('the field that renames a row', () => {
  it('has no border, no ring and no ground, so the selected name and the caret mark it', () => {
    const own = on('.rn-field').map((r) => r.body).join();
    expect(own).toMatch(/border: 0/);
    expect(own).toMatch(/outline: none/);
    expect(own).toMatch(/background: transparent/);
    expect(own).toMatch(/caret-color: var\(--accent-ink\)/);
    // The row under the field already wears the wash, so the selected name takes the accent itself with its own ink.
    const picked = on('.rn-field::selection').map((r) => r.body).join();
    expect(picked).toMatch(/background: var\(--accent\)/);
    expect(picked).toMatch(/color: var\(--accent-contrast\)/);
    const lines = rules.filter((r) => r.selector.includes('.rn-field') && /border-color|box-shadow|outline:(?!\s*none)/.test(r.body));
    expect(lines.map((r) => `${r.file}: ${r.selector}`)).toEqual([]);
  });
});
