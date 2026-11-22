import { SignInReauthWindow } from '@glimstone/vue';
import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, h, nextTick } from 'vue';
import WindowHost from '@/core/WindowHost.vue';
import { winStack } from '@/core/windows';
import { addMessages } from '@/i18n';
import { reauthClient } from '@/signin/kit';
import { reauthText } from '@/signin/words';
import { useUser } from '@/stores/user';
import de from '../de.json';
import Users from './Users.vue';

type Sent = { method: string; path: string; body: Record<string, unknown> };
let sent: Sent[] = [];
let fresh = false;
const user = (name: string, more: Record<string, unknown> = {}) => ({
  id: name, name, full: name, role: 'user', scope: '/mnt/user', groups: [], loginMethod: 'password', twofa: false, passkeys: 0,
  blocked: false, owner: false, last: null, pwChanged: null, ...more,
});
const USERS = [user('jdp', { role: 'admin', owner: true }), user('anna', { blocked: true }), user('ben')];

function answer(status: number, body?: unknown) {
  return new Response(body === undefined ? null : JSON.stringify(body), { status, headers: { 'Content-Type': status >= 400 ? 'application/problem+json' : 'application/json' } });
}
async function server(url: string, init: RequestInit = {}) {
  const u = new URL(url, 'http://cc.test');
  const path = u.pathname.replace('/api/v1', ''), method = init.method ?? 'GET';
  const body = init.body ? JSON.parse(String(init.body)) : {};
  if (method !== 'GET') sent.push({ method, path, body });
  if (path === '/api/signin/v1/me/confirm') {
    fresh = body.password === 'geheim';
    return fresh ? answer(200, {}) : answer(401, { error: 'wrongPassword' });
  }
  if (path === '/api/signin/v1/options') return answer(200, { open: false, owner: false, minPassword: 12, signedIn: true, passkeys: false, services: [] });
  if (path === '/admin/users' && method === 'GET') return answer(200, { items: USERS });
  if (path === '/admin/groups' || path === '/shares') return answer(200, path === '/shares' ? [] : { items: [] });
  if (path.startsWith('/admin/users') && method !== 'GET') {
    if (body.password && !fresh) return answer(403, { status: 403, code: 'signInAgain', params: { askPassword: true }, detail: 'sign in again' });
    return answer(200, { ...user(String(body.name ?? 'x')), ...(body.password && String(body.password).length < 12 ? { short: true } : {}) });
  }
  if (path.startsWith('/fs/list')) return answer(200, { path: '/mnt/user', items: [] });
  return answer(200, { items: [] });
}

beforeAll(() => {
  addMessages('config', { de });
  vi.stubGlobal('ResizeObserver', class { observe() {} disconnect() {} });
});
beforeEach(() => {
  setActivePinia(createPinia());
  sent = [];
  fresh = false;
  vi.stubGlobal('fetch', vi.fn(server));
});
let app: ReturnType<typeof createApp> | undefined;
afterEach(() => {
  reauthClient.reauth.cancel();
  app?.unmount();
  winStack.splice(0);
  document.body.innerHTML = '';
});

const settle = async () => {
  for (let i = 0; i < 8; i++) {
    await new Promise((r) => setTimeout(r, 0));
    await nextTick();
  }
};
async function mount() {
  useUser().me = { name: 'jdp', full: 'jdp', role: 'admin' };
  app = createApp({ render: () => [h(Users), h(WindowHost), h(SignInReauthWindow, { api: reauthClient, text: reauthText() })] });
  app.mount(document.body.appendChild(document.createElement('div')));
  await settle();
}
const q = <T extends Element = HTMLElement>(sel: string) => document.querySelector<T>(sel);
const rows = () => [...document.querySelectorAll<HTMLElement>('[data-uopen]')].filter((r) => r.tagName !== 'BUTTON');
const type = async (sel: string, text: string) => {
  const input = q<HTMLInputElement>(sel)!;
  input.value = text;
  input.dispatchEvent(new Event('input'));
  await settle();
};
const primary = () => [...document.querySelectorAll<HTMLButtonElement>('[data-f]')].at(-1)!;

describe('the users tab', () => {
  it('marks the owner and the blocked accounts, and offers no removal for the owner', async () => {
    await mount();
    const [owner, anna, ben] = rows();
    expect(owner!.textContent).toContain('Eigentümer');
    expect(anna!.textContent).toContain('Gesperrt');
    expect(ben!.textContent).not.toContain('Gesperrt');
    expect(owner!.querySelectorAll('button').length).toBeLessThan(ben!.querySelectorAll('button').length);
  });

  it('unblocks an account from its window', async () => {
    await mount();
    rows()[1]!.click();
    await settle();
    const toggle = q('#a-ublocked')!;
    expect(toggle.getAttribute('aria-checked')).toBe('true');
    toggle.click();
    await settle();
    primary().click();
    await settle();
    expect(sent.at(-1)).toMatchObject({ method: 'PATCH', path: '/admin/users/anna', body: { blocked: false } });
  });

  it('offers no block for the owner', async () => {
    await mount();
    rows()[0]!.click();
    await settle();
    expect(q('#a-ublocked')).toBeNull();
  });

  it('takes a short password for a new account and only points out that it is short', async () => {
    fresh = true;
    await mount();
    q<HTMLButtonElement>('[data-uadd]')!.click();
    await settle();
    await type('#a-uname', 'carla');
    await type('#a-upw', 'kurz');
    await type('#a-upw2', 'kurz');
    expect(document.querySelector('.glim-field-note')?.textContent).toContain('12');
    primary().click();
    await settle();
    expect(sent.at(-1)).toMatchObject({ method: 'POST', path: '/admin/users', body: { name: 'carla', password: 'kurz' } });
    expect(winStack).toHaveLength(0);
  });

  it('asks for the own password before a new account gets its password, and sends it again', async () => {
    await mount();
    q<HTMLButtonElement>('[data-uadd]')!.click();
    await settle();
    await type('#a-uname', 'carla');
    await type('#a-upw', 'ein-langes-passwort');
    await type('#a-upw2', 'ein-langes-passwort');
    primary().click();
    await settle();
    await type('[data-glim="SignInReauthWindow"] input[type="password"]', 'geheim');
    q<HTMLButtonElement>('[data-signin-reauth="confirm"]')!.click();
    await settle();
    const posts = sent.filter((s) => s.method === 'POST').map((s) => s.path);
    expect(posts).toEqual(['/admin/users', '/api/signin/v1/me/confirm', '/admin/users']);
    expect(winStack).toHaveLength(0);
  });
});
