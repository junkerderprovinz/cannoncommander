import { describe, expect, it } from 'vitest';
import { BROADSIDE, broadside, KONAMI, onceEach, sequence, typedWord } from './cues';

const press = (keys: string[], fn: (k: string) => boolean) => keys.map(fn);

describe('a key sequence', () => {
  it('fires on its last key and not before', () => {
    const hits = press(KONAMI, sequence(KONAMI));
    expect(hits.slice(0, -1).every((x) => !x)).toBe(true);
    expect(hits.at(-1)).toBe(true);
  });

  it('takes the letters in either case', () => {
    const keys = [...KONAMI.slice(0, -2), 'B', 'A'];
    expect(press(keys, sequence(KONAMI)).at(-1)).toBe(true);
  });

  it('still fires after a stray key in front of it', () => {
    const keys = ['ArrowUp', 'x', 'ArrowUp', ...KONAMI];
    expect(press(keys, sequence(KONAMI)).at(-1)).toBe(true);
  });

  it('fires when the start overlaps, as with three ups in a row', () => {
    expect(press(['ArrowUp', ...KONAMI], sequence(KONAMI)).at(-1)).toBe(true);
  });

  it('starts over once it fired, so one more a does nothing', () => {
    const next = sequence(KONAMI);
    press(KONAMI, next);
    expect(next('a')).toBe(false);
  });

  it('breaks on a wrong key in the middle', () => {
    const keys = [...KONAMI.slice(0, 5), 'Enter', ...KONAMI.slice(5)];
    expect(press(keys, sequence(KONAMI)).some(Boolean)).toBe(false);
  });
});

describe('a typed word', () => {
  const kaboom = typedWord('kaboom');

  it('fires when the text grows into the word', () => {
    expect(kaboom('kaboo', 'kaboom')).toBe(true);
  });

  it('fires at the end of a longer text in any case', () => {
    expect(kaboom('bilder KaBoo', 'bilder KaBoom')).toBe(true);
  });

  it('stays quiet while the text shrinks back onto the word', () => {
    expect(kaboom('kabooms', 'kaboom')).toBe(false);
  });

  it('stays quiet for anything else', () => {
    expect(kaboom('kab', 'kabo')).toBe(false);
    expect(kaboom('', 'boom')).toBe(false);
  });
});

describe('once each', () => {
  it('lets a key through the first time only', () => {
    const take = onceEach();
    expect([take('0:1'), take('0:1'), take('0:2'), take('1:1')]).toEqual([true, false, true, true]);
  });
});

describe('the broadside', () => {
  it('needs the storm and a hundred entries', () => {
    expect(BROADSIDE).toBe(100);
    expect(broadside('storm', 100)).toBe(true);
    expect(broadside('storm', 99)).toBe(false);
    expect(broadside('wild', 500)).toBe(false);
  });
});
