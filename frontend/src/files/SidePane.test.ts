import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import { api, toItem } from '@/api/client';
import { actions, run } from '@/core/actions';
import { prefs } from '@/core/prefs';
import { usePanels } from '@/stores/panels';
import { registerFileActions } from './actions';
import SidePane from './SidePane.vue';

const dir = (name: string) => toItem({ name, dir: true, size: 0, mtime: '2026-11-08T10:00:00Z', mode: '755' });
let app: ReturnType<typeof createApp> | undefined;

beforeEach(() => {
  const pinia = createPinia();
  setActivePinia(pinia);
  history.replaceState(null, '', '/#/home/jdp');
  vi.spyOn(api, 'fsList').mockImplementation(async (path) => ({ path, items: [dir('Bilder')], etag: path }));
  vi.spyOn(api, 'fsTree').mockResolvedValue({ path: '/', items: [] });
  vi.stubGlobal('fetch', vi.fn(async () => new Response(null, { status: 404 })));
  app = createApp(SidePane).use(pinia);
});
afterEach(() => {
  app?.unmount();
  document.body.innerHTML = '';
  prefs.sidePane = false;
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('preview pane', () => {
  it('comes and goes with its action', () => {
    registerFileActions();
    expect(actions.sidePane!.checked!()).toBe(false);
    run('sidePane');
    expect(prefs.sidePane).toBe(true);
    run('sidePane');
    expect(prefs.sidePane).toBe(false);
  });

  it('asks for a file while the cursor stands on the way up', async () => {
    const F = usePanels();
    await new Promise((r) => setTimeout(r, 0));
    F.T().cursor = F.entries().findIndex((k) => k.up);
    app!.mount(document.body.appendChild(document.createElement('div')));
    await nextTick();
    expect(document.body.textContent).toContain('Wähle eine Datei, dann zeigt sie sich hier');
  });

  it('shows the folder under the cursor with its details', async () => {
    const F = usePanels();
    await new Promise((r) => setTimeout(r, 0));
    F.T().cursor = F.entries().findIndex((k) => k.name === 'Bilder');
    app!.mount(document.body.appendChild(document.createElement('div')));
    await nextTick();
    expect(document.querySelector('.cv-details')?.textContent).toContain('Bilder');
  });
});
