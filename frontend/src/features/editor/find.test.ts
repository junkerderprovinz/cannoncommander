import { describe, expect, it } from 'vitest';
import { findAll, nearest, pattern, position, replacement } from './find';

describe('find and replace', () => {
  const text = 'Disk 2 zeigt Sektoren.\nDisk 3 schläft, disk 4 nicht.';

  it('finds every match in any case, one after the other', () => {
    expect(findAll(text, 'disk')).toEqual([[0, 4], [23, 27], [39, 43]]);
    expect(findAll(text, '')).toEqual([]);
    expect(findAll('aaaa', 'aa')).toEqual([[0, 2], [2, 4]]);
  });

  it('keeps to the case and to whole words when asked', () => {
    expect(findAll(text, 'disk', { matchCase: true })).toEqual([[39, 43]]);
    expect(findAll('disk disks diskette', 'disk', { word: true })).toEqual([[0, 4]]);
    expect(findAll('a.b axb', 'a.b')).toEqual([[0, 3]]);
  });

  it('finds with a regular expression, line by line', () => {
    expect(findAll(text, 'disk \\d', { regex: true })).toEqual([[0, 6], [23, 29], [39, 45]]);
    expect(findAll('eins\nzwei', '^\\w', { regex: true })).toEqual([[0, 1], [5, 6]]);
    expect(findAll('ab', '^', { regex: true })).toEqual([]);
    expect(findAll(text, '(unclosed', { regex: true })).toEqual([]);
    expect(pattern('(unclosed', { regex: true })).toBeNull();
  });

  it('brings groups, line breaks and tabs into the replacement', () => {
    const t = 'width=10, height=20', re = '(\\w+)=(\\d+)', o = { regex: true };
    const [m] = findAll(t, re, o);
    expect(replacement(t, m!, re, '$2 $1', o)).toBe('10 width');
    expect(replacement(t, m!, re, '\\1:\\t\\2\\n', o)).toBe('width:\t10\n');
    expect(replacement(t, m!, re, '[$&] $$', o)).toBe('[width=10] $');
    expect(replacement(t, m!, re, '$2', {})).toBe('$2');
  });

  it('starts at the first match after the caret and wraps to the first one', () => {
    const m = findAll(text, 'disk');
    expect(nearest(m, 0)).toBe(0);
    expect(nearest(m, 24)).toBe(2);
    expect(nearest([], 5)).toBe(-1);
  });

  it('tells line and column of a position, counted from one', () => {
    expect(position(text, 0)).toEqual({ line: 1, col: 1 });
    expect(position(text, 25)).toEqual({ line: 2, col: 3 });
  });
});
