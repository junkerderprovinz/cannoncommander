// Serves the prototype's made-up Unraid tree in the shape of /api/v1, so the interface can be built before the server
// answers. Only `vite --mode fixture` loads it; a real build never ships it.
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import vm from 'node:vm';
import { contentRoutes } from './fixture-content.js';

const prototype = join(import.meta.dirname, '..', '..', 'prototype', 'src');
const read = (...p) => readFileSync(join(prototype, ...p), 'utf8');
const exported = join(import.meta.dirname, '..', '..', 'testing', 'fixture', 'prototype.json');

// The tree is the prototype's after all its features ran, as testing/export.mjs wrote it, with each file's column
// values and text. The prototype's own source still lends its helpers, the sort figures and the made-up pictures.
// Its files declare top-level consts, so each run ends with an expression that hands them back.
export function loadPrototype() {
  const state = JSON.parse(readFileSync(exported, 'utf8'));
  const KB = 1024, MB = KB * 1024, GB = MB * 1024, TB = GB * 1024, DAY = 86400000, NOW = state.now;
  let seed = 7;
  const fmtSize = (b) => {
    if (b == null) return '';
    for (const [n, v] of [['TB', TB], ['GB', GB], ['MB', MB], ['KB', KB]]) {
      if (b >= v) return (b / v).toLocaleString('de-AT', { maximumFractionDigits: b / v < 10 ? 1 : 0 }) + ' ' + n;
    }
    return b + ' B';
  };
  const CC = { prefs: {}, action() {}, css() {}, bindKey() {} };
  const context = vm.createContext({
    FORMATS: JSON.parse(read('formats.json')), NOW, DAY, KB, MB, GB, TB, CC, fmtSize,
    fmtListDate: (t) => new Date(t).toLocaleDateString('de-AT', { day: '2-digit', month: '2-digit', year: 'numeric' }),
    rnd: () => (seed = (seed * 16807) % 2147483647) / 2147483647,
    esc: (s) => String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c])),
    g: () => '', menu() {}, win() {}, winStack: [], comboOf: () => '',
  });
  const { FS, DATA, ROOT } = vm.runInContext(`${read('js', '10-data.js')}\n;({ FS, DATA, ROOT })`, context);
  ROOT.children = state.tree.children;
  for (const key of ['FREE', 'TOTAL', 'LABEL']) {
    for (const k of Object.keys(FS[key])) delete FS[key][k];
    Object.assign(FS[key], state.volumes[key.toLowerCase()]);
  }
  Object.assign(DATA, { bookmarks: state.data.bookmarks, trash: state.data.trash, labels: state.data.labels || {} });
  const { COL } = vm.runInContext(`${read('js', '25-columns.js')}\n;({ COL })`, context);
  vm.runInContext(read('js', 'features', 'c-00-kit.js'), context);
  return { FS, DATA, COL, ck: CC.ck, hashOf: vm.runInContext('hashOf', context), DAY, NOW, CC, run: (code) => vm.runInContext(code, context) };
}

const iso = (t) => new Date(t).toISOString();

// The field tests of the name line, such as breite:>=3840 or geändert:<30d, as the prototype's e-30-meta.js reads
// them; the part of that file that patches the panel's filter is left out.
function metaParser(run) {
  const src = readFileSync(join(prototype, 'js', 'features', 'e-30-meta.js'), 'utf8');
  return run(src.slice(0, src.indexOf('  const plain = nameMatcher;')) + '  return parse;\n})()');
}

const UNIT = { B: 1, K: 1024, KB: 1024, M: 1024 ** 2, MB: 1024 ** 2, G: 1024 ** 3, GB: 1024 ** 3, T: 1024 ** 4, TB: 1024 ** 4 };
// A number alone counts in MB, as the search window says next to its size fields.
const bytes = (v) => { const m = String(v ?? '').trim().toUpperCase().match(/^([\d.,]+)\s*([KMGT]?B?)$/); return m ? parseFloat(m[1].replace(',', '.')) * (UNIT[m[2]] || UNIT.MB) : NaN; };
// A word without * or ? matches anywhere in a name, a word with them the whole name, as b-00-kit.js does it.
function globRe(words) {
  if (!words.length) return null;
  const one = (p) => { const body = p.replace(/[.+^${}()|[\]\\]/g, '\\$&').replace(/\*/g, '.*').replace(/\?/g, '.'); return /[*?]/.test(p) ? `^${body}$` : body; };
  return new RegExp(words.map(one).join('|'), 'i');
}

