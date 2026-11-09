// The settings pages, the account and the labels in the shape of /api/v1, as the contracts in
// backend/internal/*/openapi.yaml describe it, with the prototype's made-up people and sources. It keeps changes in
// memory, so a page can be clicked through as it will work against the server. Share links live in fixture-tools.js.
import { PASSWORDS } from './fixture-auth.js';
import { clock } from './fixture-clock.js';
import { createFixture } from './fixture.js';

const MIN = 60000, HOUR = 60 * MIN, DAY = 24 * HOUR, KB = 1024, MB = KB * 1024, GB = MB * 1024, TB = GB * 1024;
const iso = (t) => (t == null ? null : new Date(t).toISOString());
// A refusal may carry the code the interface translates, as [status, detail, code].
const json = (res, status, body, code) => {
  res.statusCode = status;
  res.setHeader('Content-Type', status >= 400 ? 'application/problem+json' : 'application/json');
  res.end(status === 204 ? '' : JSON.stringify(status >= 400 ? { type: 'about:blank', title: 'Error', status, detail: body, ...(code ? { code } : {}) } : body));
};
const readBody = (req) => new Promise((resolve) => {
  let raw = '';
  req.on('data', (c) => { raw += c; });
  req.on('end', () => { try { resolve(raw ? JSON.parse(raw) : {}); } catch { resolve({}); } });
});

// The prototype's three-finder pattern, seeded from the secret, so the code reads as a QR code.
function qr(secret) {
  const n = 25;
  let s = [...secret].reduce((a, c) => (a * 31 + c.charCodeAt(0)) % 2147483647, 7);
  const next = () => (s = (s * 16807) % 2147483647) / 2147483647;
  const finder = (x, y) => (x < 7 && y < 7) || (x >= n - 7 && y < 7) || (x < 7 && y >= n - 7);
  const inFinder = (x, y) => {
    const fx = x >= n - 7 ? x - (n - 7) : x, fy = y >= n - 7 ? y - (n - 7) : y;
    return fx === 0 || fx === 6 || fy === 0 || fy === 6 || (fx >= 2 && fx <= 4 && fy >= 2 && fy <= 4);
  };
  let rects = '';
  for (let y = 0; y < n; y++) for (let x = 0; x < n; x++) {
    const on = finder(x, y) ? inFinder(x, y) : (x === 7 || y === 7 || x === n - 8 || y === n - 8) && (finder(x - 1, y) || finder(x, y - 1) || finder(x + 1, y) || finder(x, y + 1)) ? false : next() > 0.52;
    if (on) rects += `<rect x="${x + 2}" y="${y + 2}" width="1" height="1"/>`;
  }
  return `<svg viewBox="0 0 ${n + 4} ${n + 4}" shape-rendering="crispEdges"><rect width="${n + 4}" height="${n + 4}" fill="#fff"/><g fill="#161616">${rects}</g></svg>`;
}
// The secrets and codes only have to look random. Every fixture server draws them from the same sequence, so the
// windows that show them, and the QR code of the secret, come out the same from run to run.
let seed = 0;
const next = () => {
  seed = (seed + 0x6d2b79f5) | 0;
  let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
  t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};
const randomOf = (alphabet, n) => Array.from({ length: n }, () => alphabet[Math.floor(next() * alphabet.length)]).join('');
const codes = () => Array.from({ length: 8 }, () => `${randomOf('ABCDEFGHJKLMNPQRSTUVWXYZ23456789', 4)}-${randomOf('ABCDEFGHJKLMNPQRSTUVWXYZ23456789', 4)}`);
const b64url = (n) => randomOf('ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_', n);

function mcpKey(id, owner, label, client, hint, canWrite, created, last, calls) {
  const sec = (t) => Math.floor(t / 1000);
  return {
    id, owner, viaOAuth: false, label, client, hint, canWrite, createdAt: sec(created), rotatedAt: 0, lastUsedAt: sec(last), lastUsedFrom: '192.168.10.16',
    revokedAt: 0, revokedReason: '', unusable: '', callsToday: calls,
  };
}

