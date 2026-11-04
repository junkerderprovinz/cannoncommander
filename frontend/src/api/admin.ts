// The settings pages talk to the server through these calls. Times arrive as ISO strings and leave this file as
// milliseconds, so the pages compare and format them like every other date.
import { call } from './client';
import { type Share as WireShare, shareUrl, tools } from './tools';

const q = (params: Record<string, string>) => '?' + new URLSearchParams(params).toString();
const send = (method: string, body?: unknown): RequestInit => ({
  method, ...(body === undefined ? {} : { body: JSON.stringify(body), headers: { 'Content-Type': 'application/json' } }),
});
const ms = (iso: string | null | undefined) => (iso ? Date.parse(iso) : null);
const iso = (t: number | null) => (t == null ? null : new Date(t).toISOString());
async function get<T>(path: string): Promise<T> {
  return (await call<T>(path)).body!;
}
async function write<T = void>(method: string, path: string, body?: unknown): Promise<T> {
  return (await call<T>(path, send(method, body))).body as T;
}
const items = async <T>(path: string) => (await get<{ items: T[] }>(path)).items;

export type Role = 'admin' | 'user' | 'readonly';
export interface User { name: string; full: string; role: Role; groups: string[]; scope: string; last: number | null; twofa: boolean }
export interface Group { name: string; note: string; members: string[] }
export type RuleKind = 'hide' | 'read' | 'allow';
// who is a user name, @group or * for everybody.
export interface Rule { who: string; path: string; rule: RuleKind }
export interface Verdict { result: 'hidden' | 'read' | 'visible'; scope?: string; rule?: number }
export interface Source { name: string; path: string; readOnly: boolean; indexed: boolean; skipHidden: boolean; exclude: string[]; files: number; size: number; indexing?: boolean }
export interface IndexState { every: 'hourly' | 'daily' | 'manual'; last: number | null; running: string[] }
export interface Remote { name: string; type: string; mount: string; mounted: boolean; account?: string; cfg: Record<string, string> }
export interface Share { id: string; path: string; name?: string; slug: string; url: string; by: string; created: number; expires: number | null; password: boolean; views: number; limit?: number; items?: number }
export interface Session { id: string; created: number; seen: number; userAgent: string; ip: string; current: boolean }
export interface Token { id: string; name: string; created: number; last: number | null; rights: 'r' | 'rw'; expires: number | null }
export interface Passkey { id: string; name: string; origin: string; created: number; last: number | null }
export interface Account { name: string; full: string; role: Role; twofa: boolean; pwChanged: number | null; recovery: number }
export interface Activity { t: number; who: string; what: string; obj: string; path?: string }
export type OfficeKind = 'none' | 'onlyoffice' | 'eurooffice' | 'collabora';
export interface AppSettings {
  indexEvery: IndexState['every'];
  rclone: { automount: boolean; cache: 'off' | 'writes' | 'full'; cacheSize: number };
  office: { kind: OfficeKind; url: string; secret: string };
  thumbs: boolean; videoPreview: boolean; webdav: boolean; webdavUrl: string;
  autoCheck: boolean; logLevel: 'error' | 'info' | 'debug'; image: string; tag: string;
}
export interface System { version: { app: string; rclone: string; ffmpeg: string }; paths: { rclone: string; ffmpeg: string }; started: number; update?: string }

type Wire<T, K extends keyof T> = Omit<T, K> & { [P in K]: string | null };
const user = (u: Wire<User, 'last'>): User => ({ ...u, last: ms(u.last) });
// A collection lists all its paths where a single link names its one.
const share = (s: WireShare): Share => ({
  id: s.id, path: s.paths.join(', '), slug: s.slug, url: shareUrl(s.slug), by: s.owner, created: ms(s.created)!, expires: ms(s.expires),
  password: s.password, views: s.views, limit: s.limit, ...(s.kind === 'collection' ? { name: s.name, items: s.paths.length } : {}),
});
const token = (x: Wire<Token, 'created' | 'last' | 'expires'>): Token => ({ ...x, created: ms(x.created)!, last: ms(x.last), expires: ms(x.expires) });
const passkey = (x: Wire<Passkey, 'created' | 'last'>): Passkey => ({ ...x, created: ms(x.created)!, last: ms(x.last) });

