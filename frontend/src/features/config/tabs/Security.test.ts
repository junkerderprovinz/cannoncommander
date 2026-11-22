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
  'methods/oidc': { services: [{ id: 'config', provider: 'pocket-id', name: '', issuer: 'https://id.example', clientId: 'cc', secretSet: true, scopes: 'openid', nameClaim: 'preferred_username', groupsClaim: 'groups', ca: false, on: true, paused: false, tested: true, redirect: 'https://cc.example/api/auth/oidc/callback', fromConfig: true }] },
  'methods/presets': { presets: [] },
  roles: { roles: ['admin', 'user', 'readonly'], serviceRole: 'user' },
};
const asked: string[] = [];
const sent: unknown[] = [];

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
    if (init.method === 'POST' && path === 'methods/oidc') {
      sent.push(JSON.parse(String(init.body)));
      return new Response(JSON.stringify({ error: 'issuerInvalid' }), { status: 400, headers: { 'Content-Type': 'application/json' } });
    }
    if (init.method === 'PATCH') {
      sent.push(JSON.parse(String(init.body)));
      const service = (GET['methods/oidc'] as { services: object[] }).services[0];
      return new Response(JSON.stringify({ service }), { status: 200, headers: { 'Content-Type': 'application/json' } });
    }
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

  it('shows the service of config.yaml as coming from there, with nothing to edit, remove or switch', async () => {
    await mount();
    const row = document.querySelector('[data-signin-card="services"] [data-signin-service="pocket-id"]')!;
    expect(row.textContent).toContain('Pocket ID');
    expect(row.textContent).toContain('https://id.example · aus der config.yaml');
    expect(row.textContent).not.toContain('Bearbeiten');
    expect(row.querySelector('[role="switch"]')).toBeNull();
  });

  it('turns the service of config.yaml on again after a reset paused it', async () => {
    const config = (GET['methods/oidc'] as { services: Record<string, unknown>[] }).services[0]!;
    Object.assign(config, { on: false, paused: true });
    try {
      await mount();
      const again = [...document.querySelectorAll<HTMLButtonElement>('[data-signin-service="pocket-id"] button')].find((b) => b.textContent?.includes('Wieder einschalten'))!;
      again.click();
      await settle();
      expect(asked).toContain('PATCH methods/oidc/config');
      expect(sent.at(-1)).toEqual({ on: true });
    } finally {
      Object.assign(config, { on: true, paused: false });
    }
  });

  it('sends an account from a sign-in service there for its password, two-factor and passkeys', async () => {
    const me = GET.me as Record<string, unknown>;
    Object.assign(me, { password: false, via: 'service', service: 'config' });
    try {
      await mount();
      expect(document.querySelector('[data-signin-card="password"]')?.textContent).toContain('Du meldest dich über Pocket ID an, deshalb hat dieses Konto hier kein Passwort.');
      expect(document.querySelector('[data-signin-card="twoFactor"]')?.textContent).toContain('Das läuft über deinen Anmeldedienst');
      expect(document.querySelector('[data-signin-card="passkeys"]')?.textContent).toContain('Zwei-Faktor-Authentifizierung und Passkeys richtest du dort ein.');
    } finally {
      Object.assign(me, { password: true, via: 'password', service: undefined });
    }
  });

  it('gives newcomers from a new sign-in service the role User unless the form picks another', async () => {
    await mount();
    document.querySelector<HTMLButtonElement>('[data-signin-tile="service-pick"]')!.click();
    await settle();
    [...document.querySelectorAll<HTMLElement>('[data-glim="PickerWindow"] button')].find((b) => b.textContent?.includes('Anderer Anmeldedienst'))!.click();
    await settle();
    const quiet = () => document.querySelector('.glim-signin-tile-quiet')?.textContent ?? '';
    const role = document.querySelector('[data-signin-field="role"]')!;
    expect(role.textContent).toContain('Admin');
    expect(role.textContent).toContain('Nur lesen');
    expect(quiet()).toContain('bekommt hier die Rolle Nutzer');
    [...role.querySelectorAll<HTMLElement>('button, [role="radio"]')].find((b) => b.textContent?.trim() === 'Admin')!.click();
    await settle();
    expect(quiet()).toContain('bekommt hier die Rolle Admin');
    const field = (name: string) => {
      const el = document.querySelector<HTMLElement>(`[data-signin-field="${name}"]`)!;
      return (el.tagName === 'INPUT' ? el : el.querySelector('input')) as HTMLInputElement;
    };
    for (const [name, value] of [['label', 'Haus-ID'], ['issuer', 'https://id.example'], ['clientId', 'cc'], ['secret', 'geheim']]) {
      field(name!).value = value!;
      field(name!).dispatchEvent(new Event('input'));
    }
    await settle();
    // Scopes, name and groups wait closed under More settings, beside the fields every setup needs.
    const fold = document.querySelector('[data-signin-field="scopes"]')!.closest('[data-scope="accordion"][data-part="item"]')!;
    expect(fold.querySelector('[data-part="item-trigger"]')?.textContent).toContain('Weitere Einstellungen');
    expect(fold.querySelector('[data-part="item-content"]')?.getAttribute('data-state')).toBe('closed');
    expect(document.querySelector('[data-signin-field="issuer"]')!.closest('[data-scope="accordion"]')).toBeNull();
    document.querySelector<HTMLButtonElement>('[data-signin-tile="service-test"]')!.click();
    await settle();
    expect(sent.at(-1)).toMatchObject({ provider: 'custom', name: 'Haus-ID', role: 'admin' });
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
