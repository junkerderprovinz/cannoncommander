import { createPinia } from 'pinia';
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import type { AppSettings, FoundOffice } from '@/api/admin';
import { addMessages, setLanguage } from '@/i18n';
import en from '../en.json';
import Integrations from './Integrations.vue';

type Sent = { method: string; path: string; body: Record<string, unknown> };
let sent: Sent[] = [];
let settings: AppSettings;
let found: { docker: boolean; servers: FoundOffice[] };
let probed: FoundOffice[];

const answer = (status: number, body?: unknown) =>
  new Response(body === undefined ? null : JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

async function server(url: string, init: RequestInit = {}) {
  const path = new URL(url, 'http://cc.test').pathname.replace('/api/v1', ''), method = init.method ?? 'GET';
  const body = init.body ? JSON.parse(String(init.body)) : {};
  if (method !== 'GET') sent.push({ method, path, body });
  if (path === '/admin/app' && method === 'GET') return answer(200, settings);
  if (path === '/admin/app') {
    settings.office = { ...settings.office, ...(body.office ?? {}) };
    return answer(200, settings);
  }
  if (path === '/admin/office/found') return answer(200, found);
  if (path === '/admin/office/probe') return answer(200, { servers: probed });
  if (path === '/admin/office/connect') {
    const s = found.servers.find((f) => f.id === body.id)!;
    settings.office = { kind: s.kind, url: s.url, secretSet: s.secret === 'found' };
    return answer(200, { result: { kind: s.kind, ok: true, reachable: true, token: 'ok' }, office: settings.office });
  }
  if (path === '/admin/integrations/test') return answer(200, { kind: body.kind, ok: false, reachable: true, token: 'missing' });
  return answer(404, { title: 'not found' });
}

beforeAll(() => {
  addMessages('config', { en });
  setLanguage('en');
});
beforeEach(() => {
  sent = [];
  settings = {
    office: { kind: 'none', url: '', secretSet: false },
    arrowloop: { url: '', tokenSet: false },
    previews: { thumbs: true, video: false }, webdav: true, terminal: false,
  } as unknown as AppSettings;
  found = {
    docker: true,
    servers: [
      { id: 'e1', kind: 'eurooffice', name: 'euro-office', url: 'http://tower.lan:9900', reachable: true, secret: 'found' },
      { id: 'c1', kind: 'collabora', name: 'collabora', url: 'https://tower.lan:9980', reachable: false, wopi: 'blocked', allow: 'http://tower.lan:8480' },
    ],
  };
  probed = [{ kind: 'onlyoffice', url: 'http://nas.lan:9900', reachable: true }];
  vi.stubGlobal('fetch', vi.fn(server));
  vi.stubGlobal('ResizeObserver', class { observe() {} disconnect() {} });
});

let app: ReturnType<typeof createApp> | undefined;
afterEach(() => {
  app?.unmount();
  document.body.innerHTML = '';
});

const settle = async () => {
  for (let i = 0; i < 8; i++) {
    await new Promise((r) => setTimeout(r, 0));
    await nextTick();
  }
};
async function open() {
  app = createApp(Integrations).use(createPinia());
  app.mount(document.body.appendChild(document.createElement('div')));
  await settle();
}
const rows = () => [...document.querySelectorAll<HTMLElement>('[data-found] [data-glim="ConnectionRow"]')];
const row = (name: string) => rows().find((r) => r.getAttribute('aria-label') === name)!;
const connectOf = (name: string) => row(name).querySelector<HTMLButtonElement>('[data-connection="connect"]');
const click = async (el: Element) => {
  (el as HTMLElement).click();
  await settle();
};

describe('office servers found on this server', () => {
  it('shows every suite Docker runs with its logo and address', async () => {
    await open();
    expect(rows().map((r) => r.getAttribute('aria-label'))).toEqual(['Euro-Office', 'Collabora Online']);
    expect(row('Euro-Office').querySelector('.glim-connection-status')?.textContent).toBe('http://tower.lan:9900');
    expect(row('Euro-Office').querySelector('.glim-connection-tile svg')).not.toBeNull();
    expect(row('Collabora Online').querySelector('.glim-connection-badges')?.textContent).toContain('No answer');
  });

  it('names the address Collabora still has to allow', async () => {
    await open();
    expect(row('Collabora Online').textContent).toContain('aliasgroup1');
    expect(row('Collabora Online').textContent).toContain('http://tower.lan:8480');
  });

  it('connects a container with one click and shows it as connected', async () => {
    await open();
    await click(connectOf('Euro-Office')!);
    expect(sent.at(-1)).toMatchObject({ path: '/admin/office/connect', body: { id: 'e1' } });
    expect(connectOf('Euro-Office')).toBeNull();
    expect(row('Euro-Office').querySelector('.glim-connection-badges')?.textContent).toContain('Connected');
    expect(document.querySelector<HTMLInputElement>('#a-oourl')?.value).toBe('http://tower.lan:9900');
  });

  it('without Docker asks a host the admin names and fills in what answered there', async () => {
    found = { docker: false, servers: [] };
    await open();
    const host = document.querySelector<HTMLInputElement>('[data-found] input')!;
    host.value = 'nas.lan';
    host.dispatchEvent(new Event('input'));
    await click(document.querySelector('[data-found] [data-probe]')!);
    expect(sent.at(-1)).toMatchObject({ path: '/admin/office/probe', body: { host: 'nas.lan' } });
    await click(connectOf('OnlyOffice')!);
    expect(sent.find((s) => s.path === '/admin/app')).toMatchObject({ body: { office: { kind: 'onlyoffice', url: 'http://nas.lan:9900' } } });
    expect(document.querySelector<HTMLInputElement>('#a-oourl')?.value).toBe('http://nas.lan:9900');
  });
});