// fs/meta sends figures and yes or no as such; the prototype writes everything as text, so these turn back.
const NUMERIC = ['accessed', 'statchange', 'taken', 'duration', 'season', 'episode', 'track', 'year', 'iso', 'pages', 'words', 'lines', 'entries', 'unpacked', 'vbitrate'];
const BOOLEAN = ['encrypted', 'archcrypt', 'bootable', 'signed'];

export function createFixture() {
  const { FS, DATA, COL, ck, hashOf, DAY, NOW, run } = loadPrototype();
  const parse = metaParser(run);
  const h = (k, salt) => hashOf(k.name + salt);
  const volume = (path) => {
    const v = FS.volumeOf(path);
    return FS.TOTAL[v] && path !== '/' && path !== '/mnt' ? { path: v, label: FS.LABEL[v], free: Math.round(FS.FREE[v]), total: Math.round(FS.TOTAL[v]) } : undefined;
  };
  // The prototype makes up inode, links and the sticky bit from the name, and its features settle some of them; the
  // fixture hands the values over as a server would read them.
  const num = (k, id, made) => (k.cols?.[id] ? +k.cols[id] : made);
  const item = (k) => ({
    name: k.name, dir: !!k.dir, size: Math.round(k.dir ? FS.totalSize(k) : k.size || 0), mtime: iso(k.time),
    mode: k.dir && h(k, 's') % 7 === 0 ? '1' + k.mode.slice(-3) : k.mode,
    owner: k.owner, group: k.group, ...(k.hidden ? { hidden: true } : {}), ...(k.link ? { link: k.link } : {}),
    ...(k.dir ? { items: FS.kids(k).length } : {}), ...(k.zip ? { archive: true } : {}),
    btime: iso(k.time - (hashOf(k.name) % 400) * DAY), ino: num(k, 'inode', h(k, 'i') % 9000000), nlink: num(k, 'hardlinks', k.dir ? 2 : 1 + (h(k, 'l') % 9 === 0)),
    uid: num(k, 'uid', k.owner === 'root' ? 0 : 99), gid: num(k, 'gid', k.group === 'root' ? 0 : 100),
  });
  const folder = (path, k) => {
    const p = FS.join(path, k.name);
    return {
      name: k.name, path: p, hasDirs: FS.kids(k).some((c) => c.dir),
      ...(FS.LABEL[p] ? { label: FS.LABEL[p] } : {}), ...(FS.TOTAL[p] ? { free: Math.round(FS.FREE[p]), total: Math.round(FS.TOTAL[p]) } : {}),
      ...(k.remote ? { remote: k.remote } : {}),
    };
  };
  const dirAt = (path) => {
    const n = FS.nodeAt(path);
    return n && n.dir ? n : null;
  };
  const fileAt = (path) => {
    const n = FS.nodeAt(path);
    return n && !n.dir ? { ...n, kind: FS.kindOf(n) } : null;
  };
  const metaOf = (path, columns) => {
    const k = FS.nodeAt(path);
    if (!k) return null;
    const node = { ...k, kind: FS.kindOf(k) }, at = FS.parent(path), out = {};
    for (const id of columns) {
      const c = COL[id];
      if (!c?.value) continue;
      const text = k.cols?.[id] ?? c.value(node, at);
      out[id] = !text ? null : BOOLEAN.includes(id) ? text === 'Ja' : NUMERIC.includes(id) ? (c.num ? c.num(node) : Number(text)) : text;
    }
    return out;
  };
  return {
    FS,
    DATA,
    hashOf,
    list(path) {
      const n = dirAt(path);
      return n && { path, items: FS.kids(n).map(item), volume: volume(path) };
    },
    tree(path) {
      const n = dirAt(path);
      return n && { path, items: FS.kids(n).filter((k) => k.dir).map((k) => folder(path, k)) };
    },
    meta({ paths = [], columns = [] }) {
      return { items: Object.fromEntries(paths.map((p) => [p, metaOf(p, columns)]).filter(([, v]) => v)) };
    },
    // GET /search the way the prototype's search window finds: names, field tests, tags, kind, size and age below a
    // folder, hidden entries and everything in hidden folders only when asked, folders first and then by name.
    search(path, p = {}) {
      const root = dirAt(path);
      if (!root) return null;
      const mq = parse(p.q || '');
      const words = mq.rest.split(/[\s;]+/).filter(Boolean);
      const tags = words.filter((w) => w.startsWith('#')).map((w) => w.slice(1).toLowerCase());
      const nameRe = globRe(words.filter((w) => !w.startsWith('#')));
      const tagged = (k, at) => { const l = DATA.labels[FS.join(at, k.name)]; return !!l && tags.some((w) => l.color === w || (l.tags || []).some((x) => x.toLowerCase() === w)); };
      const min = bytes(p.min), max = bytes(p.max), when = p.when || 'any', type = p.type || 'all', hidden = !!p.hidden;
      const limit = Math.min(10000, Math.max(1, Number(p.limit) || 2000));
      const typeOk = (k) => {
        if (type === 'all') return true;
        const kind = FS.kindOf(k);
        if (type === 'doc') return ['pdf', 'office', 'text', 'code'].includes(kind);
        if (type === 'arch') return kind === 'arch' || kind === 'disk' || !!k.zip;
        return kind === type;
      };
      const hits = [];
      let scanned = 0;
      const walk = (n, at) => {
        for (const k of FS.kids(n)) {
          if (!hidden && (k.hidden || k.name.startsWith('.'))) continue;
          scanned++;
          const size = FS.totalSize(k);
          let ok = (!nameRe && !tags.length) || !!nameRe?.test(k.name) || tagged(k, at);
          ok &&= typeOk(k) && (!mq.fields || mq.test({ ...k, at }));
          if (ok && !isNaN(min)) ok = size >= min;
          if (ok && !isNaN(max)) ok = size <= max;
          if (ok && when !== 'any') ok = when === 'old' ? k.time < NOW - 365 * DAY : k.time >= NOW - Number(when) * DAY;
          if (ok) hits.push({ k, at });
          if (k.dir && !k.link) walk(k, FS.join(at, k.name));
        }
      };
      walk(root, path);
      if (mq.unknown.length) hits.length = 0;
      hits.sort((a, b) => Number(!!b.k.dir) - Number(!!a.k.dir) || a.k.name.localeCompare(b.k.name, 'de', { numeric: true }));
      return { path, items: hits.slice(0, limit).map((h) => ({ ...item(h.k), at: h.at })), total: hits.length, more: hits.length > limit, scanned, pending: 0, unknown: mq.unknown, indexed: true };
    },
    me: () => ({ name: 'jdp', full: 'Junker der Provinz', role: 'Admin' }),
    bookmarks: () => ({ items: DATA.bookmarks }),
  };
}

