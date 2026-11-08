import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { addMessages, setLanguage } from '@/i18n';
import { host, loadHost, rootName } from './host';

beforeAll(() => {
  setLanguage('de');
  addMessages('files', { de: { server: 'Server', thisPc: 'Dieser PC', computer: 'Computer' } });
});
afterEach(() => {
  Object.assign(host, { desktop: false, os: '' });
  vi.unstubAllGlobals();
});

describe('the top of the path', () => {
  it('is the server in the container, whatever the system', () => {
    Object.assign(host, { desktop: false, os: 'windows' });
    expect(rootName()).toBe('Server');
  });

  it('is this PC on Windows and the computer elsewhere in the desktop app', () => {
    Object.assign(host, { desktop: true, os: 'windows' });
    expect(rootName()).toBe('Dieser PC');
    for (const os of ['darwin', 'linux']) {
      Object.assign(host, { desktop: true, os });
      expect(rootName()).toBe('Computer');
    }
  });

  it('learns where it runs from the server', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => Response.json({ deployment: 'desktop', os: 'windows', started: '2026-01-01T00:00:00Z' })));
    await loadHost();
    expect(host).toEqual({ desktop: true, os: 'windows' });
  });
});
