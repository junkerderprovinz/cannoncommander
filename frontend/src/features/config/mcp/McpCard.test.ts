import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, h, nextTick } from 'vue';
import WindowHost from '@/core/WindowHost.vue';
import { winStack } from '@/core/windows';
import { addMessages } from '@/i18n';
import { useUser } from '@/stores/user';
import de from '../de.json';
import McpCard from './McpCard.vue';

type Sent = { method: string; path: string; body: Record<string, unknown> };
let sent: Sent[] = [];
let list: Record<string, unknown>;
let loginMethod = 'password';

const key = (id: string, label: string, more: Record<string, unknown> = {}) => ({
  id: id.padEnd(32, '0'), owner: 'jdp', own: true, viaOAuth: false, label, client: 'claude-code', hint: 'q7Xe', canWrite: false, createdAt: 1_700_000_000,
  rotatedAt: 0, lastUsedAt: 0, lastUsedFrom: '', revokedAt: 0, revokedReason: '', unusable: '', callsToday: 3, ...more,
});
function answer(status: number, body?: unknown) {
  const problem = status >= 400;
  return new Response(body === undefined ? null : JSON.stringify(body), { status, headers: { 'Content-Type': problem ? 'application/problem+json' : 'application/json' } });
}
async function server(url: string, init: RequestInit = {}) {
  const u = new URL(url, 'http://cc.test');
  const path = u.pathname.replace('/api/v1', ''), method = init.method ?? 'GET';
  const body = init.body ? JSON.parse(String(init.body)) : {};
  if (method !== 'GET') sent.push({ method, path, body });
  if (path === '/me') return answer(200, { name: 'jdp', full: 'jdp', role: 'admin', loginMethod, last: null, pwChanged: null, recovery: 8, lockPassword: false });
  if (path === '/mcp/keys' && method === 'GET') {
    const all = u.searchParams.get('all') === 'true';
    const others = all ? [key('b', 'LM Studio', { owner: 'anna', own: false, client: 'lmstudio' })] : [];
    return answer(200, { ...list, keys: [...(list.keys as unknown[]), ...others] });
  }
  if (path === '/mcp/keys' && method === 'POST') {
    if (list.signIn && body.password !== 'geheim') return answer(403, { status: 403, code: 'currentPasswordWrong' });
    return answer(201, { key: 'ccmcp_fresh', item: key('c', String(body.label), { canWrite: body.canWrite }) });
  }
  if (/^\/mcp\/keys\/\w+$/.test(path) && method === 'PATCH') {
    if (body.canWrite && list.signIn && body.password !== 'geheim') return answer(403, { status: 403, code: 'currentPasswordWrong' });
    return answer(200, key('a', 'Claude Code', { canWrite: body.canWrite }));
  }
  return answer(204);
}

const settings = (more: Record<string, unknown> = {}) => ({
  endpointPath: 'mcp', limit: 10, signIn: true, hostAllowsKeys: true, desktop: false,
  writeLimits: { entriesPerCall: 25, entriesPerHour: 200, filesPerCall: 10000, bytesPerCall: 53687091200, callsPerMinute: 120 },
  keys: [key('a', 'Claude Code')], revoked: [], oauth: { possible: true, enabled: false, issuer: '', active: false, grantLimit: 10 }, ...more,
});

