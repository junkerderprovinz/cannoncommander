// The sign in and the public share links of the fixture, shaped like the contracts in backend/internal/accounts and
// backend/internal/shares. Every password is "cannoncommander", and the ones the prototype's step files type work
// too. jdp has two factor login with the code 123456 or the recovery code K7QM-3XRP, and neu has to pick a new
// password first. CC_FIXTURE_SIGNED_OUT=1 starts signed out.
// Links: k3x9qa-urlaub (folder with the password sonnenschein, takes uploads), m7tq2p-rechnung (one file),
// abgelaufen and aufgebraucht (both lead nowhere).
import { strToU8, zipSync } from 'fflate';
import { clock } from './fixture-clock.js';

const DAY = 86400000;
const PASSWORD = 'cannoncommander';
export const PASSWORDS = [PASSWORD, 'geheim', 'altes-passwort'];
const iso = (t) => new Date(t).toISOString();

const json = (res, status, body, headers = {}) => {
  res.statusCode = status;
  for (const [k, v] of Object.entries(headers)) res.setHeader(k, v);
  if (status === 204) return res.end();
  res.setHeader('Content-Type', 'application/json');
  res.end(JSON.stringify(body));
};
const problem = (res, status, detail, type, headers) => json(res, status, { ...(type ? { type } : {}), title: String(status), status, detail }, headers);
const readBody = (req) => new Promise((resolve) => {
  const parts = [];
  req.on('data', (c) => parts.push(c));
  req.on('end', () => resolve(Buffer.concat(parts)));
});

function picture(label, from, to) {
  return strToU8(`<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="800" viewBox="0 0 1200 800">
<defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="${from}"/><stop offset="1" stop-color="${to}"/></linearGradient></defs>
<rect width="1200" height="800" fill="url(#g)"/><circle cx="930" cy="210" r="110" fill="#fff6" />
<text x="80" y="720" font-family="sans-serif" font-size="72" fill="#fff">${label}</text></svg>`);
}

// A one page PDF, written out with the offsets its table needs.
function pdf(lines) {
  const text = lines.map((l, i) => `BT /F1 ${i ? 14 : 22} Tf 72 ${760 - i * 28} Td (${l}) Tj ET`).join('\n');
  const objs = [
    '<< /Type /Catalog /Pages 2 0 R >>',
    '<< /Type /Pages /Kids [3 0 R] /Count 1 >>',
    '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>',
    `<< /Length ${text.length} >>\nstream\n${text}\nendstream`,
    '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>',
  ];
  let out = '%PDF-1.4\n';
  const at = objs.map((o, i) => { const pos = out.length; out += `${i + 1} 0 obj\n${o}\nendobj\n`; return pos; });
  const xref = out.length;
  out += `xref\n0 ${objs.length + 1}\n0000000000 65535 f \n${at.map((p) => String(p).padStart(10, '0') + ' 00000 n \n').join('')}`;
  out += `trailer\n<< /Size ${objs.length + 1} /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`;
  return strToU8(out);
}

const file = (name, data, daysAgo) => ({ name, data, time: clock.now() - daysAgo * DAY });
const folder = (name, children, daysAgo = 3) => ({ name, dir: true, children, time: clock.now() - daysAgo * DAY });

function links() {
  const now = clock.now();
  return {
    'k3x9qa-urlaub': {
      id: 'urlaub', kind: 'folder', password: 'sonnenschein', uploads: true, room: 2 * 1024 ** 3, expires: now + 14 * DAY,
      message: 'Hallo ihr Lieben,\nhier sind die Fotos vom Urlaub am Meer. Ladet gern eure eigenen dazu!',
      root: folder('Urlaub 2026', [
        folder('Abends', [file('Lagerfeuer.jpg', picture('Lagerfeuer', '#ff832b', '#4b1d3f'), 6), file('Sternenhimmel.jpg', picture('Sternenhimmel', '#1d2b64', '#0b0f2b'), 6)], 6),
        file('Strand.jpg', picture('Strand', '#1d99f3', '#f6d365'), 7),
        file('Berge.jpg', picture('Berge', '#6fdc8c', '#1d4d3a'), 5),
        file('Reiseplan.md', strToU8('# Reiseplan\n\n- Montag: Anreise und Strand\n- Mittwoch: Wanderung in die Berge\n- Freitag: Lagerfeuer am Abend\n'), 9),
      ]),
    },
    'm7tq2p-rechnung': {
      id: 'rechnung', kind: 'file', root: file('Rechnung-Oktober.pdf', pdf(['Rechnung Oktober 2026', 'Webhosting fuer den Verein: 12,00 EUR', 'Danke fuer die Ueberweisung!']), 2),
    },
    abgelaufen: { gone: 'share-expired' },
    aufgebraucht: { gone: 'share-used-up' },
  };
}