function state() {
  const now = clock.now();
  const person = (id, name, full, role, scope, groups, last, more = {}) => ({
    id, name, full, role, scope, groups, loginMethod: 'password', twofa: false, passkeys: 0, last, pwChanged: null, ...more,
  });
  return {
    started: now - 3 * DAY - 4 * HOUR,
    users: [
      person('1', 'jdp', 'Junker der Provinz', 'admin', '/mnt/user', ['admins', 'familie'], now - 3 * MIN, { twofa: true, passkeys: 2, pwChanged: now - 41 * DAY }),
      person('7340032', 'anna', 'Anna', 'user', '/mnt/user/fotos', ['familie'], now - 2 * DAY),
      person('9007199254740993', 'gast', 'Gast', 'readonly', '/mnt/user/filme', [], now - 40 * DAY),
    ],
    recovery: 8,
    groups: [
      { name: 'admins', members: ['jdp'], note: 'Alles, auch Einstellungen' },
      { name: 'familie', members: ['jdp', 'anna'], note: 'Fotos, Musik, Filme' },
    ],
    sources: [
      { name: 'Shares', path: '/mnt/user', readOnly: false, indexed: true, skipHidden: true, exclude: ['*.tmp', 'appdata/plex/cache'] },
      { name: 'Cache', path: '/mnt/cache', readOnly: false, indexed: true, skipHidden: true, exclude: [] },
      { name: 'USB-Stick', path: '/boot', readOnly: true, indexed: false, skipHidden: true, exclude: [] },
      { name: 'Fernziele', path: '/remotes', readOnly: false, indexed: false, skipHidden: true, exclude: [] },
    ],
    // What the server runs with, so a saved list that differs asks for a restart.
    running: '',
    index: {
      Shares: { dirs: 14211, files: 182344, size: Math.round(6.6 * TB), last: now - 5.5 * HOUR, next: now + 25 * MIN, every: 40 * 60 },
      Cache: { dirs: 801, files: 4021, size: 580 * GB, last: now - 20 * MIN, next: now + 40 * MIN, every: 60 * 60 },
    },
    reading: new Set(),
    rules: [
      { who: 'anna', path: '/mnt/user/backups', rule: 'hide' },
      { who: 'gast', path: '/mnt/user/filme', rule: 'read' },
      { who: '*', path: '*.partial*', rule: 'hide' },
    ],
    activity: [
      { t: now - 4 * MIN, who: 'jdp', what: 'copy', path: '/mnt/user/downloads/neu', target: '/mnt/user/backups/neu', ip: '192.168.10.16' },
      { t: now - 40 * MIN, who: 'jdp', what: 'rename', path: '/mnt/user/fotos/2026', ip: '192.168.10.16' },
      { t: now - 3 * HOUR, who: 'anna', what: 'upload', path: '/mnt/user/fotos/Urlaub', ip: '192.168.30.50' },
      { t: now - 5 * HOUR, who: 'jdp', what: 'shareCreate', path: '/mnt/user/fotos/Urlaub', ip: '192.168.10.16' },
      { t: now - 1 * DAY, who: 'gast', what: 'login', ip: '192.168.30.50' },
      { t: now - 2 * DAY, who: 'jdp', what: 'delete', path: '/mnt/user/dokumente/alte-notizen.txt', ip: '192.168.10.16' },
      { t: now - 3 * DAY, who: 'jdp', what: 'tokenCreate', obj: 'backup-skript', ip: '192.168.10.16' },
    ].map((a, i) => ({ id: 100 - i, details: {}, obj: a.obj ?? (a.target ? `${a.path} → ${a.target}` : a.path ?? a.ip), ...a })),
    sessions: [
      { id: 'a1', created: now - 9 * DAY, seen: now - MIN, userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0', ip: '192.168.10.16', current: true },
      { id: 'a2', created: now - 20 * DAY, seen: now - 2 * HOUR, userAgent: 'Mozilla/5.0 (Android 15; Mobile; rv:140.0) Gecko/140.0 Firefox/140.0', ip: '192.168.30.50', current: false },
    ],
    tokens: [{ id: 't1', name: 'backup-skript', created: now - 30 * DAY, last: now - DAY, rights: 'rw', expires: null }],
    passkeys: [
      { id: 'p1', name: 'Windows Hello', origin: 'cannon.tower.lan', created: now - 60 * DAY, last: now - 3 * MIN },
      { id: 'p2', name: 'Handy', origin: '192.168.10.5:8443', created: now - 120 * DAY, last: now - 50 * DAY },
    ],
    // The sessions, tokens and passkeys of the other people, as an admin sees them.
    others: {
      anna: {
        sessions: [{ id: 'b1', created: now - 2 * DAY, seen: now - 2 * DAY, userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.5 Mobile/15E148 Safari/604.1', ip: '192.168.30.61', current: false }],
        tokens: [{ id: 't2', name: 'fotos-sync', created: now - 90 * DAY, last: now - 5 * HOUR, rights: 'r', expires: now + 275 * DAY }],
        passkeys: [{ id: 'p3', name: 'iPhone', origin: 'cannon.tower.lan', created: now - 80 * DAY, last: now - 2 * DAY }],
      },
    },
    remotes: [
      { name: 'gdrive', type: 'drive', path: '', mount: '/remotes/gdrive', label: 'rclone: gdrive', mounted: true, mountOnStart: true, params: {}, secrets: ['token'] },
      { name: 'nas-keller', type: 'smb', path: 'archiv', mount: '/remotes/nas-keller', label: 'rclone: nas-keller', mounted: true, mountOnStart: true, params: { host: 'keller', user: 'jdp', domain: '' }, secrets: ['pass'] },
    ],
    app: {
      office: { kind: 'none', url: '', secret: '' },
      arrowloop: { url: 'http://tower.lan:8095', token: 'al-token', paths: { '/mnt/user': '/data' } },
      previews: { thumbs: true, video: false }, webdav: false, terminal: false,
      remotes: { automount: true, cache: 'writes', cacheSize: 50 },
      updates: { autoCheck: true, autoUpdate: false }, logLevel: 'info',
    },
    // The switches the server only reads when it starts.
    booted: { autoCheck: true },
    colours: [['red', '#ff6b72'], ['orange', '#ff8a3d'], ['yellow', '#f5c518'], ['green', '#4cc977'], ['blue', '#3b9cf5'], ['purple', '#a98bff']].map(([id, hex]) => ({ id, name: '', hex })),
    tags: [],
    labels: {
      '/mnt/user/dokumente/Mietvertrag.pdf': { colour: 'red', tags: ['wichtig'] },
      '/mnt/user/dokumente/Steuer 2025.xlsx': { colour: 'orange', tags: ['prüfen', 'Steuer'] },
      '/mnt/user/dokumente/Rechnung-Oktober.pdf': { tags: ['Steuer'] },
      '/mnt/user/downloads/ubuntu-24.04.3-desktop-amd64.iso': { colour: 'green', tags: ['fertig'] },
      '/mnt/user/fotos/2026/IMG_0203.jpg': { colour: 'purple', tags: ['für Anna'] },
    },
    exports: [],
    totpSecret: '',
    // The MCP server's keys in the shape GET /mcp/keys sends them, with Unix seconds.
    mcp: {
      keys: [
        mcpKey('a1b2c3d4e5f60718293a4b5c6d7e8f90', 'jdp', 'Claude Code auf dem Laptop', 'claude-code', 'q7Xe', false, now - 9 * DAY, now - 12 * MIN, 14),
        mcpKey('0f1e2d3c4b5a69788796a5b4c3d2e1f0', 'jdp', 'Cursor', 'cursor', 'Lm3k', true, now - 30 * DAY, now - 2 * DAY, 0),
        mcpKey('9a8b7c6d5e4f30211203f4e5d6c7b8a9', 'anna', 'LM Studio', 'lmstudio', 'Zr8p', false, now - 6 * DAY, now - 3 * HOUR, 5),
      ],
      revoked: [{ ...mcpKey('5c4b3a29180716253443526170819203', 'jdp', 'Erster Versuch', '', 'Tt0a', false, now - 40 * DAY, now - 39 * DAY, 0), revokedAt: Math.floor((now - 38 * DAY) / 1000), revokedReason: 'owner' }],
      events: [
        { at: Math.floor((now - 12 * MIN) / 1000), tool: 'list_folder', outcome: 'ok' },
        { at: Math.floor((now - 13 * MIN) / 1000), tool: 'move_items', outcome: 'read_only_key' },
        { at: Math.floor((now - 14 * MIN) / 1000), tool: 'search_files', outcome: 'ok' },
      ],
      oauth: { enabled: false, issuer: '' },
    },
  };
}

const split = (p) => { const at = p.lastIndexOf('/'); return [p.slice(0, at) || '/', p.slice(at + 1)]; };
const APPDATA = '/mnt/user/appdata/cannoncommander', EXPORTS = APPDATA + '/exports';
const ME = 'jdp';

export function settingsFixtureMiddleware() {
  seed = 0;
  const fx = createFixture();
  const S = state();
  S.running = JSON.stringify(S.sources);
  const isDir = (p) => !!fx.list(p);
  const isFile = (p) => { const [dir, name] = split(p); return S.exports.some((e) => e.path === p) || !!fx.list(dir)?.items.some((k) => k.name === name && !k.dir); };
  const me = () => S.users.find((u) => u.name === ME);
  const passwords = [...PASSWORDS];
  const accessOf = (u) => (u.name === ME ? S : (S.others[u.name] ??= { sessions: [], tokens: [], passkeys: [] }));
  const userAt = (id) => S.users.find((x) => x.name === id || x.id === id);
  const wrongPassword = (b) => (!b.password ? [400, 'the current password is missing'] : passwords.includes(b.password) ? null : [403, 'the current password is not right']);
  const inside = (p, root) => p === root || p.startsWith(root.replace(/\/$/, '') + '/');

  const verdict = (user, path) => {
    if (!isDir(path) && !isFile(path)) return { result: 'missing', reason: 'none' };
    if (!inside(path, user.scope)) return { result: 'hidden', reason: 'scope', scope: user.scope };
    const glob = (pat, s) => new RegExp('^' + pat.replace(/[.+?^${}()|[\]\\]/g, '\\$&').replace(/\*/g, '.*') + '$', 'i').test(s);
    const i = S.rules.findIndex((r) => {
      const who = r.who === '*' || r.who === user.name || (r.who.startsWith('@') && user.groups.includes(r.who.slice(1)));
      if (!who) return false;
      if (r.path.includes('*')) return path.split('/').some((part) => glob(r.path, part));
      return inside(path, r.path);
    });
    const source = S.sources.find((s) => inside(path, s.path));
    if (i >= 0 && S.rules[i].rule === 'hide') return { result: 'hidden', reason: 'rule', rule: i };
    if (i >= 0 && S.rules[i].rule === 'read') return { result: 'read', reason: 'rule', rule: i };
    if (user.role === 'readonly') return { result: 'read', reason: 'role' };
    if (source?.readOnly) return { result: 'read', reason: 'source' };
    return i >= 0 ? { result: 'visible', reason: 'rule', rule: i } : { result: 'visible', reason: 'none' };
  };
  const lists = () => {
    const count = {};
    for (const l of Object.values(S.labels)) {
      if (l.colour) count['c:' + l.colour] = (count['c:' + l.colour] || 0) + 1;
      for (const t of l.tags || []) count['t:' + t] = (count['t:' + t] || 0) + 1;
    }
    const tags = [...new Set([...Object.values(S.labels).flatMap((l) => l.tags || []), ...S.tags])].sort((a, b) => a.localeCompare(b, 'de'));
    return { colours: S.colours.map((c) => ({ ...c, count: count['c:' + c.id] || 0 })), tags: tags.map((name) => ({ name, count: count['t:' + name] || 0 })) };
  };
  const tidy = () => { for (const [p, l] of Object.entries(S.labels)) if (!l.colour && !(l.tags || []).length) delete S.labels[p]; };
  const wireUser = (u) => ({ ...u, last: iso(u.last), pwChanged: iso(u.pwChanged) });
  const wireRemote = (r) => ({ ...r, params: { ...r.params } });
  const sources = () => ({ sources: S.sources, devices: [], restartNeeded: JSON.stringify(S.sources) !== S.running });
  const indexState = (s) => {
    const ix = S.index[s.name];
    const base = { source: s.name, path: s.path, status: 'off', dirs: 0, files: 0, size: 0, quickScanSeconds: 0, fullScanSeconds: 0, metadata: 0 };
    if (!s.indexed || !ix) return base;
    const reading = S.reading.has(s.name);
    return {
      ...base, status: reading ? 'indexing' : 'ready', dirs: ix.dirs, files: ix.files, size: ix.size, lastIndexed: iso(ix.last),
      quickScanSeconds: 2, fullScanSeconds: 41, nextScan: iso(ix.next), scanEverySeconds: ix.every, metadata: Math.round(ix.files / 3),
      ...(reading ? { job: 'j-' + s.name, scanning: s.path + '/fotos' } : {}),
    };
  };
  const app = () => ({
    ...S.app,
    office: { kind: S.app.office.kind, url: S.app.office.url, secretSet: !!S.app.office.secret },
    arrowloop: { url: S.app.arrowloop.url, tokenSet: !!S.app.arrowloop.token, ...(Object.keys(S.app.arrowloop.paths).length ? { paths: S.app.arrowloop.paths } : {}) },
    restartNeeded: S.app.updates.autoCheck !== S.booted.autoCheck,
  });
  const badUrl = (u) => !!u && !/^https?:\/\/[^/\s]+/.test(u);

  const routes = [
    ['GET', '/me', () => ({ ...wireUser(me()), home: '/mnt/user', recovery: S.recovery, lockPassword: false })],
    ['PUT', '/me/password', (b) => {
      if (!b.next || b.next.length < 12) return [400, 'the password needs 12 characters'];
      if (!passwords.includes(b.current)) return [403, 'the current password is wrong'];
      passwords.push(b.next);
      const ended = S.sessions.filter((s) => !s.current).length;
      S.sessions = S.sessions.filter((s) => s.current);
      me().pwChanged = clock.now();
      return { ended };
    }],
    ['GET', '/me/sessions', () => ({ items: S.sessions.map((s) => ({ ...s, created: iso(s.created), seen: iso(s.seen) })) })],
    ['DELETE', /^\/me\/sessions\/(.+)$/, (_, [id]) => { S.sessions = S.sessions.filter((s) => s.id !== id); return 204; }],
    ['GET', '/me/tokens', () => ({ items: S.tokens.map((t) => ({ ...t, created: iso(t.created), last: iso(t.last), expires: iso(t.expires) })) })],
    ['POST', '/me/tokens', (b) => {
      const refused = wrongPassword(b);
      if (refused) return refused;
      if (S.tokens.some((t) => t.name === b.name)) return [409, 'a token with this name exists'];
      const t = { id: 't' + clock.now(), name: b.name, created: clock.now(), last: null, rights: b.rights, expires: b.days ? clock.now() + b.days * DAY : null };
      S.tokens.push(t);
      return [201, { ...t, created: iso(t.created), last: null, expires: iso(t.expires), secret: 'cc_' + b64url(40) }];
    }],
    ['DELETE', /^\/me\/tokens\/(.+)$/, (_, [id]) => { S.tokens = S.tokens.filter((t) => t.id !== id); return 204; }],
    ['GET', '/me/passkeys', () => ({ enabled: true, items: S.passkeys.map((p) => ({ ...p, created: iso(p.created), last: iso(p.last) })) })],
    ['POST', '/me/passkeys', (b, _, __, req) => {
      if (S.passkeys.some((p) => p.name === b.name)) return [409, 'the name is taken'];
      const host = (req.headers.host || 'localhost').split(':')[0];
      if (!b.credential) {
        const refused = wrongPassword(b);
        if (refused) return refused;
        return { options: { publicKey: {
          challenge: b64url(43), rp: { name: 'CannonCommander', id: host }, user: { id: b64url(11), name: ME, displayName: me().full },
          pubKeyCredParams: [{ type: 'public-key', alg: -7 }, { type: 'public-key', alg: -257 }], timeout: 300000,
          authenticatorSelection: { residentKey: 'preferred', userVerification: 'preferred' },
        } } };
      }
      const p = { id: 'p' + clock.now(), name: b.name, origin: req.headers.host || host, created: clock.now(), last: null };
      S.passkeys.push(p);
      me().passkeys = S.passkeys.length;
      return [201, { passkey: { ...p, created: iso(p.created) } }];
    }],
    ['DELETE', /^\/me\/passkeys\/(.+)$/, (_, [id]) => { S.passkeys = S.passkeys.filter((p) => p.id !== id); me().passkeys = S.passkeys.length; return 204; }],
    ['POST', '/me/totp', (b) => {
      if (b.code === '000000') return [400, 'the code does not match'];
      if (b.regenerate) { S.recovery = 8; return { codes: codes() }; }
      if (b.code) {
        if (me().twofa) return [409, 'two factor login is on already'];
        me().twofa = true;
        S.recovery = 8;
        return { codes: codes() };
      }
      S.totpSecret = randomOf('ABCDEFGHIJKLMNOPQRSTUVWXYZ234567', 32);
      return { secret: S.totpSecret, uri: `otpauth://totp/CannonCommander:${ME}?secret=${S.totpSecret}`, qr: qr(S.totpSecret) };
    }],
    ['DELETE', '/me/totp', (b) => (b.code === '000000' ? [400, 'the code does not work'] : (me().twofa = false, 204))],

    ['GET', '/admin/users', () => ({ items: S.users.map(wireUser) })],
    ['GET', /^\/admin\/users\/([^/]+)\/(sessions|tokens|passkeys)$/, (_, [id, kind]) => {
      const u = userAt(id);
      if (!u) return [404, 'user'];
      const wire = {
        sessions: (x) => ({ ...x, current: u.name === ME && x.current, created: iso(x.created), seen: iso(x.seen) }),
        tokens: (x) => ({ ...x, created: iso(x.created), last: iso(x.last), expires: iso(x.expires) }),
        passkeys: (x) => ({ ...x, created: iso(x.created), last: iso(x.last) }),
      }[kind];
      return { items: accessOf(u)[kind].map(wire) };
    }],
    ['DELETE', /^\/admin\/users\/([^/]+)\/(sessions|tokens|passkeys)\/([^/]+)$/, (_, [id, kind, item]) => {
      const u = userAt(id), access = u && accessOf(u);
      if (!access?.[kind].some((x) => x.id === item)) return [404, kind];
      access[kind] = access[kind].filter((x) => x.id !== item);
      if (kind === 'passkeys') u.passkeys = access.passkeys.length;
      return 204;
    }],
    ['POST', '/admin/users', (b) => {
      if (S.users.some((u) => u.name === b.name)) return [409, 'the login name is taken'];
      if (!b.password || b.password.length < 12) return [400, 'the password needs 12 characters'];
      const u = { id: String(clock.now()), name: b.name, full: b.full || b.name, role: b.role || 'user', scope: b.scope, groups: b.groups || [], loginMethod: 'password', twofa: false, passkeys: 0, last: null, pwChanged: clock.now() };
      S.users.push(u);
      syncGroups(u.name, u.groups);
      return [201, wireUser(u)];
    }],
    ['PATCH', /^\/admin\/users\/(.+)$/, (b, [id]) => {
      const u = S.users.find((x) => x.name === id || x.id === id);
      if (!u) return [404, 'user'];
      const was = u.name;
      if (b.name && b.name !== was && was === ME) return [400, 'you cannot change your own login name'];
      if (was === ME && b.role && b.role !== 'admin') return [409, 'you cannot give up your own admin role'];
      if (b.name && b.name !== was) S.rules.forEach((r) => { if (r.who === was) r.who = b.name; });
      if (b.revoke && was === ME) return [400, 'end your own sessions, tokens and passkeys under account and security'];
      const { password, revoke, ...rest } = b;
      Object.assign(u, rest, password ? { pwChanged: clock.now() } : {});
      if (password && was !== ME) accessOf(u).sessions = [];
      if (revoke) {
        Object.assign(accessOf(u), { sessions: [], tokens: [], passkeys: [] });
        u.passkeys = 0;
      }
      if (b.groups || b.name) syncGroups(u.name, u.groups, was);
      return wireUser(u);
    }],
    ['DELETE', /^\/admin\/users\/(.+)$/, (_, [id]) => {
      const u = S.users.find((x) => x.name === id || x.id === id);
      if (!u) return [404, 'user'];
      if (u.name === ME) return [403, 'nobody can delete themselves'];
      S.users = S.users.filter((x) => x !== u);
      S.groups.forEach((g) => { g.members = g.members.filter((m) => m !== u.name); });
      S.rules = S.rules.filter((r) => r.who !== u.name);
      return 204;
    }],
    ['GET', '/admin/groups', () => ({ items: S.groups })],
    ['POST', '/admin/groups', (b) => {
      if (S.groups.some((g) => g.name === b.name)) return [409, 'the group exists'];
      const g = { name: b.name, note: b.note || '', members: b.members || [] };
      S.groups.push(g);
      syncMembers(g);
      return [201, g];
    }],
    ['PATCH', /^\/admin\/groups\/(.+)$/, (b, [name]) => {
      const g = S.groups.find((x) => x.name === name);
      if (!g) return [404, 'group'];
      if (b.name && b.name !== name) {
        if (S.groups.some((x) => x.name === b.name)) return [409, 'a group with the new name exists'];
        S.users.forEach((u) => { u.groups = u.groups.map((x) => (x === name ? b.name : x)); });
        S.rules.forEach((r) => { if (r.who === '@' + name) r.who = '@' + b.name; });
      }
      Object.assign(g, b);
      syncMembers(g);
      return g;
    }],
    ['DELETE', /^\/admin\/groups\/(.+)$/, (_, [name]) => {
      S.groups = S.groups.filter((g) => g.name !== name);
      S.users.forEach((u) => { u.groups = u.groups.filter((x) => x !== name); });
      S.rules = S.rules.filter((r) => r.who !== '@' + name);
      return 204;
    }],
    ['GET', '/admin/rules', () => ({ items: S.rules })],
    ['PUT', '/admin/rules', (b) => {
      const bad = b.items.findIndex((r) => !r.path.includes('*') && !isDir(r.path) && !isFile(r.path));
      if (bad >= 0) return [400, `rule ${bad + 1}: the path does not exist`];
      S.rules = b.items;
      return { items: S.rules };
    }],
    ['POST', '/admin/rules/test', (b) => { const u = S.users.find((x) => x.name === b.user); return u ? verdict(u, b.path) : [404, 'there is no such user']; }],
    ['GET', '/admin/activity', (_, __, q) => {
      const limit = Number(q.get('limit') || 50);
      const items = S.activity.slice(0, limit).map((a) => ({ ...a, t: iso(a.t) }));
      return { items, total: S.activity.length, page: 1, limit };
    }],
    ['GET', '/admin/sources', sources],
    ['PUT', '/admin/sources', (b) => {
      if (!b.sources?.length) return [400, 'keep at least one source'];
      for (const s of b.sources) {
        if (!isDir(s.path) && !['/boot', '/mnt/cache', '/remotes'].includes(s.path)) return [400, `${s.path} is not a folder on the server`];
        if ((s.exclude || []).some((p) => p.includes('/') && p.includes('*'))) return [400, 'a star works in a name, not in a path'];
      }
      S.sources = b.sources.map((s) => ({ readOnly: false, indexed: false, skipHidden: false, exclude: [], ...s }));
      return sources();
    }],
    ['GET', '/admin/index', () => ({ sources: JSON.parse(S.running).map(indexState) })],
    ['POST', '/admin/index', (b) => {
      const names = JSON.parse(S.running).filter((s) => s.indexed && S.index[s.name] && (!b.source || s.name === b.source)).map((s) => s.name);
      if (!names.length) return [400, b.source ? `the source ${b.source} has no index` : 'no source has an index'];
      for (const name of names) {
        S.reading.add(name);
        clock.setTimeout(() => { S.reading.delete(name); S.index[name].last = clock.now(); }, 2500);
      }
      return [202, { id: 'j' + clock.now(), kind: 'reindex', name: names.length === 1 ? names[0] : 'All sources', state: 'queued', owner: ME, createdAt: iso(clock.now()) }];
    }],
    ['GET', '/admin/app', app],
    ['PUT', '/admin/app', (b) => {
      if (b.logLevel && !['error', 'info', 'debug'].includes(b.logLevel)) return [400, 'log level must be error, info or debug'];
      if (badUrl(b.office?.url) || badUrl(b.arrowloop?.url)) return [400, 'the address starts with http:// or https://'];
      const office = { ...S.app.office, ...b.office };
      if (office.kind !== 'none' && !office.url) return [400, 'the office server needs an address'];
      if (b.arrowloop?.paths && Object.entries(b.arrowloop.paths).some(([h, t]) => !h.startsWith('/') || !t.startsWith('/'))) return [400, 'the ArrowLoop paths have to be full paths'];
      S.app = {
        ...S.app, ...b, office, arrowloop: { ...S.app.arrowloop, ...b.arrowloop }, previews: { ...S.app.previews, ...b.previews },
        remotes: { ...S.app.remotes, ...b.remotes }, updates: { ...S.app.updates, ...b.updates },
      };
      delete S.app.restartNeeded;
      return app();
    }],
    ['POST', '/admin/app/restart', () => { S.started = clock.now(); S.running = JSON.stringify(S.sources); S.booted = { autoCheck: S.app.updates.autoCheck }; return [202, {}]; }],
    ['POST', '/admin/app/export', (b) => {
      if (isFile(b.path)) return [409, 'there is a file of that name already'];
      S.exports.push({ path: b.path, time: clock.now() });
      return [201, { path: b.path, size: 14 * KB }];
    }],
    ['POST', '/admin/app/import', (b) => (isFile(b.path) ? { sections: ['sources', 'app', 'remotes'], notes: [], restartNeeded: true } : [404, 'path not found'])],
    ['POST', '/admin/integrations/test', (b) => {
      const reachable = !b.url.includes('nirgends');
      const saved = b.kind === 'arrowloop' ? S.app.arrowloop.token : S.app.office.secret;
      const key = b.kind === 'arrowloop' ? b.token : b.secret;
      const token = b.kind === 'collabora' ? 'none' : key || saved ? 'ok' : 'missing';
      return { kind: b.kind, ok: reachable && ['ok', 'none'].includes(token), reachable, ...(reachable ? { token, version: '8.3.2' } : { error: 'connection refused' }) };
    }],
    ['GET', '/remotes', () => ({ remotes: S.remotes.map(wireRemote), rclone: { installed: true, running: true, version: '1.71.2' } })],
    ['POST', '/remotes', (b) => {
      if (S.remotes.some((r) => r.name === b.name)) return [409, 'the name is taken'];
      const params = { ...b.params }, secrets = [];
      for (const k of ['pass', 'secret_access_key', 'token']) if (params[k]) { secrets.push(k); delete params[k]; }
      const r = { name: b.name, type: b.type, path: b.path || '', mount: b.mount || '/remotes/' + b.name, label: 'rclone: ' + b.name, mounted: true, mountOnStart: true, params, secrets };
      S.remotes.push(r);
      return [201, wireRemote(r)];
    }],
    ['PUT', /^\/remotes\/([^/]+)$/, (b, [name]) => {
      const r = S.remotes.find((x) => x.name === name);
      if (!r) return [404, 'remote'];
      for (const [k, v] of Object.entries(b.params || {})) {
        if (['pass', 'secret_access_key', 'token'].includes(k)) { if (v && !r.secrets.includes(k)) r.secrets.push(k); } else if (v === '') delete r.params[k]; else r.params[k] = v;
      }
      Object.assign(r, { name: b.name || r.name, path: b.path ?? r.path, mount: b.mount || r.mount });
      r.label = 'rclone: ' + r.name;
      return wireRemote(r);
    }],
    ['DELETE', /^\/remotes\/([^/]+)$/, (_, [name]) => { S.remotes = S.remotes.filter((r) => r.name !== name); return 204; }],
    ['POST', /^\/remotes\/([^/]+)\/(mount|unmount)$/, (_, [name, how]) => {
      const r = S.remotes.find((x) => x.name === name);
      if (!r) return [404, 'remote'];
      r.mounted = r.mountOnStart = how === 'mount';
      return wireRemote(r);
    }],
    ['POST', '/remotes/test', (b) => (b.params?.host === 'nirgends' ? { ok: false, entries: 0, error: 'dial tcp: no such host' } : { ok: true, entries: 12, total: 4 * TB, used: TB, free: 3 * TB })],
    ['POST', '/remotes/oauth', (b) => (b.token.trim().startsWith('{') ? { token: b.token.trim(), expiry: iso(clock.now() + HOUR) } : [400, 'this is not a token from rclone authorize'])],
    ['GET', '/system', () => ({
      version: '0.1.0', commit: 'f00dfeed', release: true, deployment: 'container', go: 'go1.27.0', os: 'linux', arch: 'amd64', started: iso(S.started), uptime: Math.round((clock.now() - S.started) / 1000),
      tools: {
        ffmpeg: { available: true, path: '/usr/bin/ffmpeg', version: '7.1.1' }, ffprobe: { available: true, path: '/usr/bin/ffprobe', version: '7.1.1' },
        mutool: { available: true, path: '/usr/bin/mutool', version: '1.26.3' }, '7zz': { available: true, path: '/usr/bin/7zz', version: '24.09' },
        rclone: { available: true, path: '/usr/bin/rclone', version: '1.71.2' },
      },
      unraid: { available: true, socket: false },
      features: { office: S.app.office.kind === 'none' ? { kind: 'none' } : { kind: S.app.office.kind, url: S.app.office.url }, thumbs: S.app.previews.thumbs, videoPreview: S.app.previews.video, webdav: S.app.webdav, mupdf: true, terminal: S.app.terminal },
    })],
    // Only the check: the WebSocket itself is fixture-terminal.js, on the upgrade of the same address.
    ['GET', '/terminal', (_, __, params) => {
      if (!S.app.terminal) return [403, 'the terminal is switched off', 'terminalOff'];
      const path = params.get('path') ?? '';
      if (!isDir(path)) return isFile(path) ? [400, 'a terminal opens in a folder on disk', 'notAFolder'] : [404, 'path not found', 'notFound'];
      return 204;
    }],
    ['POST', '/system/update', () => ({ version: '0.1.0', latest: '0.1.0', updateAvailable: false, releaseUrl: 'https://github.com/junkerderprovinz/cannoncommander/releases/tag/v0.1.0' })],
    ['POST', '/unraid/notify/test', () => 204],

    ['GET', '/labels', () => lists()],
    ['PUT', '/labels/colours', (b) => {
      const kept = new Set();
      S.colours = b.colours.map((c) => { const id = c.id || 'c' + clock.now() + Math.random().toString(36).slice(2, 6); kept.add(id); return { id, name: c.name, hex: c.hex }; });
      for (const l of Object.values(S.labels)) if (l.colour && !kept.has(l.colour)) delete l.colour;
      tidy();
      return lists();
    }],
    ['PUT', '/labels/tags', (b) => {
      const rename = new Map(b.tags.filter((t) => t.from).map((t) => [t.from, t.name]));
      for (const l of Object.values(S.labels)) l.tags = [...new Set((l.tags || []).filter((t) => rename.has(t)).map((t) => rename.get(t)))];
      S.tags = b.tags.map((t) => t.name);
      tidy();
      return lists();
    }],
    ['GET', '/labels/items', (_, __, q) => {
      const dir = q.get('path') || '/', items = {};
      for (const [p, l] of Object.entries(S.labels)) {
        const [d, name] = split(p);
        if (d === dir) items[name] = l;
      }
      return { path: dir, items };
    }],
    ['PUT', '/labels/items', (b) => {
      const out = {};
      for (const p of b.paths) {
        const l = { tags: [], ...S.labels[p] };
        if (b.colour !== undefined) l.colour = b.colour || undefined;
        if (b.tags) l.tags = b.tags;
        if (b.add) l.tags = [...new Set([...l.tags, ...b.add])];
        if (b.remove) l.tags = l.tags.filter((t) => !b.remove.includes(t));
        if (l.colour || l.tags.length) S.labels[p] = l; else delete S.labels[p];
        out[p] = S.labels[p] || {};
      }
      return { items: out };
    }],
    ['GET', '/mcp/keys', (_, __, q) => {
      const view = (k) => ({ ...k, own: k.owner === ME });
      const mine = (k) => q.get('all') === 'true' || k.owner === ME;
      return {
        endpointPath: 'mcp', limit: 10, signIn: true, hostAllowsKeys: true, desktop: false,
        writeLimits: { entriesPerCall: 25, entriesPerHour: 200, filesPerCall: 10000, callsPerMinute: 120 },
        keys: S.mcp.keys.filter(mine).map(view), revoked: S.mcp.revoked.filter(mine).map(view),
        oauth: { possible: true, ...S.mcp.oauth, active: S.mcp.oauth.enabled && !!S.mcp.oauth.issuer, grantLimit: 10 },
      };
    }],
    ['POST', '/mcp/keys', (b) => {
      const refused = wrongPassword(b);
      if (refused) return refused;
      const label = (b.label || '').trim();
      if (!label || label.length > 64) return [400, 'a key needs a name of 1 to 64 characters', 'mcpKeyLabel'];
      if (S.mcp.keys.some((k) => k.owner === ME && k.label === label)) return [409, 'another active key has this name', 'mcpKeyLabelTaken'];
      const key = 'ccmcp_' + b64url(40);
      const item = mcpKey(randomOf('0123456789abcdef', 32), ME, label, b.client || '', key.slice(-4), !!b.canWrite, clock.now(), 0, 0);
      item.lastUsedAt = 0;
      S.mcp.keys.push(item);
      return [201, { key, item: { ...item, own: true } }];
    }],
    ['PATCH', /^\/mcp\/keys\/([0-9a-f]{32})$/, (b, [id]) => {
      const k = S.mcp.keys.find((x) => x.id === id);
      if (!k) return [404, 'there is no such key', 'mcpKeyNotFound'];
      if (b.canWrite && !k.canWrite) {
        if (k.owner !== ME) return [403, 'only the account a key belongs to can do this', 'mcpKeyNotYours'];
        const refused = wrongPassword(b);
        if (refused) return refused;
      }
      if (b.label !== undefined) k.label = b.label.trim();
      if (b.canWrite !== undefined) k.canWrite = b.canWrite;
      return { ...k, own: k.owner === ME };
    }],
    ['POST', /^\/mcp\/keys\/([0-9a-f]{32})\/rotate$/, (b, [id]) => {
      const k = S.mcp.keys.find((x) => x.id === id && x.owner === ME);
      if (!k) return [404, 'there is no such key', 'mcpKeyNotFound'];
      const refused = wrongPassword(b);
      if (refused) return refused;
      const key = 'ccmcp_' + b64url(40);
      Object.assign(k, { hint: key.slice(-4), rotatedAt: Math.floor(clock.now() / 1000) });
      return { key, item: { ...k, own: true } };
    }],
    ['POST', /^\/mcp\/keys\/([0-9a-f]{32})\/revoke$/, (_, [id]) => {
      const k = S.mcp.keys.find((x) => x.id === id);
      if (!k) return [404, 'there is no such key', 'mcpKeyNotFound'];
      S.mcp.keys = S.mcp.keys.filter((x) => x !== k);
      S.mcp.revoked.unshift({ ...k, revokedAt: Math.floor(clock.now() / 1000), revokedReason: k.owner === ME ? 'owner' : 'admin' });
      return 204;
    }],
    ['DELETE', /^\/mcp\/keys\/([0-9a-f]{32})$/, (_, [id]) => {
      if (S.mcp.keys.some((x) => x.id === id)) return [409, 'revoke the key before removing it', 'mcpKeyActive'];
      S.mcp.revoked = S.mcp.revoked.filter((x) => x.id !== id);
      return 204;
    }],
    ['GET', /^\/mcp\/keys\/([0-9a-f]{32})\/activity$/, (_, [id]) => ({ events: id === S.mcp.keys[0]?.id ? S.mcp.events : [] })],
    ['PUT', '/mcp/oauth', (b) => {
      const issuer = (b.issuer || '').trim().replace(/\/$/, '');
      if (b.enabled && !/^https:\/\/[^/\s?#]+$/.test(issuer)) return [400, 'the public address is an https address without a path', 'mcpOAuthIssuer'];
      S.mcp.oauth = { enabled: !!b.enabled, issuer };
      return { possible: true, ...S.mcp.oauth, active: S.mcp.oauth.enabled && !!issuer, grantLimit: 10 };
    }],
    ['GET', '/mcp/oauth/authorize', (_, __, q) => {
      if (!S.mcp.oauth.enabled) return [404, 'sign-in through OAuth is not switched on', 'oauthOff'];
      if (!q.get('client_id')) return [400, 'this client is not registered here', 'oauthUnknownClient'];
      return { client: { id: q.get('client_id'), name: 'Claude', known: 'claude', redirectHost: 'claude.ai', loopback: false }, account: ME, ticket: 'fixture-ticket', limitReached: false, grantLimit: 10 };
    }],
    ['POST', '/mcp/oauth/authorize', (b) => ({ redirect: `https://claude.ai/api/mcp/auth_callback?${b.allow ? 'code=fixture' : 'error=access_denied'}` })],
    // The app's own folder appears once the container has run, so exports have somewhere to go.
    ['GET', '/fs/list', (_, __, q) => {
      const p = q.get('path');
      if (p !== APPDATA && p !== EXPORTS) return undefined;
      const item = (name, dir, t, size = 0) => ({ name, dir, size, mtime: iso(t), mode: dir ? '0775' : '0664', owner: 'nobody', group: 'users' });
      if (p === APPDATA) return { path: p, items: [item('exports', true, clock.now() - 3 * DAY), item('config.yaml', false, clock.now() - 3 * DAY, 4 * KB), item('cannoncommander.db', false, clock.now() - 10 * MIN, 22 * MB)] };
      return { path: p, items: S.exports.map((e) => item(split(e.path)[1], false, e.time, 14 * KB)) };
    }],
  ];

  function syncGroups(user, groups, was = user) {
    S.groups.forEach((g) => {
      g.members = g.members.filter((m) => m !== was && m !== user);
      if (groups.includes(g.name)) g.members.push(user);
    });
  }
  function syncMembers(group) {
    S.users.forEach((u) => {
      const has = group.members.includes(u.name);
      u.groups = has ? [...new Set([...u.groups, group.name])] : u.groups.filter((x) => x !== group.name);
    });
  }

  const middleware = async (req, res, next) => {
    const url = new URL(req.url, 'http://fixture');
    if (!url.pathname.startsWith('/api/v1/')) return next();
    const path = url.pathname.slice('/api/v1'.length);
    for (const [method, match, fn] of routes) {
      if (method !== req.method) continue;
      const m = typeof match === 'string' ? (match === path ? [] : null) : path.match(match)?.slice(1).map(decodeURIComponent);
      if (!m) continue;
      const out = fn(req.method === 'GET' || !req.headers['content-length'] ? {} : await readBody(req), m, url.searchParams, req);
      if (out === undefined) return next();
      if (out === 204) return json(res, 204);
      if (Array.isArray(out)) return json(res, out[0], out[1], out[2]);
      return json(res, 200, out);
    }
    next();
  };
  return Object.assign(middleware, { terminalOn: () => S.app.terminal });
}
