import { beforeAll, describe, expect, it } from 'vitest';
import { addMessages, setLanguage } from '@/i18n';
import { driveName } from './drives';

beforeAll(() => {
  setLanguage('de');
  addMessages('drives', { de: { fixed: 'Lokaler Datenträger', removable: 'USB-Laufwerk', optical: 'DVD-Laufwerk', network: 'Netzlaufwerk', volume: 'Datenträger mit {size}' } });
});

describe('a drive', () => {
  it('goes by its label and letter as in Explorer', () => {
    expect(driveName({ path: '/C', label: 'Windows', kind: 'fixed' })).toBe('Windows (C:)');
    expect(driveName({ path: '/E', kind: 'removable' })).toBe('USB-Laufwerk (E:)');
    expect(driveName({ path: '/F', kind: 'virtual' })).toBe('Lokaler Datenträger (F:)');
    expect(driveName({ path: '/D', kind: 'optical', size: 4e9 })).toBe('DVD-Laufwerk (D:)');
  });

  it('names a network drive after its share and server', () => {
    expect(driveName({ path: '/Z', kind: 'network', remote: '\\\\nas\\daten' })).toBe('daten (\\\\nas) (Z:)');
    expect(driveName({ path: '/run/user/1000/gvfs/x', kind: 'network', remote: 'smb://nas/daten', label: 'daten' })).toBe('daten');
    expect(driveName({ kind: 'network', remote: 'sftp://pi.local' })).toBe('sftp://pi.local');
  });

  it('without a label or a letter goes by its size, as on Linux', () => {
    expect(driveName({ kind: 'fixed', size: 16 * 2 ** 30 })).toMatch(/^Datenträger mit 16/);
    expect(driveName({ path: '/run/media/jdp/URLAUB', kind: 'removable', label: 'URLAUB' })).toBe('URLAUB');
  });
});
