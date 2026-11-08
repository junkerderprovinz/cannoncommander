import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api, toItem } from '@/api/client';
import { actions, labelOf, run } from '@/core/actions';
import { host } from '@/core/host';
import { usePanels } from '@/stores/panels';
import { useUser } from '@/stores/user';
import { createFixture } from '../../scripts/fixture.js';
import { registerFileActions } from './actions';

const fx = createFixture();

beforeEach(() => {
  setActivePinia(createPinia());
  vi.restoreAllMocks();
  history.replaceState(null, '', '/');
  vi.spyOn(api, 'fsList').mockImplementation(async (path) => ({ path, items: (fx.list(path)?.items ?? []).map(toItem), etag: path }));
  vi.spyOn(api, 'fsTree').mockResolvedValue({ path: '/', items: [] });
  Object.assign(host, { desktop: false, os: '' });
});

function homeGoesTo() {
  useUser().me = { name: 'jdp', full: '', role: 'admin', home: '/C/Users/jdp' };
  const F = usePanels();
  const go = vi.spyOn(F, 'go').mockResolvedValue(true);
  registerFileActions();
  run('home');
  return go.mock.calls[0]?.[0];
}

describe('file actions', () => {
  it('takes home to the shares on the server', () => {
    expect(homeGoesTo()).toBe('/mnt/user');
    expect(labelOf(actions.home!)).toBe('Zu den Shares');
  });

  it('takes home to the home folder in the desktop app', () => {
    Object.assign(host, { desktop: true, os: 'windows' });
    expect(homeGoesTo()).toBe('/C/Users/jdp');
    expect(labelOf(actions.home!)).toBe('Zum persönlichen Ordner');
  });
});
