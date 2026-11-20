import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import { SRC } from '@/testing/motionCss';

const sheets = readdirSync(SRC, { recursive: true, encoding: 'utf8' }).filter((f) => f.endsWith('.css'));
const rules = sheets.flatMap((file) =>
  [...readFileSync(join(SRC, file), 'utf8').replace(/\/\*[\s\S]*?\*\//g, '').matchAll(/([^{}]+)\{([^{}]*)\}/g)].map((m) => ({ file, selector: m[1]!.trim(), body: m[2]! })),
);

describe("the desktop window's own title bar", () => {
  // GlimStone sets the height inside a cascade layer, which any rule of the app's own outside a layer would beat, and
  // the page would slide under the bar.
  it("leaves the bar's height to GlimStone", () => {
    expect(rules.filter((r) => /--window-bar\s*:/.test(r.body)).map((r) => `${r.file}: ${r.selector}`)).toEqual([]);
  });

  it('keeps the app below the bar', () => {
    expect(rules.find((r) => r.selector === '.app')?.body).toMatch(/height: calc\(100vh - var\(--window-bar\)\)/);
  });
});
