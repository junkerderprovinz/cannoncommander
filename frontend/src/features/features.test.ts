import { createPinia, setActivePinia } from 'pinia';
import { beforeAll, describe, expect, it, vi } from 'vitest';
import { actions } from '@/core/actions';
import { host } from '@/core/host';
import { prefs } from '@/core/prefs';
import { loadFeatures } from '@/features';
import { registerFileActions } from '@/files/actions';
import { usePanels } from '@/stores/panels';

const settle = () => new Promise((r) => setTimeout(r, 0));

beforeAll(() => {
  setActivePinia(createPinia());
  prefs.desktopStyle = 'commander';
  vi.stubGlobal('fetch', vi.fn(async () => new Response(null, { status: 404 })));
  usePanels();
  registerFileActions();
  Object.assign(host, { desktop: false, os: '' });
  loadFeatures();
});

describe('the features of the desktop app', () => {
  it('stay away while the server has not said it is the desktop app', async () => {
    await settle();
    expect(actions.launch).toBeUndefined();
    expect(actions.dropIn).toBeUndefined();
    expect(actions.download).toBeDefined();
  });

  it('come once it has', async () => {
    Object.assign(host, { desktop: true, os: 'windows' });
    await vi.waitFor(() => expect(actions.launch).toBeDefined());
    expect(actions.dropIn).toBeDefined();
    expect(actions.download).toBeUndefined();
  });
});
