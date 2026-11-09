import { beforeAll, describe, expect, it } from 'vitest';
import { addMessages, setLanguage } from '@/i18n';
import { driveName, freeText } from './drives';

beforeAll(() => {
  setLanguage('de');
  addMessages('files', { de: { free: '{size} frei', freeOn: '{name}: {size} frei' } });
  addMessages('drives', { de: { fixed: 'Lokaler Datenträger', removable: 'USB-Laufwerk', optical: 'DVD-Laufwerk', network: 'Netzlaufwerk', volume: 'Datenträger mit {size}', shareOn: '{share} auf {server}' } });
});

describe('a drive', () => {
  it('goes by its label and letter as in Explorer', () => {
    expect(driveName({ path: '/C', label: 'Windows', kind: 'fixed' })).toBe('Windows (C:)');
    expect(driveName({ path: '/E', kind: 'removable' })).toBe('USB-Laufwerk (E:)');
    expect(driveName({ path: '/F', kind: 'virtual' })).toBe('Lokaler Datenträger (F:)');
    expect(driveName({ path: '/D', kind: 'optical', size: 4e9 })).toBe('DVD-Laufwerk (D:)');
  });

  it('names a network drive after its share and server', () => {
    expect(driveName({ path: '/Z', kind: 'network', remote: '\\\\nas\\daten', label: 'Archiv' })).toBe('daten (\\\\nas) (Z:)');
    expect(driveName({ path: '/run/user/1000/gvfs/smb-share:server=localhost,share=ccshare', kind: 'network', remote: 'smb://localhost/ccshare', label: 'ccshare' })).toBe('ccshare auf localhost');
    expect(driveName({ kind: 'network', remote: 'sftp://jdp@pi.local', label: 'pi.local' })).toBe('pi.local');
    expect(driveName({ kind: 'network', remote: 'sftp://pi.local' })).toBe('sftp://pi.local');
  });

  it('without a label or a letter goes by its size, as on Linux', () => {
    expect(driveName({ kind: 'fixed', size: 16 * 2 ** 30 })).toMatch(/^Datenträger mit 16/);
    expect(driveName({ path: '/run/media/jdp/URLAUB', kind: 'removable', label: 'URLAUB' })).toBe('URLAUB');
  });
});

describe('the free space in the footer', () => {
  it('names a drive of the computer and leaves a source of the server unnamed', () => {
    expect(freeText({ path: '/run/user/1000/gvfs/smb-share:server=localhost,share=ccshare', label: 'ccshare auf localhost', kind: 'network', free: 22 * 2 ** 30, total: 40 * 2 ** 30 })).toMatch(/^ccshare auf localhost: 22.* frei$/);
    expect(freeText({ path: '/mnt/user', label: 'Shares', free: 2 ** 30, total: 2 ** 31 })).toMatch(/^1.* frei$/);
  });
});
