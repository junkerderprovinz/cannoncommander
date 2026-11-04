import { describe, expect, it } from 'vitest';
import { MB } from '@/core/format';
import { lineName, partSize, SUMFILE } from './sums';

describe('sum lists', () => {
  it('writes names relative to the folder of the list, and full paths outside it', () => {
    expect(lineName('ab12', '/mnt/user/isos/a.iso', '/mnt/user/isos')).toBe('ab12  a.iso\n');
    expect(lineName('ab12', '/mnt/user/isos/sub/b.iso', '/mnt/user/isos/')).toBe('ab12  sub/b.iso\n');
    expect(lineName('ab12', '/mnt/user/other/c.iso', '/mnt/user/isos')).toBe('ab12  /mnt/user/other/c.iso\n');
  });

  it('escapes a backslash or a line break the way coreutils does', () => {
    expect(lineName('ab12', '/x/a\\b\nc', '/x')).toBe('\\ab12  a\\\\b\\nc\n');
  });

  it('knows the lists it writes and reads', () => {
    expect(['a.sha256', 'b.MD5', 'SHA256SUMS', 'x.b3'].every((n) => SUMFILE.test(n))).toBe(true);
    expect(SUMFILE.test('notes.txt')).toBe(false);
  });

  it('reads the part size from the choice or the own field, with a comma as well', () => {
    expect(partSize({ size: '1024', custom: '' })).toBe(1024 * MB);
    expect(partSize({ size: 'custom', custom: '2,5' })).toBe(2.5 * MB);
    expect(partSize({ size: 'custom', custom: 'viel' })).toBe(0);
  });
});
