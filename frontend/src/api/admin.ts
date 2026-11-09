// The settings pages talk to the server through these calls. Times arrive as ISO strings and leave this file as
// milliseconds, so the pages compare and format them like every other date.
import { ApiError, call } from './client';
import { type Share as WireShare, shareUrl, tools } from './tools';
import type { Problem } from './types';
import { creationOptions, credentialJSON } from './webauthn';

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
const at = (name: string) => encodeURIComponent(name);

// The status of a refused call, so a page can tell a wrong password from too many tries.
export const statusOf = (e: unknown) => (e instanceof ApiError ? e.status : 0);

export type Role = 'admin' | 'user' | 'readonly';
export interface User {
  id: string; name: string; full: string; role: Role; scope: string; groups: string[];
  loginMethod: 'password' | 'oidc'; twofa: boolean; passkeys: number; last: number | null; pwChanged: number | null;
}
export interface Account extends User { recovery: number; lockPassword: boolean; token?: { name: string; rights: Token['rights'] } }
// revoke ends every session, API token and passkey of the user.
export interface UserInput { name?: string; full?: string; role?: Role; scope?: string; groups?: string[]; password?: string; revoke?: boolean }
export interface Group { name: string; note: string; members: string[] }
export type RuleKind = 'hide' | 'read' | 'allow';
// who is a user name, @group or * for everybody.
export interface Rule { who: string; path: string; rule: RuleKind }
export interface Verdict { result: 'visible' | 'read' | 'hidden' | 'missing'; reason: 'scope' | 'rule' | 'role' | 'source' | 'none'; rule?: number; scope?: string }
export interface Source { name: string; path: string; readOnly: boolean; indexed: boolean; skipHidden: boolean; exclude: string[] }
export interface Device { name: string; path: string }
export interface Sources { sources: Source[]; devices: Device[]; restartNeeded: boolean }
export interface IndexState {
  source: string; path: string; status: 'ready' | 'indexing' | 'unavailable' | 'off'; dirs: number; files: number; size: number;
  lastIndexed: number | null; quickScanSeconds: number; fullScanSeconds: number; nextScan: number | null; scanEverySeconds?: number;
  scanning?: string; metadata: number; job?: string;
}
export interface Remote {
  name: string; type: string; path: string; mount: string; label: string; mounted: boolean; mountOnStart: boolean;
  error?: string; params?: Record<string, string>; secrets?: string[];
}
export interface RemoteInput { name?: string; type?: string; path?: string; mount?: string; params?: Record<string, string> }
export interface RemoteTest { ok: boolean; entries: number; total?: number; used?: number; free?: number; error?: string }
export interface Rclone { installed: boolean; running: boolean; version?: string; error?: string }
export interface Share extends Omit<WireShare, 'created' | 'expires'> { created: number; expires: number | null; url: string }
export interface Session { id: string; created: number; seen: number; userAgent: string; ip: string; current: boolean }
export interface Token { id: string; name: string; created: number; last: number | null; rights: 'r' | 'rw'; expires: number | null }
export interface Passkey { id: string; name: string; origin: string; created: number; last: number | null }
// blocked says why no passkey can be added from this address, null when one can.
export interface Passkeys { blocked: Problem | null; items: Passkey[] }
// authMethod is mcp for what an assistant did, and tokenName then names its key.
export interface Activity {
  id: number; t: number; who: string; what: string; obj: string; path?: string; target?: string; ip?: string; authMethod?: string; tokenName?: string;
  details: Record<string, unknown>;
}
export type OfficeKind = 'none' | 'onlyoffice' | 'eurooffice' | 'collabora';
export interface AppSettings {
  office: { kind: OfficeKind; url: string; secret?: string; secretSet?: boolean };
  arrowloop: { url: string; token?: string; tokenSet?: boolean; paths?: Record<string, string> };
  previews: { thumbs: boolean; video: boolean };
  webdav: boolean;
  terminal: boolean;
  remotes: { automount: boolean; cache: 'off' | 'writes' | 'full'; cacheSize: number };
  // autoUpdate is the desktop app's own switch; a container reports it off.
  updates: { autoCheck: boolean; autoUpdate: boolean };
  logLevel: 'error' | 'info' | 'debug';
}
// A save sends only what changed, down to a single switch.
export type AppChange = { [K in keyof AppSettings]?: AppSettings[K] extends object ? Partial<AppSettings[K]> : AppSettings[K] };
export interface IntegrationResult { kind: string; ok: boolean; reachable: boolean; token?: 'ok' | 'rejected' | 'missing' | 'off' | 'none'; version?: string; error?: string }
// checking is set while the desktop app still looks for ffmpeg.
export interface Tool { available: boolean; path?: string; version?: string; checking?: boolean }
export interface System {
  version: string; commit?: string; release: boolean; deployment: 'container' | 'desktop'; go: string; os: string; arch: string; started: number; uptime: number;
  tools: Record<string, Tool>; unraid: { available: boolean; socket: boolean };
  features: { office: { kind: OfficeKind; url?: string }; thumbs: boolean; videoPreview: boolean; webdav: boolean; mupdf: boolean; terminal: boolean };
}
// ready is the version the desktop app has put in place for its next start.
export interface UpdateCheck { version: string; latest: string; updateAvailable: boolean; releaseUrl?: string; ready?: string }
export interface Imported { sections: string[]; notes: string[]; restartNeeded: boolean }