beforeAll(() => {
  addMessages('config', { de });
  vi.stubGlobal('ResizeObserver', class { observe() {} disconnect() {} });
});
beforeEach(() => {
  setActivePinia(createPinia());
  sent = [];
  list = settings();
  loginMethod = 'password';
  vi.stubGlobal('fetch', vi.fn(server));
});
let app: ReturnType<typeof createApp> | undefined;
afterEach(() => {
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
async function mount(role = 'user') {
  useUser().me = { name: 'jdp', full: 'jdp', role };
  app = createApp({ render: () => [h(McpCard), h(WindowHost)] });
  app.mount(document.body.appendChild(document.createElement('div')));
  await settle();
}
const q = <T extends Element = HTMLElement>(sel: string) => document.querySelector<T>(sel);
const click = async (el: Element | null) => {
  (el as HTMLElement).click();
  await settle();
};
// The window's own buttons sit in its foot, the primary one last.
const primary = () => [...document.querySelectorAll<HTMLButtonElement>('[data-f]')].at(-1)!;
const type = async (sel: string, text: string) => {
  const input = q<HTMLInputElement>(sel)!;
  input.value = text;
  input.dispatchEvent(new Event('input'));
  await settle();
};
const cardTitles = () => [...document.querySelectorAll('.glim-notch-card > h2')].map((x) => x.textContent?.trim());

describe('the MCP server card', () => {
  it('shows the account its own keys and the endpoint, and no other account\'s', async () => {
    await mount();
    expect(cardTitles()).toEqual(['MCP-Server']);
    expect(q('[data-mcp="keys"]')?.textContent).toContain('Claude Code');
    expect(q('[data-mcp="keys"]')?.textContent).toContain('…q7Xe');
    expect(document.body.innerHTML).toContain('http://localhost:3000/mcp');
  });

  it('makes a new key that only reads unless the switch is turned on, and asks for the password', async () => {
    await mount();
    await click(q('#mcp-choose'));
    await click(q('[data-client="cursor"] button'));
    const toggle = q('[data-mcp="newWrite"]');
    expect(toggle?.getAttribute('aria-checked')).toBe('false');
    expect(q('#mcp-cpw')).not.toBeNull();
    await type('#mcp-cpw', 'geheim');
    await click(q('#mcp-make'));
    expect(sent.at(-1)).toEqual({ method: 'POST', path: '/mcp/keys', body: { label: 'Cursor', client: 'cursor', canWrite: false, password: 'geheim' } });
    expect(document.body.innerHTML).toContain('ccmcp_fresh');
  });

  it('asks for the password before a key may write, and sends nothing without it', async () => {
    await mount();
    await click(q('[data-mcp="write"]'));
    expect(winStack).toHaveLength(1);
    expect(sent).toEqual([]);
    await click(primary());
    expect(sent).toEqual([]);
    await type('#mcp-pw', 'geheim');
    await click(primary());
    expect(sent).toEqual([{ method: 'PATCH', path: '/mcp/keys/a0000000000000000000000000000000', body: { canWrite: true, password: 'geheim' } }]);
    expect(winStack).toHaveLength(0);
  });

  it('takes writing away at once, without a window', async () => {
    list = settings({ keys: [key('a', 'Claude Code', { canWrite: true })] });
    await mount();
    await click(q('[data-mcp="write"]'));
    expect(winStack).toHaveLength(0);
    expect(sent).toEqual([{ method: 'PATCH', path: '/mcp/keys/a0000000000000000000000000000000', body: { canWrite: false } }]);
  });

  it('asks an account that signs in elsewhere for no password', async () => {
    loginMethod = 'oidc';
    await mount();
    await click(q('[data-mcp="write"]'));
    expect(q('#mcp-pw')).toBeNull();
  });

  it('shows an admin everybody else\'s keys with their owner, and the sign-in through OAuth', async () => {
    await mount('admin');
    expect(cardTitles()).toEqual(['MCP-Server', 'Schlüssel aller Konten']);
    const others = q('[data-mcp="others"]')!;
    expect(others.textContent).toContain('LM Studio');
    expect(others.textContent).toContain('anna');
    expect(q('[data-mcp="oauth"]')).not.toBeNull();
    // Only the owner allows writing, so the admin's switch for it refuses and opens nothing.
    await click(others.querySelector('[data-mcp="write"]'));
    expect(winStack).toHaveLength(0);
    expect(sent).toEqual([]);
    await click(others.querySelector('[data-mcp="revoke"]'));
    await click(primary());
    expect(sent).toEqual([{ method: 'POST', path: '/mcp/keys/b0000000000000000000000000000000/revoke', body: {} }]);
  });

  it('in the desktop app says the port may change, asks for no password and leaves out the cloud', async () => {
    list = settings({ signIn: false, desktop: true, oauth: { possible: false, enabled: false, issuer: '', active: false, grantLimit: 10 } });
    await mount('admin');
    expect(q('[data-mcp="port"]')).not.toBeNull();
    expect(q('[data-mcp="oauth"]')).toBeNull();
    await click(q('#mcp-choose'));
    expect(q('[data-group="cloud"]')).toBeNull();
    expect(q('[data-group="local"]')).not.toBeNull();
    await click(q('[data-client="zed"] button'));
    expect(q('#mcp-cpw')).toBeNull();
  });
});
