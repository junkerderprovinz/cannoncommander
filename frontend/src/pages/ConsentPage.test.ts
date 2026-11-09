import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import ConsentPage from './ConsentPage.vue';

const { assigned } = vi.hoisted(() => ({ assigned: [] as string[] }));
vi.mock('./consent', async (original) => ({ ...(await original<typeof import('./consent')>()), returnTo: (url: string) => assigned.push(url) }));

type Sent = { method: string; path: string; body: Record<string, unknown> };
let sent: Sent[] = [];
let refuse: string | null = null;

const answer = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': status >= 400 ? 'application/problem+json' : 'application/json' } });
async function server(url: string, init: RequestInit = {}) {
  const u = new URL(url, 'http://cc.test');
  const method = init.method ?? 'GET';
  sent.push({ method, path: u.pathname + u.search, body: init.body ? JSON.parse(String(init.body)) : {} });
  if (refuse) return answer(400, { status: 400, code: refuse, detail: 'refused' });
  if (method === 'GET') {
    return answer(200, { client: { id: 'c1', name: 'Something', known: 'claude', redirectHost: 'claude.ai', loopback: false }, account: 'anna', ticket: 't1', limitReached: false, grantLimit: 10 });
  }
  return answer(200, { redirect: 'https://claude.ai/api/mcp/auth_callback?code=x' });
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  sent = [];
  refuse = null;
  assigned.length = 0;
  vi.stubGlobal('fetch', vi.fn(server));
  history.replaceState(null, '', '/oauth/authorize?client_id=c1&state=s');
});
let app: ReturnType<typeof createApp> | undefined;
afterEach(() => {
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

  it('keeps a refused request on the page instead of sending anybody anywhere', async () => {
    refuse = 'oauthBadRedirect';
    await mount();
    expect(document.querySelector('[data-consent="problem"]')).not.toBeNull();
    expect(button('allow')).toBeNull();
    expect(assigned).toEqual([]);
  });
});
