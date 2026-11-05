// Motion rules name the quiet levels, never the lively ones. A rule keyed on [data-motion="wild"] leaves out the storm
// above it and the page before the attribute is set, although the bare :root carries wild's numbers.
// :not([data-motion="subtle"]):not([data-motion="off"]) covers wild, storm, no attribute and any level added later.
// The scan reads the stylesheets rather than the page, because the defect is a level nobody rendered.
import { describe, expect, it } from 'vitest';
import { lineOf, sheets } from '@/testing/motionCss';

const all = sheets();

describe('motion selectors in the app', () => {
  it('never key a rule on a lively level by name', () => {
    const offenders = all.flatMap(({ file, css }) => [...css.matchAll(/\[data-motion=["'](wild|storm)["']\]/g)]
      // A brace right after the attribute opens the level's own token block, the one place it may be named.
      .filter((m) => !/^\s*\{/.test(css.slice(m.index + m[0].length)))
      .map((m) => `${file}:${lineOf(css, m.index)} names ${m[0]}; write :root:not([data-motion="subtle"]):not([data-motion="off"]) instead`));
    expect(offenders).toEqual([]);
  });

  it('still has quiet-level rules, so the scan has something to find', () => {
    // Otherwise the guard passes on a renamed attribute or a stylesheet without the levels.
    const quiet = all.flatMap(({ css }) => [...css.matchAll(/\[data-motion=["'](subtle|off)["']\]/g)]);
    expect(quiet.length).toBeGreaterThanOrEqual(5);
  });

  it('declares the hidden level its own block', () => {
    // Without it the selector rule still passes while the storm plays at wild's numbers.
    expect(all.some(({ css }) => css.includes(':root[data-motion="storm"] {'))).toBe(true);
  });
});
