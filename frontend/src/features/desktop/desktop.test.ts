import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { nextTick } from 'vue';
import { api, toItem } from '@/api/client';
import { action, actions } from '@/core/actions';
import { menu } from '@/core/menu';
import { host } from '@/core/host';
import { prefs } from '@/core/prefs';
import { toasts } from '@/core/toast';
import { winStack } from '@/core/windows';
import { addMessages, setLanguage } from '@/i18n';
import { rowMenu } from '@/files/menus';
import { usePanels } from '@/stores/panels';
import { useUser } from '@/stores/user';
import keysDe from '@/features/keys/de.json';
import keysEn from '@/features/keys/en.json';
import de from './de.json';
import en from './en.json';
import setup from './index';
import { launch } from './launch';
import { fileManager, followFileManager, setFileManager } from './filemanager';
import { openHandedOver, takeStartFolders } from './open';
import { askStyle } from './style';

addMessages('desktop', { de, en });
addMessages('keys', { de: keysDe, en: keysEn });
vi.mock('@/core/menu', async (original) => ({ ...(await original<typeof import('@/core/menu')>()), menu: vi.fn() }));

let posted: { path: string; run: boolean }[] = [];
let answer: Response | null = null;
let switched: { on: boolean; words: Record<string, string> }[] = [];
let managerOn = false;
function server(url: string, init?: RequestInit) {
  if (url === '/api/v1/filemanager') {
    if (init?.method === 'PUT') {
      const body = JSON.parse(String(init.body));
      switched.push(body);
      managerOn = body.on;
    }
    return Response.json({ on: managerOn });
  }
  if (url === '/api/v1/launch') {
    posted.push(JSON.parse(String(init?.body)));
    return answer ?? new Response(null, { status: 204 });
  }
  if (url.startsWith('/api/v1/fs/list')) return Response.json({ path: '/', items: [] });
  return new Response(null, { status: 404 });
}
const problem = (status: number, code: string) => Response.json({ status, code }, { status, headers: { 'Content-Type': 'application/problem+json' } });
const settle = () => new Promise((r) => setTimeout(r, 0));

beforeEach(() => {
  setActivePinia(createPinia());
  setLanguage('de');
  toasts.splice(0);
  winStack.splice(0);
  posted = [];
  answer = null;
  switched = [];
  managerOn = false;
  Object.assign(host, { desktop: true, os: 'linux' });
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => server(url, init)));
  const file = (name: string) => toItem({ name, dir: false, size: 1, mtime: '2026-11-08T10:00:00Z', mode: '644' });
  vi.spyOn(api, 'fsList').mockImplementation(async (path) => ({ path, items: [file('a.txt'), file('b.txt')], etag: path }));
  vi.spyOn(api, 'fsTree').mockResolvedValue({ path: '/', items: [] });
  useUser().me = { name: 'jdp', full: '', role: 'admin', home: '/C/Users/jdp' };
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  Object.assign(host, { desktop: false, os: '' });
});

describe('launch', () => {
  it('hands the file to the computer', async () => {
    await launch('/home/jdp/Briefe/brief.odt');
    expect(posted).toEqual([{ path: '/home/jdp/Briefe/brief.odt', run: false }]);
    expect(winStack).toHaveLength(0);
  });

  it('asks before it starts a program, and starts it once told to', async () => {
    answer = problem(409, 'askRun');
    await launch('/home/jdp/bin/backup.sh');
    expect(winStack.map((w) => w.title)).toEqual(['Programm starten?']);
    expect(String(winStack[0]!.body)).toContain('<b>backup.sh</b>');
    answer = null;
    winStack[0]!.foot.find((f) => f.label === 'Starten')!.run!(document.body);
    await settle();
    expect(posted.at(-1)).toEqual({ path: '/home/jdp/bin/backup.sh', run: true });
  });

  it('says so when the file did not open', async () => {
    answer = problem(502, 'notStarted');
    await launch('/home/jdp/x.weird');
    expect(toasts.at(-1)?.text).toBe('Die Datei ließ sich nicht öffnen');
  });
});

