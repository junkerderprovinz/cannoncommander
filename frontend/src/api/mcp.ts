// The MCP server's keys, its sign-in through OAuth and the consent page. Times arrive as Unix seconds and leave this
// file as milliseconds, with 0 for never, so the card compares and formats them like every other date.
import { call } from './client';
import { now } from '@/core/format';

export interface McpKey {
  id: string; owner: string; own: boolean; viaOAuth: boolean; label: string; client: string; hint: string; canWrite: boolean;
  createdAt: number; rotatedAt: number; lastUsedAt: number; lastUsedFrom: string; revokedAt: number; revokedReason: string;
  // Why the key stopped working although nobody revoked it, such as signingKeyChanged.
  unusable: string; callsToday: number;
}
export interface McpOAuth { possible: boolean; enabled: boolean; issuer: string; active: boolean; grantLimit: number }
export interface McpWriteLimits { entriesPerCall: number; entriesPerHour: number; filesPerCall: number; bytesPerCall: number; callsPerMinute: number }
export interface McpKeys {
  endpointPath: string; limit: number; signIn: boolean; hostAllowsKeys: boolean; desktop: boolean; writeLimits: McpWriteLimits;
  keys: McpKey[]; revoked: McpKey[]; oauth: McpOAuth;
  stats?: { requests: Record<string, number>; tools: Record<string, number> };
}
export interface McpEvent { at: number; tool: string; outcome: string; jobId?: string }
export interface McpConsent {
  client: { id: string; name: string; known: string; redirectHost: string; loopback: boolean };
  // password says that Allow asks for the account's password, which only an account that signs in with one has.
  account: string; password: boolean; ticket: string; limitReached: boolean; grantLimit: number;
}
export interface McpSecret { key: string; item: McpKey }

const ms = (s: number) => (s ? s * 1000 : 0);
const key = (k: McpKey): McpKey => ({ ...k, createdAt: ms(k.createdAt), rotatedAt: ms(k.rotatedAt), lastUsedAt: ms(k.lastUsedAt), revokedAt: ms(k.revokedAt) });
const secret = (s: McpSecret): McpSecret => ({ key: s.key, item: key(s.item) });
const send = (method: string, body?: unknown): RequestInit => ({ method, ...(body === undefined ? {} : { body: JSON.stringify(body) }) });
async function get<T>(path: string) {
  return (await call<T>(path)).body!;
}
async function write<T = void>(method: string, path: string, body?: unknown) {
  return (await call<T>(path, send(method, body))).body as T;
}
const at = (id: string) => '/mcp/keys/' + encodeURIComponent(id);

// The calls of today count from this browser's midnight, which is not the server's in every time zone.
function midnight() {
  const d = new Date(now());
  d.setHours(0, 0, 0, 0);
  return String(Math.floor(d.getTime() / 1000));
}

export const mcp = {
  async keys(all = false): Promise<McpKeys> {
    const got = await get<McpKeys>('/mcp/keys?' + new URLSearchParams({ since: midnight(), ...(all ? { all: 'true' } : {}) }));
    return { ...got, keys: got.keys.map(key), revoked: got.revoked.map(key) };
  },
  // password is the account's own, left out where the server asks for none.
  create: async (label: string, client: string, canWrite: boolean, password?: string) =>
    secret(await write<McpSecret>('POST', '/mcp/keys', { label, client, canWrite, password })),
  update: async (id: string, change: { label?: string; canWrite?: boolean; password?: string }) => key(await write<McpKey>('PATCH', at(id), change)),
  rotate: async (id: string, password?: string) => secret(await write<McpSecret>('POST', at(id) + '/rotate', { password })),
  revoke: (id: string) => write('POST', at(id) + '/revoke'),
  purge: (id: string) => write('DELETE', at(id)),
  activity: async (id: string) => (await get<{ events: McpEvent[] }>(at(id) + '/activity')).events.map((e) => ({ ...e, at: ms(e.at) })),
  setOAuth: (enabled: boolean, issuer: string) => write<McpOAuth>('PUT', '/mcp/oauth', { enabled, issuer }),
  // search is the query the client opened the consent page with, passed on as it came.
  consent: (search: string) => get<McpConsent>('/mcp/oauth/authorize' + search),
  // Allow needs the password of an account that has one, as making a key does.
  answer: (ticket: string, allow: boolean, canWrite: boolean, password?: string) =>
    write<{ redirect: string }>('POST', '/mcp/oauth/authorize', password === undefined ? { ticket, allow, canWrite } : { ticket, allow, canWrite, password }),
};
