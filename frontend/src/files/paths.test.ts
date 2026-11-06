import { describe, expect, it } from 'vitest';
import { outsideArchive } from './paths';

describe('paths', () => {
  it('finds the folder that holds the archive a path lies in', () => {
    expect(outsideArchive('/mnt/user/isos/pack.zip::')).toBe('/mnt/user/isos');
    expect(outsideArchive('/mnt/user/isos/pack.zip::/docs/a')).toBe('/mnt/user/isos');
    expect(outsideArchive('/mnt/user/isos')).toBe('/mnt/user/isos');
  });
});