export const admin = {
  users: async () => (await items<Wire<User, 'last'>>('/admin/users')).map(user),
  createUser: (u: Omit<User, 'last' | 'twofa'> & { password: string }) => write('POST', '/admin/users', u),
  updateUser: (name: string, u: Partial<User> & { password?: string }) => write('PATCH', `/admin/users/${encodeURIComponent(name)}`, u),
  deleteUser: (name: string) => write('DELETE', `/admin/users/${encodeURIComponent(name)}`),
  groups: () => items<Group>('/admin/groups'),
  createGroup: (g: Group) => write('POST', '/admin/groups', g),
  updateGroup: (name: string, g: Group) => write('PATCH', `/admin/groups/${encodeURIComponent(name)}`, g),
  deleteGroup: (name: string) => write('DELETE', `/admin/groups/${encodeURIComponent(name)}`),
  rules: () => items<Rule>('/admin/rules'),
  saveRules: (rules: Rule[]) => write('PUT', '/admin/rules', { items: rules }),
  testRule: (user: string, path: string) => write<Verdict>('POST', '/admin/rules/test', { user, path }),
  activity: async () => (await items<Wire<Activity, 't'>>('/admin/activity')).map((a) => ({ ...a, t: ms(a.t)! })),
  sources: () => items<Source>('/admin/sources'),
  saveSources: (sources: Source[]) => write('PUT', '/admin/sources', { items: sources }),
  index: async (): Promise<IndexState> => { const s = await get<Wire<IndexState, 'last'>>('/admin/index'); return { ...s, last: ms(s.last) }; },
  reindex: (path?: string) => write('POST', '/admin/index', path ? { path } : {}),
  app: () => get<AppSettings>('/admin/app'),
  saveApp: (s: AppSettings) => write('PUT', '/admin/app', s),
  testOffice: (office: AppSettings['office']) => write<{ ok: boolean; reason?: 'secret' | 'unreachable' }>('POST', '/admin/integrations/test', { integration: 'office', ...office }),
  restart: () => write('POST', '/admin/app/restart'),
  exportSettings: (path: string, users: boolean) => write('POST', '/admin/app/export', { path, users }),
  importSettings: (path: string, users: boolean) => write('POST', '/admin/app/import', { path, users }),
  shares: async (all: boolean) => (await (all ? tools.allShares() : tools.shares())).map(share),
  updateShare: (id: string, change: { expires?: number | null; password?: string | null }) =>
    write('PATCH', `/shares/${id}`, { ...change, ...('expires' in change ? { expires: iso(change.expires ?? null) } : {}), ...(change.password === null ? { password: '' } : {}) }),
  deleteShare: (id: string) => write('DELETE', `/shares/${id}`),
  remotes: () => items<Remote>('/remotes'),
  addRemote: (r: Omit<Remote, 'mounted'>) => write('POST', '/remotes', r),
  updateRemote: (name: string, r: Omit<Remote, 'mounted'>) => write('PUT', `/remotes/${encodeURIComponent(name)}`, r),
  deleteRemote: (name: string) => write('DELETE', `/remotes/${encodeURIComponent(name)}`),
  mountRemote: (name: string, on: boolean) => write('POST', `/remotes/${encodeURIComponent(name)}/${on ? 'mount' : 'unmount'}`),
  testRemote: (type: string, cfg: Record<string, string>) => write<{ ok: boolean; detail?: string }>('POST', '/remotes/test', { type, cfg }),
  remoteToken: (type: string, token: string) => write<{ account: string }>('POST', '/remotes/oauth', { type, token }),
  system: async (): Promise<System> => { const s = await get<Wire<System, 'started'>>('/system'); return { ...s, started: ms(s.started)! }; },
  checkUpdate: () => write<{ latest: string; current: boolean }>('POST', '/system/update'),
};

export const me = {
  account: async (): Promise<Account> => { const a = await get<Wire<Account, 'pwChanged'>>('/me'); return { ...a, pwChanged: ms(a.pwChanged) }; },
  password: (current: string, next: string) => write<{ ended: number }>('PUT', '/me/password', { current, next }),
  sessions: async () => (await items<Wire<Session, 'created' | 'seen'>>('/me/sessions')).map((s) => ({ ...s, created: ms(s.created)!, seen: ms(s.seen)! })),
  endSession: (id: string) => write('DELETE', `/me/sessions/${id}`),
  tokens: async () => (await items<Wire<Token, 'created' | 'last' | 'expires'>>('/me/tokens')).map(token),
  createToken: (name: string, rights: Token['rights'], days: number | null) => write<{ secret: string }>('POST', '/me/tokens', { name, rights, days }),
  deleteToken: (id: string) => write('DELETE', `/me/tokens/${id}`),
  passkeys: async () => (await items<Wire<Passkey, 'created' | 'last'>>('/me/passkeys')).map(passkey),
  addPasskey: (body: { name: string; credential?: unknown }) => write<{ options?: CredentialCreationOptions; passkey?: unknown }>('POST', '/me/passkeys', body),
  deletePasskey: (id: string) => write('DELETE', `/me/passkeys/${id}`),
  // The server draws the QR code, so no encoder ships to the browser.
  totpBegin: () => write<{ secret: string; uri: string; qr: string }>('POST', '/me/totp', {}),
  totpConfirm: (code: string) => write<{ codes: string[] }>('POST', '/me/totp', { code }),
  totpCodes: (code: string) => write<{ codes: string[] }>('POST', '/me/totp', { code, regenerate: true }),
  totpOff: (code: string) => write('DELETE', '/me/totp', { code }),
};

// Whether a folder or, with files, a file exists, asked the way the file list asks.
export async function exists(path: string, kind: 'dir' | 'file' = 'dir'): Promise<boolean> {
  if (kind === 'dir') return call(`/fs/list${q({ path })}`).then(() => true, () => false);
  const at = path.lastIndexOf('/');
  const parent = path.slice(0, at) || '/', name = path.slice(at + 1);
  return call<{ items: { name: string; dir: boolean }[] }>(`/fs/list${q({ path: parent })}`)
    .then((r) => !!r.body?.items.some((k) => k.name === name && !k.dir), () => false);
}

export async function listDir(path: string) {
  return (await call<{ items: { name: string; dir: boolean }[] }>(`/fs/list${q({ path })}`)).body?.items ?? [];
}
