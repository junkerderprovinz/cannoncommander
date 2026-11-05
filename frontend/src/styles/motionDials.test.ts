// Every animation and transition of the app's own reads a dial, so the level setting reaches it. A fixed time plays
// every level at one speed, and nothing on screen says which one went deaf.
import { describe, expect, it } from 'vitest';
import { gates, inside, keyframes, lineOf, rules, sheets } from '@/testing/motionCss';

const all = sheets().map((s) => ({ ...s, gates: gates(s.css), rules: rules(s.css) }));

// A spinner turns under reduced motion too, at one slow pace, because one standing still reads as a hang.
const STILL_TURNING = ['a-spin', 'sp-spin', 'cl-spin'];

const declarations = (prop: RegExp) => all.flatMap((s) => s.rules.flatMap((r) => [...r.body.matchAll(prop)]
  .map((m) => ({ file: s.file, value: m[1]!.trim(), line: lineOf(s.css, r.at + m.index), gated: inside(s.gates, r.at) }))));

describe('the animations of the app', () => {
  const frames = all.flatMap((s) => keyframes(s.css).map((k) => ({ ...k, file: s.file })));
  const animations = declarations(/(?:^|;)\s*animation:\s*([^;]+)/g);

  it('are found, so the checks below have something to check', () => {
    expect(frames.length).toBeGreaterThanOrEqual(8);
  });

  it('each play inside the gate on a dial', () => {
    const deaf = frames.filter((k) => !animations.some((a) => a.gated && new RegExp(`(^|[\\s,])${k.name}\\b`).test(a.value) && a.value.includes('var(--motion-')));
    expect(deaf.map((k) => `${k.file}: ${k.name}`)).toEqual([]);
  });

  it('play outside the gate only as a spinner or not at all', () => {
    const loose = animations.filter((a) => !a.gated && a.value !== 'none' && !STILL_TURNING.some((n) => a.value.startsWith(n + ' ')));
    expect(loose.map((a) => `${a.file}:${a.line} ${a.value}`)).toEqual([]);
  });

  it('end every transform at none, so nothing left behind becomes the box a fixed child is placed in', () => {
    const open = frames.filter((k) => /transform:/.test(k.body)).filter((k) => {
      const last = [...k.body.matchAll(/(?:to|100%)\s*\{([^}]*)\}/g)].at(-1);
      return !last || !/transform:\s*none/.test(last[1]!);
    });
    expect(open.map((k) => `${k.file}: ${k.name}`)).toEqual([]);
  });
});

describe('the transitions of the app', () => {
  const transitions = declarations(/(?:^|;)\s*transition:\s*([^;]+)/g).filter((t) => t.value !== 'none');

  it('each read a dial', () => {
    expect(transitions.filter((t) => !t.value.includes('var(--motion-')).map((t) => `${t.file}:${t.line} ${t.value}`)).toEqual([]);
  });

  it('move things only inside the gate', () => {
    const moving = transitions.filter((t) => /\b(transform|rotate|scale|translate)\b/.test(t.value) && !t.gated);
    expect(moving.map((t) => `${t.file}:${t.line} ${t.value}`)).toEqual([]);
  });
});
