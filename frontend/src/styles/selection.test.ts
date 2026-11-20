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
    const framed = rules.filter((r) => /\.row\.(cursor|marked)/.test(r.selector) && /outline:\s*(?!none)|border(-\w+)?:/.test(r.body));
    expect(framed.map((r) => `${r.file}: ${r.selector}`)).toEqual([]);
  });

  it('leaves the cursor row a bar at its start in the active panel, in either writing direction', () => {
    expect(on('.panel.active .row.cursor').map((r) => r.body).join()).toMatch(/box-shadow: inset 3px 0 0 var\(--accent-ink\)/);
    expect(on('[dir=rtl] .panel.active .row.cursor').map((r) => r.body).join()).toMatch(/box-shadow: inset -3px 0 0 var\(--accent-ink\)/);
  });
});
