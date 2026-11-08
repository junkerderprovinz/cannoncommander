import type { MenuEntry as GlimEntry, MenuItem as GlimItem } from '@glimstone/vue';
import { createPinia, setActivePinia } from 'pinia';
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import type { Item } from '@/api/client';
import { host } from '@/core/host';
import { menuState, toGlim } from '@/core/menu';
import { toasts } from '@/core/toast';
import { winStack } from '@/core/windows';
import { loadDesktopFeatures, loadFeatures } from '@/features';
import { registerFileActions } from '@/files/actions';
import { rowMenu } from '@/files/menus';
import { setLanguage } from '@/i18n';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import ChooserBody from './ChooserBody.vue';

type Row = GlimItem;
const isRow = (e: GlimEntry): e is Row => e !== '-' && !('head' in e);

const HERE = '/C/Users/jdp/Documents';
const item = (name: string, dir = false): Item => ({ name, dir, size: 1, time: 0, mode: '0664', owner: 'jdp', group: 'jdp' });

let offered: { apps: unknown[]; dialog: boolean };
let posted: { url: string; body: unknown }[] = [];
// Only the calls for programs; other features read the folder too.
let asked: string[] = [];
function server(url: string, init?: RequestInit) {
  if (init?.method === 'POST') {
    posted.push({ url, body: JSON.parse(String(init.body)) });
    return new Response(null, { status: 204 });
  }
  if (url.startsWith('/api/v1/openwith?')) {
    asked.push(url);
    if (new URL(url, 'http://x').searchParams.get('every') === 'true') return Response.json({ apps: [{ id: 'gimp.desktop', name: 'GIMP' }, { id: 'calc.desktop', name: 'Rechner' }], dialog: false });
    return Response.json(offered);
  }
  return new Response(null, { status: 404 });
}

const settle = () => new Promise((r) => setTimeout(r, 0));

// Opens the row menu for the names given, marked in the folder, and returns the top rows once the programs are in.
async function menuFor(...names: string[]) {
  const F = usePanels();
  F.T().marked = new Set(names);
  rowMenu(0, 0);
  await settle();
  return toGlim(menuState.list ?? []).filter(isRow);
}
const openWithRow = (rows: Row[]) => rows.find((r) => r.label === 'Öffnen mit');

beforeAll(async () => {
  setLanguage('de');
  setActivePinia(createPinia());
  usePanels();
  registerFileActions();
  loadFeatures();
  await loadDesktopFeatures();
});

beforeEach(() => {
  const F = usePanels();
  F.T().path = HERE;
  useListing().lists.set(HERE, { path: HERE, items: [item('Brief.txt'), item('Bild.png'), item('Fotos', true), item('Akten.zip')] });
  Object.assign(host, { desktop: true, os: 'windows' });
  offered = {
    apps: [
      { id: 'C:\\Windows\\notepad.exe', name: 'Editor', icon: 'data:image/png;base64,AAAA', default: true },
      { id: 'C:\\Code\\Code.exe', name: 'Visual Studio Code' },
    ],
    dialog: true,
  };
  posted = [];
  asked = [];
  toasts.splice(0);
  winStack.splice(0);
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => server(url, init)));
});

