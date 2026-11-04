// Serves the prototype's made-up Unraid tree in the shape of /api/v1, so the interface can be built before the server
// answers. Only `vite --mode fixture` loads it; a real build never ships it.
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import vm from 'node:vm';
import { contentRoutes } from './fixture-content.js';

const prototype = join(import.meta.dirname, '..', '..', 'prototype', 'src');

// 10-data.js declares its tree with top-level consts, so the script ends with an expression that hands them back.
function loadPrototype() {
  const KB = 1024, MB = KB * 1024, GB = MB * 1024, TB = GB * 1024;
  let seed = 7;
  const context = vm.createContext({
    FORMATS: JSON.parse(readFileSync(join(prototype, 'formats.json'), 'utf8')),
    NOW: Date.UTC(2026, 10, 2, 9, 30), DAY: 86400000, KB, MB, GB, TB,
    rnd: () => (seed = (seed * 16807) % 2147483647) / 2147483647,
    CC: { prefs: {} },
  });
  const source = readFileSync(join(prototype, 'js', '10-data.js'), 'utf8');
  return vm.runInContext(`${source}\n;({ FS, DATA })`, context);
}

const iso = (t) => new Date(t).toISOString();

export function createFixture() {
  const { FS, DATA } = loadPrototype();
  const volume = (path) => {
    const v = FS.volumeOf(path);
    return FS.TOTAL[v] ? { path: v, label: FS.LABEL[v], free: Math.round(FS.FREE[v]), total: Math.round(FS.TOTAL[v]) } : undefined;
  };
  const item = (k) => ({
    name: k.name, dir: !!k.dir, size: Math.round(k.dir ? FS.totalSize(k) : k.size || 0), mtime: iso(k.time), mode: k.mode,
    owner: k.owner, group: k.group, ...(k.hidden ? { hidden: true } : {}), ...(k.link ? { link: k.link } : {}),
    ...(k.dir ? { items: FS.kids(k).length } : {}),
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
  return {
    FS,
    list(path) {
      const n = dirAt(path);
      return n && { path, items: FS.kids(n).map(item), volume: volume(path) };
    },
    tree(path) {
      const n = dirAt(path);
      return n && { path, items: FS.kids(n).filter((k) => k.dir).map((k) => folder(path, k)) };
    },
    me: () => ({ name: 'jdp', full: 'Junker der Provinz', role: 'Admin' }),
    bookmarks: () => ({ items: DATA.bookmarks }),
    trash: () => ({ items: DATA.trash.map((t) => ({ ...t, size: Math.round(t.size), deleted: iso(t.deleted) })) }),
  };
}

const problem = (res, status, title) => {
  res.statusCode = status;
  res.setHeader('Content-Type', 'application/problem+json');
  res.end(JSON.stringify({ type: 'about:blank', title, status }));
};

// Connect middleware for Vite's dev and preview servers.
export function fixtureMiddleware() {
  const fx = createFixture();
  const content = contentRoutes(fx.FS);
  const routes = {
    '/api/v1/fs/list': (q) => content.list(q.get('path') || '/') || fx.list(q.get('path') || '/'),
    '/api/v1/fs/tree': (q) => fx.tree(q.get('path') || '/'),
    '/api/v1/me': () => fx.me(),
    '/api/v1/me/bookmarks': () => fx.bookmarks(),
    // Settings are taken and forgotten; the browser's own copy carries them in this mode.
    '/api/v1/me/prefs': () => ({}),
    '/api/v1/trash': () => fx.trash(),
  };
  return (req, res, next) => {
    const url = new URL(req.url, 'http://fixture');
    const route = routes[url.pathname];
    if (!url.pathname.startsWith('/api/')) return next();
    if (content.handle(url, req, res)) return;
    if (!route) return problem(res, 501, 'Not built yet');
    const body = route(url.searchParams);
    if (!body) return problem(res, 404, 'Not found');
    res.setHeader('Content-Type', 'application/json');
    res.end(JSON.stringify(body));
  };
}
