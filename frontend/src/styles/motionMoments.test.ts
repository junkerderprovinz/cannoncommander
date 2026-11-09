// Every moment that moves reads its size from the level, and the levels are far enough apart to tell by eye: the
// storm throws further, tilts more and fires more pieces than wild, and wild more than subtle. A dial that only grows
// a little from one level to the next makes two levels look the same, which is what nobody notices in a review.
import { describe, expect, it } from 'vitest';
import { gates, inside, keyframes, rules, sheets } from '@/testing/motionCss';

const all = sheets();
const dials = all.find((s) => s.file === 'styles/motion.css')!.css;

function block(selector: string) {
  const at = dials.indexOf(selector + ' {');
  const body = dials.slice(at, dials.indexOf('}', at));
  return new Map([...body.matchAll(/(--motion-[a-z-]+)\s*:\s*([^;]+);/g)].map((m) => [m[1]!, m[2]!.trim()]));
}

const LEVELS = { off: block(':root[data-motion="off"]'), subtle: block(':root[data-motion="subtle"]'), wild: block(':root'), storm: block(':root[data-motion="storm"]') };
const isTime = (v: string) => /^-?[\d.]+m?s$/.test(v);
const num = (v: string) => parseFloat(v);

// Durations that set a pace: a shorter turn of the spinner or run of the dashes is the livelier one.
const PACE = ['--motion-spin-dur', '--motion-hue-dur', '--motion-flow-dur', '--motion-trail-flicker', '--motion-dropzone-dur'];
// A step must grow the movement by at least this much, measured from the still value at off.
const APART = 4 / 3;

// The moments of the app and the dials each has to read, in its rule or in the keyframes the rule plays.
const MOMENTS: [string, RegExp, string[]][] = [
  ['a copy or move setting off', /\.fx-trace\b/, ['trace-dur', 'trace-length', 'trace-width']],
  ['an entry falling into the trash', /\.fx-drop\b/, ['drop-dur', 'drop-reach', 'drop-tilt']],
  ['the trash taking the hit', /\.fx-bin-hit\b/, ['bin-tilt']],
  ['a finished job throwing sparks', /\.fx-spark\b/, ['spark-dur', 'spark-reach']],
  ['a finished copy landing in its panel', /\.fx-land\b/, ['land-dur', 'land-dist']],
  ['a failed job jolting the head', /\.fx-jolt\b/, ['jolt-dur', 'jolt-dist']],
  ['a menu opening', /\.glim-menu(?![\w-])(?!\s*>)/, ['menu-dur', 'menu-travel']],
  ['the rows of a menu coming in', /\.glim-menu\s*>/, ['menu-step', 'menu-sway']],
  ['a window or dialog opening', /\.glim-modal-card\b/, ['win-dur', 'win-travel']],
  ['the rows of a folder coming in', /\.glim-stagger-row\b/, ['row-travel', 'row-sway']],
  ['a button giving way under the press', /\.glim-btn\b.*:active/, ['press-scale']],
  ['a panel waiting for a drop', /\.dragover\b/, ['dropzone-dur', 'dropzone-glow']],
  ['a row that just changed', /\.row\.fresh\b/, ['fresh-dur', 'fresh-glow']],
  ['the cannonball of a copy in the storm', /\.fx-ball\b/, ['cannon-dur', 'cannon-arc']],
  ['the cloud where it lands', /\.fx-puff\b/, ['puff-dur', 'puff-swell']],
  ['the smoke over an emptied trash', /\.fx-smoke\b/, ['smoke-dur', 'smoke-rise']],
  ['the salute', /\.fx-salute\b/, ['salute-dur', 'salute-burst', 'salute-step']],
  ['the head kicking back', /\.fx-recoil\b/, ['recoil-dur', 'recoil-dist']],
  ['the tumbleweed', /\.fx-weed\b/, ['tumble-dur', 'tumble-hop']],
];

const frames = new Map(all.flatMap((s) => keyframes(s.css).map((k) => [k.name, k.body] as const)));
const gated = all.flatMap((s) => rules(s.css).filter((r) => inside(gates(s.css), r.at)).map((r) => ({ ...r, file: s.file })));

// What a rule reads: its own body and the frames of every keyframe it plays.
function reads(body: string) {
  const played = [...body.matchAll(/animation(?:-name)?:\s*([^;]+)/g)].flatMap((m) => m[1]!.split(/[\s,]+/));
  return body + played.map((name) => frames.get(name) ?? '').join('');
}

describe('the moments that move', () => {
  for (const [moment, selector, names] of MOMENTS) {
    it(`read the level: ${moment}`, () => {
      const own = gated.filter((r) => selector.test(r.selector)).map((r) => reads(r.body)).join('\n');
      expect(own, `no rule inside the gate for ${moment}`).not.toBe('');
      expect(names.filter((n) => !own.includes(`var(--motion-${n}`))).toEqual([]);
    });
  }
});

describe('the levels', () => {
  const sizes = [...LEVELS.wild].filter(([name, v]) => !isTime(v) && !name.endsWith('-ease')).map(([name]) => name);

  it('have sizes to compare', () => {
    expect(sizes.length).toBeGreaterThanOrEqual(30);
  });

  it('move every size further from subtle to wild to the storm, each step clearly', () => {
    const flat = sizes.flatMap((name) => {
      const still = num(LEVELS.off.get(name)!);
      const [subtle, wild, storm] = (['subtle', 'wild', 'storm'] as const).map((l) => Math.abs(num(LEVELS[l].get(name)!) - still));
      return wild >= subtle * APART && wild > subtle && storm >= wild * APART ? [] : [`${name}: ${subtle} ${wild} ${storm}`];
    });
    expect(flat).toEqual([]);
  });

  it('quicken every pace from subtle to wild to the storm', () => {
    const slow = PACE.filter((name) => {
      const [subtle, wild, storm] = (['subtle', 'wild', 'storm'] as const).map((l) => num(LEVELS[l].get(name)!));
      return !(subtle > wild && wild > storm);
    });
    expect(slow).toEqual([]);
  });

  it('hold the moments that matter at subtle to a calm size', () => {
    // Subtle is the default, so its numbers are what most people see every day.
    expect(num(LEVELS.subtle.get('--motion-win-travel')!)).toBeLessThanOrEqual(8);
    expect(num(LEVELS.subtle.get('--motion-menu-travel')!)).toBeLessThanOrEqual(4);
    expect(num(LEVELS.subtle.get('--motion-spark-count')!)).toBe(0);
  });

  it('give the storm a real shake, and more pieces than wild by half again', () => {
    for (const name of ['--motion-spark-count', '--motion-trace-count', '--motion-drop-count', '--motion-salute-count']) {
      expect(num(LEVELS.storm.get(name)!), name).toBeGreaterThanOrEqual(num(LEVELS.wild.get(name)!) * 1.5);
    }
    expect(num(LEVELS.storm.get('--motion-jolt-dist')!)).toBeGreaterThanOrEqual(8);
    expect(num(LEVELS.storm.get('--motion-drop-tilt')!)).toBeGreaterThanOrEqual(180);
  });
});
