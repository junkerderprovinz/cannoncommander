import { describe, expect, it } from 'vitest';
import { nativePath, outsideArchive, typedPath } from './paths';

const windows = { windows: true, home: '/C/Users/jdp' };
const linux = { windows: false, home: '/home/jdp' };

describe('paths', () => {
  it('finds the folder that holds the archive a path lies in', () => {
    expect(outsideArchive('/mnt/user/isos/pack.zip::')).toBe('/mnt/user/isos');
    expect(outsideArchive('/mnt/user/isos/pack.zip::/docs/a')).toBe('/mnt/user/isos');
    expect(outsideArchive('/mnt/user/isos')).toBe('/mnt/user/isos');
  });

  it('writes a path on a Windows drive the way Windows does', () => {
    expect(nativePath('/C/Users/jdp/Downloads', true)).toBe('C:\\Users\\jdp\\Downloads');
    expect(nativePath('/D', true)).toBe('D:\\');
    expect(nativePath('/', true)).toBe('/');
    expect(nativePath('/C/Users/jdp', false)).toBe('/C/Users/jdp');
    expect(nativePath('/home/jdp/Bilder', false)).toBe('/home/jdp/Bilder');
  });

  it('takes a typed Windows path in any of its usual forms', () => {
    const forms = ['C:\\Users\\jdp\\Downloads', 'c:/Users/jdp/Downloads\\', '"C:\\Users\\jdp\\Downloads"', ' /C/Users/jdp/Downloads ',
      'file:///C:/Users/jdp/Downloads', '%USERPROFILE%\\Downloads', '~/Downloads'];
    for (const typed of forms) expect(typedPath(typed, windows), typed).toBe('/C/Users/jdp/Downloads');
    expect(typedPath('D:', windows)).toBe('/D');
    expect(typedPath('D:\\', windows)).toBe('/D');
    expect(typedPath('file:///C:/Program%20Files', windows)).toBe('/C/Program Files');
  });

  it('takes a typed Linux path with the home folder and file URIs', () => {
    expect(typedPath('~', linux)).toBe('/home/jdp');
    expect(typedPath('~/Bilder/', linux)).toBe('/home/jdp/Bilder');
    expect(typedPath('$HOME/Bilder', linux)).toBe('/home/jdp/Bilder');
    expect(typedPath('file:///home/jdp/Neuer%20Ordner', linux)).toBe('/home/jdp/Neuer Ordner');
    expect(typedPath('/mnt/user/isos', linux)).toBe('/mnt/user/isos');
    expect(typedPath('', linux)).toBe('/');
  });

  it('leaves a path inside an archive as it is typed', () => {
    expect(typedPath('/mnt/user/a.zip::/docs', linux)).toBe('/mnt/user/a.zip::/docs');
  });
});
