import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import { addMessages } from '@/i18n';
import de from '../de.json';
import { takeArrivedFromUrl } from '../arrived';
import Security from './Security.vue';

const { toasts } = vi.hoisted(() => ({ toasts: [] as [string, string][] }));
vi.mock('@/core/toast', () => ({ toast: (text: string, kind = 'ok') => toasts.push([text, kind]) }));

const GET: Record<string, unknown> = {
  options: { open: false, owner: false, minPassword: 12, signedIn: true, passkeys: true, services: [] },
  me: { account: '1', name: 'jdp', role: 'admin', via: 'password', owner: true, password: true, twoFactor: false, recoveryLeft: 0, fresh: true, askPassword: true },
  settings: { askPassword: true },
  'me/sessions': { sessions: [{ id: 's1', via: 'password', created: '2026-11-21T10:00:00Z', seen: '2026-11-21T12:00:00Z', agent: 'Mozilla/5.0 (Windows NT 10.0) Firefox/150.0', from: '192.168.10.16', current: true }] },
  'me/tokens': { tokens: [] },
  'me/passkeys': { passkeys: [] },
  'methods/oidc': { services: [{ id: 'config', provider: 'custom', name: 'OIDC', issuer: 'https://id.example', clientId: 'cc', secretSet: true, scopes: 'openid', nameClaim: 'preferred_username', groupsClaim: 'groups', ca: false, on: true, paused: false, tested: true, redirect: 'https://cc.example/api/auth/oidc/callback', fromConfig: true }] },
  'methods/presets': { presets: [] },
};
const asked: string[] = [];

beforeAll(() => {
  addMessages('config', { de });
  vi.stubGlobal('ResizeObserver', class { observe() {} disconnect() {} });
});
beforeEach(() => {
  asked.length = 0;
  toasts.length = 0;
  vi.stubGlobal('fetch', async (url: string, init: RequestInit = {}) => {
    const path = url.replace('/api/signin/v1/', '');
    asked.push(`${init.method ?? 'GET'} ${path}`);
    const body = GET[path];
    return new Response(JSON.stringify(body ?? { error: 'notFound' }), { status: body ? 200 : 404, headers: { 'Content-Type': 'application/json' } });
  });
});
let app: ReturnType<typeof createApp> | undefined;
afterEach(() => {
  app?.unmount();
  document.body.innerHTML = '';
  history.replaceState(null, '', '/');
});

const settle = async () => {
  for (let i = 0; i < 8; i++) {
    await new Promise((r) => setTimeout(r, 0));
    await nextTick();
  }
};
async function mount() {
  app = createApp(Security);
  app.mount(document.body.appendChild(document.createElement('div')));
  await settle();
}
const cards = () => [...document.querySelectorAll<HTMLElement>('[data-signin-card]')].map((c) => c.dataset.signinCard);

describe('the account and security tab', () => {
  it('draws the sign-in tile without the accounts, which the Users tab manages', async () => {
    await mount();
    expect(cards()).toEqual(['password', 'askPassword', 'twoFactor', 'passkeys', 'sessions', 'tokens', 'services']);
    expect(asked).not.toContain('GET accounts');
  });

  it('shows the service of config.yaml among the sign-in services and says where it is changed', async () => {
    await mount();
    const services = document.querySelector('[data-signin-card="services"]')!;
    expect(services.textContent).toContain('OIDC');
    expect(services.textContent).toContain('https://id.example');
    expect(document.body.innerHTML).toContain('ändern kannst du ihn nur in dieser Datei');
  });

  it('names a session by its browser and system', async () => {
    await mount();
    expect(document.querySelector('[data-signin-card="sessions"]')?.textContent).toContain('Firefox auf Windows');
  });

  it('says once what a test sign-in came back with', async () => {
    history.replaceState(null, '', '/?signin=testOk');
    expect(takeArrivedFromUrl()).toBe(true);
    expect(location.search).toBe('');
    await mount();
    expect(toasts).toEqual([['Die Testanmeldung hat geklappt, und der Dienst steht auf der Anmeldeseite.', 'ok']]);
    app?.unmount();
    toasts.length = 0;
    await mount();
    expect(toasts).toEqual([]);
  });
});