const problem = (res, status, title) => {
  res.statusCode = status;
  res.setHeader('Content-Type', 'application/problem+json');
  res.end(JSON.stringify({ type: 'about:blank', title, status }));
};

const bodyOf = (req) => new Promise((resolve) => {
  let raw = '';
  req.on('data', (c) => { raw += c; });
  req.on('end', () => { try { resolve(JSON.parse(raw || '{}')); } catch { resolve({}); } });
});

// One tree for every middleware, so what one of them changes the others serve.
let shared;
export const sharedFixture = () => (shared ??= createFixture());

// Connect middleware for Vite's dev and preview servers.
export function fixtureMiddleware() {
  const fx = sharedFixture();
  const content = contentRoutes(fx.FS);
  const routes = {
    'GET /api/v1/fs/list': (q) => content.list(q.get('path') || '/') || fx.list(q.get('path') || '/'),
    'GET /api/v1/fs/tree': (q) => fx.tree(q.get('path') || '/'),
    'POST /api/v1/fs/meta': (_q, body) => fx.meta(body),
    'GET /api/v1/search': (q) => fx.search(q.get('path') || '/', Object.fromEntries(q)),
    'GET /api/v1/me': () => fx.me(),
    'GET /api/v1/me/bookmarks': () => fx.bookmarks(),
    // Settings, shortcuts and watched folders are taken and forgotten; the browser's own copy carries them here.
    'PUT /api/v1/me/bookmarks': (_q, body) => body,
    'GET /api/v1/me/prefs': () => ({}),
    'PUT /api/v1/me/prefs': () => ({}),
    'PUT /api/v1/events/watch': (_q, body) => body,
  };
  return async (req, res, next) => {
    const url = new URL(req.url, 'http://fixture');
    if (!url.pathname.startsWith('/api/')) return next();
    if (content.handle(url, req, res)) return;
    const route = routes[`${req.method} ${url.pathname}`];
    if (!route) return problem(res, 501, 'Not built yet');
    const body = route(url.searchParams, req.method === 'GET' ? undefined : await bodyOf(req));
    if (!body) return problem(res, 404, 'Not found');
    res.setHeader('Content-Type', 'application/json');
    res.end(JSON.stringify(body));
  };
}
