// Every motion level answers every dial of the app's own. A level that leaves one out inherits the bare :root value,
// which is the lively one, so subtle would keep a lively number and the storm would keep wild's, and the block would
// still read as complete because every line in it is right.
import { describe, expect, it } from 'vitest';
import { gates, glimstoneTokens, inside, sheets } from '@/testing/motionCss';

const all = sheets();
const dials = all.find((s) => s.file === 'styles/motion.css')!.css;

function tokensOf(selector: string): Set<string> {
  const at = dials.indexOf(selector + ' {');
  if (at < 0) return new Set();
  const body = dials.slice(at, dials.indexOf('}', at));
  return new Set([...body.matchAll(/(--motion-[a-z-]+)\s*:/g)].map((m) => m[1]!));
}

const lively = tokensOf(':root');
const library = new Set([...glimstoneTokens().matchAll(/(--motion-[a-z-]+)\s*:/g)].map((m) => m[1]!));

describe('the motion dials of the app', () => {
  it('sets the lively numbers on the bare root', () => {
    // With only a handful, the comparisons below would check nothing.
    expect(lively.size).toBeGreaterThanOrEqual(20);
  });

  for (const level of ['subtle', 'off', 'storm']) {
    it(`${level} answers every dial`, () => {
      const mine = tokensOf(`:root[data-motion="${level}"]`);
      expect([...lively].filter((t) => !mine.has(t))).toEqual([]);
    });
  }

  it('sets no level a dial the lively numbers lack, which would go missing on the bare root', () => {
    const extra = ['subtle', 'off', 'storm'].flatMap((l) => [...tokensOf(`:root[data-motion="${l}"]`)].filter((t) => !lively.has(t)));
    expect(extra).toEqual([]);
  });

  it('leaves GlimStone its own dials', () => {
    // Setting one of them here would quietly change every GlimStone component that reads it.
    expect([...lively].filter((t) => library.has(t))).toEqual([]);
  });

  it('keeps the levels inside the gate, so a system that asked for less motion never reads them', () => {
    const ranges = gates(dials);
    for (const selector of [':root {', ':root[data-motion="subtle"] {', ':root[data-motion="off"] {', ':root[data-motion="storm"] {']) {
      expect(inside(ranges, dials.indexOf(selector)), selector).toBe(true);
    }
  });

  it('reads no dial that nobody declares', () => {
    const unknown = all.flatMap(({ file, css }) => [...css.matchAll(/var\((--motion-[a-z-]+)/g)]
      .filter((m) => !lively.has(m[1]!) && !library.has(m[1]!)).map((m) => `${file}: ${m[1]}`));
    expect(unknown).toEqual([]);
  });
});
