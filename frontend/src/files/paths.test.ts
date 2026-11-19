import { describe, expect, it } from 'vitest';
import { nativePath, outsideArchive, stemLength, typedPath } from './paths';

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

  it('writes a share on the network as Windows does and takes it typed so', () => {
    expect(nativePath('/net/nas/daten/Filme', true)).toBe('\\\\nas\\daten\\Filme');
    expect(nativePath('/net/nas', true)).toBe('\\\\nas');
    expect(nativePath('/net', true)).toBe('\\\\');
    expect(nativePath('/net/nas/daten', false)).toBe('/net/nas/daten');
    expect(typedPath('\\\\nas\\daten\\Filme\\', windows)).toBe('/net/nas/daten/Filme');
    expect(typedPath('\\\\192.168.10.10\\code', windows)).toBe('/net/192.168.10.10/code');
    expect(typedPath('\\\\nas', windows)).toBe('/net/nas');
    expect(typedPath('\\\\', windows)).toBe('/net');
    expect(typedPath('//nas/daten', windows)).toBe('/net/nas/daten');
    expect(typedPath('file://nas/daten/a%20b', windows)).toBe('/net/nas/daten/a b');
  });

  it('leaves a path inside an archive as it is typed', () => {
    expect(typedPath('/mnt/user/a.zip::/docs', linux)).toBe('/mnt/user/a.zip::/docs');
  });
});

describe('the part of a name to type over', () => {
  it('stops before the last extension of a file', () => {
    expect(stemLength('Citizen Vigilante (2026).mkv', false)).toBe(24);
    expect(stemLength('Citizen.Vigilante.2026.German.DTSHD.DL.2160p.UHD.BluRay.DV.HDR.x265-VECTOR.mkv', false)).toBe(74);
    expect(stemLength('album.flac', false)).toBe(5);
    expect(stemLength('archiv.tar.gz', false)).toBe(10);
  });

  it('takes all of a folder, dots and all', () => {
    expect(stemLength('Citizen.Vigilante.2026.German.DTSHD.DL.2160p.UHD.BluRay.DV.HDR.x265-VECTOR', true)).toBe(74);
    expect(stemLength('Citizen Vigilante (2026)', true)).toBe(24);
  });

  it('takes all of a file without an extension', () => {
    expect(stemLength('nfo', false)).toBe(3);
    expect(stemLength('.bashrc', false)).toBe(7);
    expect(stemLength('Mr. Robot (2015)', false)).toBe(16);
    expect(stemLength('Citizen.Vigilante.2026.German.DTSHD.DL.2160p.UHD.BluRay.DV.HDR.x265-VECTOR', false)).toBe(74);
  });
});