describe('double click', () => {
  it('opens a file in its program when the setting says so', async () => {
    setup();
    await nextTick();
    expect(actions.launch).toBeDefined();
    prefs.dblclick = 'system';
    const F = usePanels();
    F.T().path = '/home/jdp/Briefe';
    await F.open(0, { name: 'brief.odt', kind: 'office' });
    await settle();
    expect(posted).toEqual([{ path: '/home/jdp/Briefe/brief.odt', run: false }]);
    prefs.dblclick = 'open';
  });

  it('offers Öffnen for a file in its context menu', async () => {
    setup();
    const F = usePanels();
    await settle();
    F.T().cursor = F.entries().findIndex((k) => k.name === 'a.txt');
    rowMenu(10, 10);
    expect(vi.mocked(menu).mock.calls.at(-1)![2]).toContain('launch');
  });

  it('drops download and upload once it knows it is the desktop app', async () => {
    host.desktop = false;
    prefs.desktopStyle = 'commander';
    for (const id of ['download', 'upload']) action(id, { label: id, run: () => {} });
    setup();
    host.desktop = true;
    await nextTick();
    expect(actions.download).toBeUndefined();
    expect(actions.upload).toBeUndefined();
  });

  it('has no launcher in the container', async () => {
    host.desktop = false;
    setup();
    await nextTick();
    expect(actions.launch).toBeUndefined();
  });
});

describe('folders handed over', () => {
  it('open in a new tab of the active panel with the cursor on the file', async () => {
    host.os = 'windows';
    const F = usePanels();
    const before = F.P().tabs.length;
    await openHandedOver([{ folder: 'C:\\Users\\jdp\\Downloads', select: 'b.txt' }, {}]);
    const tabs = F.P().tabs.slice(before);
    expect(tabs.map((t) => t.path)).toEqual(['/C/Users/jdp/Downloads', '/C/Users/jdp']);
    expect(tabs[0]!.cursor).toBe(1);
    expect(tabs[0]!.marked.size).toBe(0);
    expect(F.P().cur).toBe(F.P().tabs.length - 1);
  });

  it('mark the file that was named when the selection works as in Explorer', async () => {
    host.os = 'windows';
    prefs.clickSelect = true;
    const F = usePanels();
    await openHandedOver([{ folder: 'C:\\Users\\jdp\\Downloads', select: 'a.txt' }]);
    expect([...F.T().marked]).toEqual(['a.txt']);
    prefs.clickSelect = false;
  });

  it('come once with the start of the window and leave its address', () => {
    history.replaceState(null, '', '/?open=' + encodeURIComponent(JSON.stringify([{ folder: '/home/jdp/Bilder' }])) + '#/home/jdp');
    expect(takeStartFolders()).toEqual([{ folder: '/home/jdp/Bilder' }]);
    expect(location.search).toBe('');
    expect(location.hash).toBe('#/home/jdp');
    expect(takeStartFolders()).toEqual([]);
  });
});

describe('default file manager', () => {
  it('switches on with the words of the window and back off', async () => {
    await setFileManager(true);
    expect(fileManager.on).toBe(true);
    expect(switched[0]).toEqual({ on: true, words: { open: 'In CannonCommander öffnen', show: 'CannonCommander zeigen', quit: 'Beenden' } });
    expect(toasts.at(-1)?.text).toBe('Schön, deine Ordner öffnen sich jetzt in CannonCommander');
    await setFileManager(false);
    expect(fileManager.on).toBe(false);
    expect(toasts.at(-1)?.text).toBe('Deine Ordner öffnen sich wieder wie vorher');
  });

  it('writes the entry again at every start while it is on, and only then', async () => {
    await followFileManager();
    expect(switched).toEqual([]);
    managerOn = true;
    await followFileManager();
    expect(switched.map((s) => s.on)).toEqual([true]);
    expect(toasts).toHaveLength(0);
  });
});

describe('first start', () => {
  const answer = (label: string) => {
    const w = winStack.at(-1)!;
    w.foot.find((f) => f.label === label)!.run!(document.body);
    w.close();
  };

  it('asks once how the app should handle and opens files in their programs', () => {
    Object.assign(prefs, { desktopStyle: '', clickSelect: false, dblclick: 'open', keys: null });
    action('sidePane', { label: 'Vorschaubereich', run: () => {} });
    askStyle();
    expect(winStack.map((w) => w.title)).toEqual(['Wie im Explorer oder wie ein Commander?']);
    expect(prefs.dblclick).toBe('system');
    answer('Wie im Explorer');
    expect(prefs).toMatchObject({ desktopStyle: 'explorer', clickSelect: true });
    expect((prefs.keys as { preset: string }).preset).toBe('explorer');
    expect(actions.sidePane!.key).toBe('Alt+P');
  });

  it('stays a commander when the question is closed without an answer', () => {
    Object.assign(prefs, { desktopStyle: '', clickSelect: true, keys: null });
    askStyle();
    winStack.at(-1)!.close();
    expect(prefs).toMatchObject({ desktopStyle: 'commander', clickSelect: false, keys: null });
  });
});
