import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { nextTick } from 'vue';
import { actions } from '@/core/actions';
import { host } from '@/core/host';
import { prefs } from '@/core/prefs';
import { toasts } from '@/core/toast';
import { winStack } from '@/core/windows';
import { addMessages, setLanguage } from '@/i18n';
import { usePanels } from '@/stores/panels';
import de from './de.json';
import en from './en.json';
import setup from './index';
import { launch } from './launch';

addMessages('desktop', { de, en });

let posted: { path: string; run: boolean }[] = [];
let answer: Response | null = null;
function server(url: string, init?: RequestInit) {
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
  Object.assign(host, { desktop: true, os: 'linux' });
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => server(url, init)));
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
    await F.open(0, { name: 'brief.odt', kind: 'doc' });
    await settle();
    expect(posted).toEqual([{ path: '/home/jdp/Briefe/brief.odt', run: false }]);
    prefs.dblclick = 'open';
  });

  it('has no launcher in the container', async () => {
    host.desktop = false;
    setup();
    await nextTick();
    expect(actions.launch).toBeUndefined();
  });
});