type Wire<T, K extends keyof T> = Omit<T, K> & { [P in K]: string | null };
type WireUser = Wire<User, 'last' | 'pwChanged'>;
const user = <U extends WireUser>(u: U) => ({ ...u, last: ms(u.last), pwChanged: ms(u.pwChanged) });
const share = (s: WireShare): Share => ({ ...s, created: ms(s.created)!, expires: ms(s.expires), url: shareUrl(s.slug) });
const token = (x: Wire<Token, 'created' | 'last' | 'expires'>): Token => ({ ...x, created: ms(x.created)!, last: ms(x.last), expires: ms(x.expires) });
const passkey = (x: Wire<Passkey, 'created' | 'last'>): Passkey => ({ ...x, created: ms(x.created)!, last: ms(x.last) });
const session = (x: Wire<Session, 'created' | 'seen'>): Session => ({ ...x, created: ms(x.created)!, seen: ms(x.seen)! });
const index = (x: Wire<IndexState, 'lastIndexed' | 'nextScan'>): IndexState => ({ ...x, lastIndexed: ms(x.lastIndexed), nextScan: ms(x.nextScan) });

export const admin = {
  users: async () => (await items<WireUser>('/admin/users')).map(user),
  createUser: (u: UserInput) => write('POST', '/admin/users', u),
  updateUser: (name: string, u: UserInput) => write('PATCH', `/admin/users/${at(name)}`, u),
  deleteUser: (name: string) => write('DELETE', `/admin/users/${at(name)}`),
  userSessions: async (name: string) => (await items<Wire<Session, 'created' | 'seen'>>(`/admin/users/${at(name)}/sessions`)).map(session),
  endUserSession: (name: string, id: string) => write('DELETE', `/admin/users/${at(name)}/sessions/${at(id)}`),
  userTokens: async (name: string) => (await items<Wire<Token, 'created' | 'last' | 'expires'>>(`/admin/users/${at(name)}/tokens`)).map(token),
  revokeUserToken: (name: string, id: string) => write('DELETE', `/admin/users/${at(name)}/tokens/${at(id)}`),
  userPasskeys: async (name: string) => (await items<Wire<Passkey, 'created' | 'last'>>(`/admin/users/${at(name)}/passkeys`)).map(passkey),
  removeUserPasskey: (name: string, id: string) => write('DELETE', `/admin/users/${at(name)}/passkeys/${at(id)}`),
  groups: () => items<Group>('/admin/groups'),
  createGroup: (g: Group) => write('POST', '/admin/groups', g),
  updateGroup: (name: string, g: Partial<Group>) => write('PATCH', `/admin/groups/${at(name)}`, g),
  deleteGroup: (name: string) => write('DELETE', `/admin/groups/${at(name)}`),
  rules: () => items<Rule>('/admin/rules'),
  saveRules: (rules: Rule[]) => write('PUT', '/admin/rules', { items: rules }),
  testRule: (user: string, path: string) => write<Verdict>('POST', '/admin/rules/test', { user, path }),
  // Without a time span the server sends the last seven days, which is what the page shows.
  activity: async () => (await items<Wire<Activity, 't'>>('/admin/activity' + q({ limit: '500' }))).map((a) => ({ ...a, t: ms(a.t)! })),
  sources: () => get<Sources>('/admin/sources'),
  saveSources: (sources: Source[]) => write<Sources>('PUT', '/admin/sources', { sources }),
  index: async () => (await get<{ sources: Wire<IndexState, 'lastIndexed' | 'nextScan'>[] }>('/admin/index')).sources.map(index),
  reindex: (source?: string) => write('POST', '/admin/index', source ? { source } : {}),
  app: () => get<AppSettings>('/admin/app'),
  saveApp: (change: AppChange) => write<AppSettings>('PUT', '/admin/app', change),
  testIntegration: (kind: Exclude<OfficeKind, 'none'> | 'arrowloop', url: string, key?: string) =>
    write<IntegrationResult>('POST', '/admin/integrations/test', { kind, url, ...(key ? (kind === 'arrowloop' ? { token: key } : { secret: key }) : {}) }),
  restart: () => write('POST', '/admin/app/restart'),
  exportSettings: (path: string, users: boolean) => write<{ path: string; size: number }>('POST', '/admin/app/export', { path, users }),
  importSettings: (path: string, users: boolean) => write<Imported>('POST', '/admin/app/import', { path, users }),
  shares: async (all: boolean) => (await (all ? tools.allShares() : tools.shares())).map(share),
  // An expiry of null makes the link last, an empty password takes it off.
  updateShare: (id: string, change: { expires?: number | null; password?: string; message?: string }) =>
    write('PATCH', `/shares/${at(id)}`, { ...change, ...('expires' in change ? { expires: iso(change.expires ?? null) } : {}) }),
  deleteShare: (id: string) => write('DELETE', `/shares/${at(id)}`),
  remotes: () => get<{ remotes: Remote[]; rclone: Rclone }>('/remotes'),
  addRemote: (r: RemoteInput) => write<Remote>('POST', '/remotes', r),
  updateRemote: (name: string, r: RemoteInput) => write<Remote>('PUT', `/remotes/${at(name)}`, r),
  deleteRemote: (name: string) => write('DELETE', `/remotes/${at(name)}`),
  mountRemote: (name: string, on: boolean) => write<Remote>('POST', `/remotes/${at(name)}/${on ? 'mount' : 'unmount'}`),
  // With the name of a saved remote, its stored password fills in the one the form leaves empty.
  testRemote: (r: RemoteInput) => write<RemoteTest>('POST', '/remotes/test', r),
  remoteToken: (type: string, pasted: string) => write<{ token?: string; expiry?: string }>('POST', '/remotes/oauth', { type, token: pasted }),
  system: async (): Promise<System> => { const s = await get<Wire<System, 'started'>>('/system'); return { ...s, started: ms(s.started)! }; },
  checkUpdate: () => write<UpdateCheck>('POST', '/system/update'),
  notifyUnraid: (subject: string, description: string) => write('POST', '/unraid/notify/test', { subject, description }),
};