function account(name, twofa) {
  const full = { jdp: 'Junker der Provinz', anna: 'Anna', neu: 'Neu' }[name] ?? name;
  return {
    id: String(name.length), name, full, role: name === 'jdp' ? 'admin' : 'user', scope: '/mnt/user', groups: [], loginMethod: 'password',
    twofa, passkeys: 0, last: iso(clock.now()), pwChanged: iso(clock.now() - 41 * DAY), recovery: twofa ? 8 : 0, lockPassword: false,
  };
}

export function authFixtureMiddleware() {
  const users = { jdp: { totp: true, used: new Set(), recovery: ['K7QM3XRP'] }, anna: { totp: false }, neu: { totp: false, renew: true } };
  const S = { out: process.env.CC_FIXTURE_SIGNED_OUT === '1', fails: 0, steps: new Map(), unlocked: new Set(), uploads: new Map(), links: links() };

  const done = (res, name) => {
    S.out = false;
    S.fails = 0;
    json(res, 200, { next: 'done', me: account(name, users[name].totp) }, { 'Set-Cookie': 'cannoncommander_session=fixture; Path=/; HttpOnly; SameSite=Strict' });
  };
  const step = (res, name, need) => {
    const challenge = Math.random().toString(36).slice(2);
    S.steps.set(challenge, { name, need });
    json(res, 200, { next: need, challenge });
  };
  const after = (res, name) => (users[name].renew ? step(res, name, 'password') : users[name].totp ? step(res, name, 'totp') : done(res, name));

  const auth = {
    'GET /auth/options': (_b, res) => json(res, 200, { password: true, passkey: false, oidc: null, server: 'Bottich', version: '0.1.0', minPassword: 12 }),
    'POST /auth/login': (b, res) => {
      if (S.fails >= 5) return problem(res, 429, 'too many wrong tries, please wait a few minutes', undefined, { 'Retry-After': '900' });
      if (!b.username || !b.password) return problem(res, 400, 'name and password are both needed');
      const name = String(b.username).trim().toLowerCase();
      if (!users[name] || !PASSWORDS.includes(b.password)) {
        S.fails++;
        return problem(res, 401, 'the name or the password is not right');
      }
      after(res, name);
    },
    'POST /auth/totp': (b, res) => {
      const st = S.steps.get(b.challenge);
      if (!st || st.need !== 'totp') return problem(res, 410, 'this sign in ran out, please start again');
      const u = users[st.name], code = String(b.code || '').replace(/[\s-]/g, '').toUpperCase();
      const ok = (code === '123456' && !u.used.has(code)) || u.recovery.includes(code);
      if (!ok) return problem(res, 400, 'this code does not work, take a fresh one from the app or a recovery code');
      u.used.add(code);
      u.recovery = u.recovery.filter((c) => c !== code);
      S.steps.delete(b.challenge);
      done(res, st.name);
    },
    'POST /auth/password': (b, res) => {
      const st = S.steps.get(b.challenge);
      if (!st || st.need !== 'password') return problem(res, 410, 'this sign in ran out, please start again');
      if (String(b.password || '').length < 12 || b.password === PASSWORD) return problem(res, 400, 'the password needs at least 12 characters');
      users[st.name].renew = false;
      S.steps.delete(b.challenge);
      after(res, st.name);
    },
    'POST /auth/passkey/options': (_b, res) => problem(res, 409, 'passkeys are turned off on this server'),
    'POST /auth/passkey': (_b, res) => problem(res, 409, 'passkeys are turned off on this server'),
    // Only the fixture has this: a step file that sets its session back without signing in.
    'PUT /fixture/session': (b, res) => { S.out = !b.signedIn; json(res, 204, null); },
    'POST /auth/logout': (_b, res) => { S.out = true; json(res, 204, null, { 'Set-Cookie': 'cannoncommander_session=; Path=/; Max-Age=0' }); },
    'POST /auth/reset-code': (_b, res) => json(res, 202, null),
    'POST /auth/reset': (b, res) => {
      if (String(b.code || '').replace(/[\s-]/g, '').toUpperCase() !== 'K7QM3XRP9DTA') return problem(res, 400, 'this code is not right or ran out, please ask for a new one');
      if (String(b.password || '').length < 12) return problem(res, 400, 'the password needs at least 12 characters');
      S.out = false;
      json(res, 200, account('jdp', false));
    },
  };

  const unlocked = (l, req) => {
    const url = new URL(req.url, 'http://fixture');
    const token = req.headers['x-share-token'] || url.searchParams.get('token') || (req.headers.cookie || '').match(new RegExp(`cc_share_${l.id}=([^;]+)`))?.[1];
    return !l.password || S.unlocked.has(token);
  };
  // The share link the request is about. A link that leads nowhere gets its answer here and comes back as null.
  function visit(slug, req, res, needUnlock = true) {
    const l = S.links[slug];
    if (!l) problem(res, 404, 'there is no such link');
    else if (l.gone) problem(res, 410, 'this link leads nowhere any more', l.gone);
    else if (needUnlock && !unlocked(l, req)) problem(res, 401, 'this link needs its password');
    else return l;
    return null;
  }
  const nodeAt = (l, path) => {
    if (l.kind === 'file') return path === '/' || path === '/' + l.root.name ? l.root : null;
    let n = l.root;
    for (const part of path.split('/').filter(Boolean)) n = n?.children?.find((k) => k.name === part);
    return n ?? null;
  };
  const item = (k) => ({ name: k.name, dir: !!k.dir, size: k.dir ? 0 : k.data.length, mtime: iso(k.time) });
  const zipOf = (n) => {
    const out = {};
    const walk = (k, at) => (k.dir ? k.children.forEach((c) => walk(c, at + k.name + '/')) : (out[at + k.name] = k.data));
    walk(n, '');
    return zipSync(out);
  };
  const TYPES = { jpg: 'image/svg+xml', md: 'text/markdown; charset=utf-8', pdf: 'application/pdf', txt: 'text/plain; charset=utf-8' };

  async function shares(req, res, slug, rest, url) {
    const path = url.searchParams.get('path') || '/';
    if (req.method === 'GET' && rest === '') {
      const l = visit(slug, req, res, false);
      if (!l) return;
      if (!unlocked(l, req)) return json(res, 200, { slug, locked: true });
      const out = { slug, locked: false, kind: l.kind, name: l.root.name, ...(l.message ? { message: l.message } : {}), ...(l.expires ? { expires: iso(l.expires) } : {}) };
      if (l.uploads) Object.assign(out, { uploads: true, uploadRoom: l.room });
      if (l.kind === 'file') Object.assign(out, { size: l.root.data.length, mtime: iso(l.root.time) });
      return json(res, 200, out);
    }
    if (req.method === 'POST' && rest === '/unlock') {
      const l = visit(slug, req, res, false);
      if (!l) return;
      const b = JSON.parse((await readBody(req)).toString() || '{}');
      if (b.password !== l.password) return problem(res, 403, 'the password is not right');
      const token = 'fx-' + Math.random().toString(36).slice(2);
      S.unlocked.add(token);
      return json(res, 200, { token, expires: iso(clock.now() + DAY / 2) }, { 'Set-Cookie': `cc_share_${l.id}=${token}; Path=/; HttpOnly; SameSite=Lax` });
    }
    const l = visit(slug, req, res);
    if (!l) return;
    if (req.method === 'GET' && rest === '/list') {
      const n = nodeAt(l, path);
      if (!n) return problem(res, 404, 'there is nothing at this place in the link');
      if (!n.dir) return json(res, 200, { path: '/', name: n.name, items: [item(n)] });
      return json(res, 200, { path, name: n.name, items: n.children.map(item).sort((a, b) => b.dir - a.dir || a.name.localeCompare(b.name, 'de')) });
    }
    if (req.method === 'GET' && rest === '/raw') {
      const n = nodeAt(l, path);
      if (!n) return problem(res, 404, 'there is nothing at this place in the link');
      const download = url.searchParams.get('download');
      if (n.dir) {
        res.setHeader('Content-Type', 'application/zip');
        res.setHeader('Content-Disposition', `attachment; filename="${encodeURIComponent(n.name)}.zip"`);
        return res.end(Buffer.from(zipOf(n)));
      }
      res.setHeader('Content-Type', TYPES[n.name.split('.').pop().toLowerCase()] || 'application/octet-stream');
      res.setHeader('Content-Disposition', `${download ? 'attachment' : 'inline'}; filename="${encodeURIComponent(n.name)}"`);
      return res.end(Buffer.from(n.data));
    }
    if (req.method === 'POST' && rest === '/upload') {
      if (!l.uploads) return problem(res, 403, 'this link takes no uploads');
      const b = JSON.parse((await readBody(req)).toString() || '{}');
      if (b.size > l.room) return problem(res, 413, 'the file does not fit into the upload limit of the link');
      const id = 'up-' + Math.random().toString(36).slice(2);
      const up = { id, path: (b.path === '/' ? '' : b.path) + '/' + b.name, size: b.size, offset: 0, state: 'open', rule: 'both', chunk: 256 * 1024, parts: [], dir: b.path || '/', name: b.name };
      S.uploads.set(id, up);
      if (!b.size) return json(res, 200, land(l, up));
      return json(res, 201, wire(up));
    }
    const m = rest.match(/^\/upload\/([^/]+)$/);
    const up = m && S.uploads.get(decodeURIComponent(m[1]));
    if (m && !up) return problem(res, 404, 'there is no such upload');
    if (up && req.method === 'GET') return json(res, 200, wire(up));
    if (up && req.method === 'DELETE') { S.uploads.delete(up.id); return json(res, 204, null); }
    if (up && req.method === 'PUT') {
      if (Number(req.headers['upload-offset']) !== up.offset) return problem(res, 409, 'the upload is at another offset');
      const chunk = await readBody(req);
      up.parts.push(chunk);
      up.offset += chunk.length;
      return json(res, 200, up.offset >= up.size ? land(l, up) : wire(up));
    }
    problem(res, 501, 'not in the fixture');
  }
  const wire = (up) => ({ id: up.id, path: up.path, size: up.size, offset: up.offset, state: up.state, rule: up.rule, chunk: up.chunk });
  // A finished upload becomes a file in the folder, with a number when the name is taken.
  function land(l, up) {
    const dir = nodeAt(l, up.dir);
    let name = up.name.split('/').pop();
    const [base, ext] = name.includes('.') ? [name.slice(0, name.lastIndexOf('.')), name.slice(name.lastIndexOf('.'))] : [name, ''];
    for (let i = 2; dir.children.some((k) => k.name === name); i++) name = `${base} (${i})${ext}`;
    dir.children.push({ name, data: new Uint8Array(Buffer.concat(up.parts)), time: clock.now() });
    l.room -= up.size;
    up.state = 'done';
    S.uploads.delete(up.id);
    return wire(up);
  }

  return async (req, res, next) => {
    const url = new URL(req.url, 'http://fixture');
    if (!url.pathname.startsWith('/api/v1/')) return next();
    const path = url.pathname.slice('/api/v1'.length);
    const share = path.match(/^\/s\/([^/]+)(\/.*)?$/);
    if (share) return shares(req, res, decodeURIComponent(share[1]), share[2] || '', url);
    const route = auth[`${req.method} ${path}`];
    if (route) {
      const raw = req.method === 'GET' ? '' : (await readBody(req)).toString();
      return route(raw ? JSON.parse(raw) : {}, res);
    }
    if (S.out) return problem(res, 401, 'login required');
    next();
  };
}
