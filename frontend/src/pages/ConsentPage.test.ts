import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import { reauthClient } from '@/signin/kit';
import ConsentPage from './ConsentPage.vue';

const { assigned } = vi.hoisted(() => ({ assigned: [] as string[] }));
vi.mock('./consent', async (original) => ({ ...(await original<typeof import('./consent')>()), returnTo: (url: string) => assigned.push(url) }));

type Sent = { method: string; path: string; body: Record<string, unknown> };
let sent: Sent[] = [];
let refuse: string | null = null;
// How the server answers Allow, one answer after the other, and what the sign-in says to a password.
let allowAnswers: [number, unknown][] = [];
let confirmAnswer: [number, unknown] = [200, {}];

const answer = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': status >= 400 ? 'application/problem+json' : 'application/json' } });
async function server(url: string, init: RequestInit = {}) {
  const u = new URL(url, 'http://cc.test');
  const method = init.method ?? 'GET';
  sent.push({ method, path: u.pathname + u.search, body: init.body ? JSON.parse(String(init.body)) : {} });
  if (u.pathname === '/api/signin/v1/me/confirm') return answer(...confirmAnswer);
  if (refuse) return answer(400, { status: 400, code: refuse, detail: 'refused' });
  if (method === 'GET') {
    return answer(200, { client: { id: 'c1', name: 'Something', known: 'claude', redirectHost: 'claude.ai', loopback: false }, account: 'anna', ticket: 't1', limitReached: false, grantLimit: 10 });
  }
  const next = allowAnswers.shift();
  if (next) return answer(...next);
  return answer(200, { redirect: 'https://claude.ai/api/mcp/auth_callback?code=x' });
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  sent = [];
  refuse = null;
  allowAnswers = [];
  confirmAnswer = [200, {}];
  assigned.length = 0;
  vi.stubGlobal('fetch', vi.fn(server));
  history.replaceState(null, '', '/oauth/authorize?client_id=c1&state=s');
});
let app: ReturnType<typeof createApp> | undefined;
afterEach(() => {
  reauthClient.reauth.cancel();
  app?.unmount();
  document.body.innerHTML = '';
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

const settle = async () => {
  for (let i = 0; i < 6; i++) {
    await vi.advanceTimersByTimeAsync(0);
    await nextTick();
  }
};
async function mount() {
  app = createApp(ConsentPage);
  app.mount(document.body.appendChild(document.createElement('div')));
  await settle();
}
const button = (name: string) => document.querySelector<HTMLButtonElement>(`[data-consent="${name}"]`)!;

describe('the consent page', () => {
  it('asks with the query the client opened it with and names the client, the account and where the answer goes', async () => {
    await mount();
    expect(sent[0]).toMatchObject({ method: 'GET', path: '/api/v1/mcp/oauth/authorize?client_id=c1&state=s' });
    const text = document.body.textContent!;
    expect(text).toContain('Claude');
    expect(text).toContain('anna');
    expect(text).toContain('claude.ai');
  });

  it('keeps Allow off for a moment, so a stray double click cannot answer', async () => {
    await mount();
    expect(button('allow').disabled).toBe(true);
    await vi.advanceTimersByTimeAsync(1100);
    await nextTick();
    expect(button('allow').disabled).toBe(false);
  });

  it('allows reading only unless the write switch is turned on, then goes back to the client', async () => {
    await mount();
    expect(button('write').getAttribute('aria-checked')).toBe('false');
    await vi.advanceTimersByTimeAsync(1100);
    await nextTick();
    button('allow').click();
    await settle();
    expect(sent.at(-1)).toEqual({ method: 'POST', path: '/api/v1/mcp/oauth/authorize', body: { ticket: 't1', allow: true, canWrite: false } });
    expect(assigned).toEqual(['https://claude.ai/api/mcp/auth_callback?code=x']);
  });

  it('tells the client no when denied', async () => {
    await mount();
    button('deny').click();
    await settle();
    expect(sent.at(-1)?.body).toEqual({ ticket: 't1', allow: false, canWrite: false });
    expect(assigned).toHaveLength(1);
  });

  it('asks for the password in a window when the sign-in is old, and allows once it is confirmed', async () => {
    allowAnswers = [[403, { status: 403, code: 'signInAgain', params: { askPassword: true }, detail: 'sign in again' }]];
    await mount();
    await vi.advanceTimersByTimeAsync(1100);
    await nextTick();
    button('allow').click();
    await settle();
    const field = document.querySelector<HTMLInputElement>('[data-glim="SignInReauthWindow"] input[type="password"]')!;
    field.value = 'richtig-richtig';
    field.dispatchEvent(new Event('input'));
    await settle();
    document.querySelector<HTMLButtonElement>('[data-signin-reauth="confirm"]')!.click();
    await settle();
    expect(sent.filter((s) => s.method === 'POST').map((s) => [s.path, s.body])).toEqual([
      ['/api/v1/mcp/oauth/authorize', { ticket: 't1', allow: true, canWrite: false }],
      ['/api/signin/v1/me/confirm', { password: 'richtig-richtig' }],
      ['/api/v1/mcp/oauth/authorize', { ticket: 't1', allow: true, canWrite: false }],
    ]);
    expect(assigned).toEqual(['https://claude.ai/api/mcp/auth_callback?code=x']);
  });

  it('keeps the window open on a wrong password', async () => {
    allowAnswers = [[403, { status: 403, code: 'signInAgain', params: { askPassword: true }, detail: 'sign in again' }]];
    confirmAnswer = [401, { error: 'wrongPassword' }];
    await mount();
    await vi.advanceTimersByTimeAsync(1100);
    await nextTick();
    button('allow').click();
    await settle();
    const field = document.querySelector<HTMLInputElement>('[data-glim="SignInReauthWindow"] input[type="password"]')!;
    field.value = 'falsch-falsch';
    field.dispatchEvent(new Event('input'));
    await settle();
    document.querySelector<HTMLButtonElement>('[data-signin-reauth="confirm"]')!.click();
    await settle();
    expect(document.querySelector('[data-glim="SignInReauthWindow"]')?.textContent).toContain('Das Passwort stimmt nicht');
    expect(assigned).toEqual([]);
  });

  it('says to sign in again where no password can stand in for it', async () => {
    allowAnswers = [[403, { status: 403, code: 'signInAgain', params: { askPassword: false }, detail: 'sign in again' }]];
    await mount();
    await vi.advanceTimersByTimeAsync(1100);
    await nextTick();
    button('allow').click();
    await settle();
    expect(document.querySelector('[data-glim="SignInReauthWindow"]')).toBeNull();
    expect(document.querySelector('[data-consent="problem"]')?.textContent).toContain('frische Anmeldung');
    expect(assigned).toEqual([]);
  });

  it('keeps a refused request on the page instead of sending anybody anywhere', async () => {
    refuse = 'oauthBadRedirect';
    await mount();
    expect(document.querySelector('[data-consent="problem"]')).not.toBeNull();
    expect(button('allow')).toBeNull();
    expect(assigned).toEqual([]);
  });
});