export const me = {
  account: async (): Promise<Account> => user(await get<Wire<Account, 'last' | 'pwChanged'>>('/me')),
  password: (current: string, next: string) => write<{ ended: number }>('PUT', '/me/password', { current, next }),
  sessions: async () => (await items<Wire<Session, 'created' | 'seen'>>('/me/sessions')).map(session),
  endSession: (id: string) => write('DELETE', `/me/sessions/${at(id)}`),
  tokens: async () => (await items<Wire<Token, 'created' | 'last' | 'expires'>>('/me/tokens')).map(token),
  // password is the current one, for an account that signs in with one.
  createToken: (name: string, rights: Token['rights'], days: number | null, password?: string) =>
    write<{ secret: string }>('POST', '/me/tokens', { name, rights, days, password }),
  deleteToken: (id: string) => write('DELETE', `/me/tokens/${at(id)}`),
  passkeys: async (): Promise<Passkeys> => { const p = await get<{ blocked?: Problem; items: Wire<Passkey, 'created' | 'last'>[] }>('/me/passkeys'); return { blocked: p.blocked ?? null, items: p.items.map(passkey) }; },
  // Two steps on one path: the name brings the options, the browser's answer to them adds the passkey.
  async addPasskey(name: string, password?: string): Promise<void> {
    const { options } = await write<{ options: { publicKey: Record<string, unknown> } }>('POST', '/me/passkeys', { name, password });
    const credential = credentialJSON(await navigator.credentials.create(creationOptions(options)));
    await write('POST', '/me/passkeys', { name, credential });
  },
  deletePasskey: (id: string) => write('DELETE', `/me/passkeys/${at(id)}`),
  // The server draws the QR code, so no encoder ships to the browser.
  totpBegin: () => write<{ secret: string; uri: string; qr: string }>('POST', '/me/totp', {}),
  totpConfirm: (code: string) => write<{ codes: string[] }>('POST', '/me/totp', { code }),
  totpCodes: (code: string) => write<{ codes: string[] }>('POST', '/me/totp', { code, regenerate: true }),
  totpOff: (code: string) => write('DELETE', '/me/totp', { code }),
};

// Whether a folder or, with files, a file exists, asked the way the file list asks.
export async function exists(path: string, kind: 'dir' | 'file' = 'dir'): Promise<boolean> {
  if (kind === 'dir') return call(`/fs/list${q({ path })}`).then(() => true, () => false);
  const cut = path.lastIndexOf('/');
  const parent = path.slice(0, cut) || '/', name = path.slice(cut + 1);
  return call<{ items: { name: string; dir: boolean }[] }>(`/fs/list${q({ path: parent })}`)
    .then((r) => !!r.body?.items.some((k) => k.name === name && !k.dir), () => false);
}

export async function listDir(path: string) {
  return (await call<{ items: { name: string; dir: boolean }[] }>(`/fs/list${q({ path })}`)).body?.items ?? [];
}