describe('Öffnen mit', () => {
  it('stands below Ansehen, Bearbeiten and Öffnen for one file in the desktop app', async () => {
    const rows = await menuFor('Brief.txt');
    const labels = rows.map((r) => r.label);
    expect(labels.indexOf('Öffnen')).toBe(labels.indexOf('Bearbeiten') + 1);
    expect(labels.indexOf('Öffnen mit')).toBe(labels.indexOf('Öffnen') + 1);
    expect(asked).toEqual([`/api/v1/openwith?path=${encodeURIComponent(HERE + '/Brief.txt')}`]);
  });

  it('is not there in the container', async () => {
    host.desktop = false;
    expect(openWithRow(await menuFor('Brief.txt'))).toBeUndefined();
    expect(asked).toEqual([]);
  });

  it('is not there for a folder, as in Explorer', async () => {
    expect(openWithRow(await menuFor('Fotos'))).toBeUndefined();
  });

  it('is not there for several files at once, as in Explorer', async () => {
    expect(openWithRow(await menuFor('Brief.txt', 'Bild.png'))).toBeUndefined();
  });

  it('is not there for a file inside an archive', async () => {
    usePanels().T().path = HERE + '/Akten.zip::/';
    useListing().lists.set(HERE + '/Akten.zip::/', { path: HERE + '/Akten.zip::/', items: [item('Liste.txt')] });
    expect(openWithRow(await menuFor('Liste.txt'))).toBeUndefined();
  });

  it('lists the programs with their icons, then a way to pick another', async () => {
    const sub = openWithRow(await menuFor('Brief.txt'))!.sub!;
    expect(sub.map((r) => (isRow(r) ? r.label : r))).toEqual(['Editor', 'Visual Studio Code', '-', 'Andere App auswählen']);
    const [editor, code] = sub.filter(isRow);
    // The icon stands where the glyph would, the program without one keeps a glyph.
    const shown = (r: Row) => {
      const el = document.createElement('div');
      const app = createApp({ render: () => (r.glyph ? (r.glyph as () => unknown)() : null) });
      app.mount(el);
      const html = el.innerHTML;
      app.unmount();
      return html;
    };
    expect(shown(editor!)).toContain('<img src="data:image/png;base64,AAAA"');
    expect(shown(code!)).toContain('<svg');
  });

  it('says so when the computer offers no program, and still offers to pick one', async () => {
    offered = { apps: [], dialog: true };
    const sub = openWithRow(await menuFor('Brief.txt'))!.sub!.filter(isRow);
    expect(sub.map((r) => [r.label, !!r.disabled])).toEqual([['Keine App gefunden', true], ['Andere App auswählen', false]]);
  });

  it('opens the file with the program picked', async () => {
    const sub = openWithRow(await menuFor('Brief.txt'))!.sub!.filter(isRow);
    sub[1]!.onSelect!();
    await settle();
    expect(posted).toEqual([{ url: '/api/v1/openwith', body: { path: HERE + '/Brief.txt', app: 'C:\\Code\\Code.exe', every: false } }]);
  });

  it('leaves another program to the dialog of the system that has one', async () => {
    const sub = openWithRow(await menuFor('Bild.png'))!.sub!.filter(isRow);
    sub.at(-1)!.onSelect!();
    await settle();
    expect(posted).toEqual([{ url: '/api/v1/openwith/dialog', body: { path: HERE + '/Bild.png' } }]);
    expect(winStack).toHaveLength(0);
  });

  it('lists every program where the system has no dialog, as on Linux', async () => {
    offered = { apps: [], dialog: false };
    const sub = openWithRow(await menuFor('Bild.png'))!.sub!.filter(isRow);
    sub.at(-1)!.onSelect!();
    expect(winStack).toHaveLength(1);
    const el = document.body.appendChild(document.createElement('div'));
    let closed = false;
    const app = createApp(ChooserBody, { path: HERE + '/Bild.png', close: () => { closed = true; } });
    app.mount(el);
    await settle();
    await nextTick();
    const rows = [...el.querySelectorAll<HTMLElement>('[data-app]')];
    expect(rows.map((r) => r.dataset.app)).toEqual(['gimp.desktop', 'calc.desktop']);
    rows[0]!.click();
    await settle();
    expect(closed).toBe(true);
    expect(posted).toEqual([{ url: '/api/v1/openwith', body: { path: HERE + '/Bild.png', app: 'gimp.desktop', every: true } }]);
    app.unmount();
    el.remove();
  });

  it('shows why the programs could not be listed', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => Response.json({ status: 403, code: 'denied' }, { status: 403, headers: { 'Content-Type': 'application/problem+json' } })));
    const sub = openWithRow(await menuFor('Brief.txt'))!.sub!.filter(isRow);
    expect(sub).toHaveLength(1);
    expect(sub[0]!.disabled).toBe(true);
    expect(sub[0]!.label).toBe('Dafür fehlen dir die Rechte');
  });
});
