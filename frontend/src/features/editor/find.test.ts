import { describe, expect, it } from 'vitest';
import { findAll, nearest, position } from './find';

describe('find and replace', () => {
  const text = 'Disk 2 zeigt Sektoren.\nDisk 3 schläft, disk 4 nicht.';

  it('finds every match in any case, one after the other', () => {
    expect(findAll(text, 'disk')).toEqual([[0, 4], [23, 27], [39, 43]]);
    expect(findAll(text, '')).toEqual([]);
    expect(findAll('aaaa', 'aa')).toEqual([[0, 2], [2, 4]]);
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
