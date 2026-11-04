// Saved searches, comparing and syncing folders, comparing files, duplicates, disk usage and the hand over to
// ArrowLoop, played on the prototype's tree in the shape of backend/internal/{prefs,search,tools}. The long runs are
// jobs on the queue of fixture-ops.js, so they show in the jobs window and end with job.done like the server's.
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import vm from 'node:vm';
import { sharedOps } from './fixture-ops.js';
import { sharedFixture } from './fixture.js';

const KB = 1024, MB = KB * 1024, GB = MB * 1024, DAY = 86400000;
const kit = readFileSync(join(import.meta.dirname, '..', '..', 'prototype', 'src', 'js', 'features', 'b-00-kit.js'), 'utf8');
const iso = (t) => new Date(t).toISOString();
const base = (p) => p.split('/').filter(Boolean).pop() || '';

// The prototype's made-up contents: the text of a file it only knows by name, a hex dump of a binary one, and a second
// version of a text with a few believable edits. Only the functions are taken; the files b-00-kit.js adds to the tree
// are in the exported tree already.
function madeUp(FS, NOW) {
  const from = kit.indexOf('  const strHash'), to = kit.indexOf('  CC.bk = {');
  const context = vm.createContext({ FS, NOW, KB, MB, GB, DAY, CC: { prefs: {} }, Files: {}, esc: String, g: () => '', win: () => ({}), document: { addEventListener() {} } });
  return vm.runInContext(`(() => {\n${kit.slice(from, to)}\nreturn { textOf, variant, isBinary, hex };\n})()`, context);
}

// Plain LCS over lines; a run of removals next to a run of additions reads as changed lines, paired row by row.
export function diffLines(a, b) {
  const n = a.length, m = b.length, t = Array.from({ length: n + 1 }, () => new Uint32Array(m + 1));
  for (let i = n - 1; i >= 0; i--) for (let j = m - 1; j >= 0; j--) t[i][j] = a[i] === b[j] ? t[i + 1][j + 1] + 1 : Math.max(t[i + 1][j], t[i][j + 1]);
  const ops = [];
  let i = 0, j = 0;
  while (i < n || j < m) {
    if (i < n && j < m && a[i] === b[j]) ops.push(['eq', i++, j++]);
    else if (j < m && (i === n || t[i][j + 1] >= t[i + 1][j])) ops.push(['add', null, j++]);
    else ops.push(['del', i++, null]);
  }
  const rows = [];
  for (let k = 0; k < ops.length;) {
    if (ops[k][0] === 'eq') { rows.push({ t: 'eq', l: ops[k][1], r: ops[k][2] }); k++; continue; }
    const dels = [], adds = [];
    while (k < ops.length && ops[k][0] !== 'eq') { if (ops[k][0] === 'del') dels.push(ops[k][1]); else adds.push(ops[k][2]); k++; }
    for (let x = 0; x < Math.max(dels.length, adds.length); x++) {
      const l = dels[x] ?? null, r = adds[x] ?? null;
      rows.push({ t: l != null && r != null ? 'chg' : l != null ? 'del' : 'add', l, r });
    }
  }
  return rows;
}

const KIND_OF = { pdf: 'doc', office: 'doc' };
const KIND_NAME = { img: 'Bild', video: 'Video', audio: 'Musik', arch: 'Archiv', disk: 'Abbild', doc: 'Dokument', text: 'Text', code: 'Quelltext', file: 'Sonstiges' };

export function toolsFixture(fx = sharedFixture(), ops = sharedOps()) {
  const { FS } = fx;
  const NOW = JSON.parse(readFileSync(join(import.meta.dirname, '..', '..', 'testing', 'fixture', 'prototype.json'), 'utf8')).now;
  const mk = madeUp(FS, NOW);
  const dirAt = (path) => { const n = FS.nodeAt(path || ''); return n && n.dir ? n : null; };
  const fileAt = (path) => { const n = FS.nodeAt(path || ''); return n && !n.dir ? n : null; };
  const hidden = (k) => k.hidden || k.name.startsWith('.');
  const show = (all) => (k) => all || !hidden(k);
  const clone = (n) => JSON.parse(JSON.stringify(n));

  // Saved searches, the prototype's two to begin with.
  const blank = { text: '', in: '/mnt/user', hidden: false, min: '', max: '', when: 'any', type: 'all' };
  let lastId = 2;
  const saved = [
    { id: 1, name: 'Große Abbilder', query: { ...blank, name: '*.iso *.img', min: '1 GB' } },
    { id: 2, name: 'Diesen Monat geändert', query: { ...blank, name: '', when: '30' } },
  ];
  const savedInput = (b) => (b && typeof b.name === 'string' && b.name.trim() && b.query && typeof b.query === 'object' && (b.query.in == null || dirAt(b.query.in)) ? null : 'a name and a query with a folder in');

  // Two copies count as the same when size and time agree, or by content when the sizes do.
  function state(l, r, content) {
    if (!r) return 'onlyL';
    if (!l) return 'onlyR';
    if (l.dir || r.dir) return l.dir && r.dir && FS.totalSize(l) === FS.totalSize(r) ? 'same' : 'differ';
    const sameSize = l.size === r.size;
    if (content ? sameSize : sameSize && Math.abs(l.time - r.time) < 2000) return 'same';
    if (l.time > r.time + 2000) return 'newerL';
    if (r.time > l.time + 2000) return 'newerR';
    return 'differ';
  }
  const side = (n) => n && { dir: !!n.dir, size: Math.round(FS.totalSize(n)), mtime: iso(n.time) };
  function compare(L, R, { content = false, recursive = false, top = false, hidden: all = false } = {}) {
    const out = [];
    const walk = (ln, rn, rel) => {
      const names = [...new Set([...FS.kids(ln).filter(show(all)), ...FS.kids(rn).filter(show(all))].map((k) => k.name))].sort((a, b) => a.localeCompare(b, 'de', { numeric: true }));
      for (const name of names) {
        const l = FS.kids(ln).find((k) => k.name === name), r = FS.kids(rn).find((k) => k.name === name);
        const p = rel ? rel + '/' + name : name;
        if (!top && l?.dir && r?.dir) { if (recursive) walk(l, r, p); continue; }
        out.push({ rel: p, name, l, r, state: state(l, r, content) });
      }
    };
    walk(dirAt(L), dirAt(R), '');
    return out;
  }
  const compared = (it) => ({ rel: it.rel, name: it.name, state: it.state, ...(it.l ? { left: side(it.l) } : {}), ...(it.r ? { right: side(it.r) } : {}) });

  // A step that would overwrite something newer starts switched off and says why.
  function plan(items, dir, mirror) {
    const steps = [];
    const add = (it, act, to, off = false, why) => {
      const n = act === 'delete' ? (to === 'R' ? it.r : it.l) : (to === 'R' ? it.l : it.r);
      steps.push({ key: `${it.rel}|${act}|${to}`, rel: it.rel, act, to, dir: !!n.dir, ...(off ? { off } : {}), ...(why ? { why } : {}), size: Math.round(FS.totalSize(n)) });
    };
    for (const it of items) {
      const s = it.state;
      if (s === 'same') continue;
      if (dir === 'right') {
        if (s === 'onlyL') add(it, 'copy', 'R');
        else if (s === 'onlyR') { if (mirror) add(it, 'delete', 'R'); }
        else add(it, 'replace', 'R', s !== 'newerL', s === 'newerR' ? 'rightNewer' : s === 'differ' ? 'sameAge' : undefined);
      } else if (dir === 'left') {
        if (s === 'onlyR') add(it, 'copy', 'L');
        else if (s === 'onlyL') { if (mirror) add(it, 'delete', 'L'); }
        else add(it, 'replace', 'L', s !== 'newerR', s === 'newerL' ? 'leftNewer' : s === 'differ' ? 'sameAge' : undefined);
      } else if (s === 'onlyL') add(it, 'copy', 'R');
      else if (s === 'onlyR') add(it, 'copy', 'L');
      else if (s === 'newerL') add(it, 'replace', 'R');
      else if (s === 'newerR') add(it, 'replace', 'L');
      else add(it, 'replace', 'R', true, 'sameAge');
    }
    return steps;
  }

  ops.kinds.sync = {
    prepare({ left, right, steps = [] }) {
      if (!dirAt(left) || !dirAt(right)) return null;
      const nodes = steps.map((s) => FS.nodeAt(FS.join(s.to === 'R' ? (s.act === 'delete' ? right : left) : (s.act === 'delete' ? left : right), s.rel))).filter(Boolean);
      return {
        name: base(left), dest: right, count: steps.length, files: steps.length,
        total: steps.reduce((a, s, i) => a + (s.act === 'delete' || !nodes[i] ? 0 : FS.totalSize(nodes[i])), 0),
        apply() {
          for (const s of steps) {
            const into = s.to === 'R' ? right : left, from = s.to === 'R' ? left : right;
            const relDir = s.rel.split('/').slice(0, -1).join('/');
            const parent = relDir ? FS.join(into, relDir) : into;
            if (s.act === 'delete') {
              const victim = ops.detach(FS.join(into, s.rel));
              if (victim) ops.toTrash(victim, FS.join(into, s.rel));
            } else {
              const src = FS.nodeAt(FS.join(from, s.rel));
              if (!src) continue;
              const dir = ops.ensureDir(parent);
              dir.children = dir.children.filter((c) => c.name !== src.name);
              dir.children.push(clone(src));
            }
            ops.changed(parent);
          }
        },
      };
    },
  };

  // Files grouped by name and size, or by size alone; the copy with the newest time comes first, on equal times the
  // one with the shorter path.
  function duplicates(path, by, all) {
    const groups = new Map();
    let files = 0;
    const walk = (n, at) => {
      for (const k of FS.kids(n).filter(show(all))) {
        if (k.dir) { if (!k.link) walk(k, FS.join(at, k.name)); continue; }
        if (k.link || !k.size) continue;
        files++;
        const key = by === 'size' ? `${k.size}` : `${k.name}|${k.size}`;
        if (!groups.has(key)) groups.set(key, []);
        groups.get(key).push({ k, at });
      }
    };
    walk(dirAt(path), path);
    const list = [...groups.values()].filter((g) => g.length > 1).map((copies) => {
      copies.sort((a, b) => b.k.time - a.k.time || a.at.length - b.at.length);
      const size = Math.round(copies[0].k.size);
      return { name: copies[0].k.name, size, sha256: mk.hex(`sha256|${copies[0].k.name}|${copies[0].k.size}`, 64), wasted: size * (copies.length - 1),
        copies: copies.map((c) => ({ path: FS.join(c.at, c.k.name), at: c.at, mtime: iso(c.k.time) })) };
    }).sort((a, b) => b.wasted - a.wasted);
    return { groups: list.slice(0, 5000), ...(list.length > 5000 ? { more: true } : {}), wasted: list.reduce((a, g) => a + g.wasted, 0), files };
  }
  const countFiles = (n, all) => FS.kids(n).filter(show(all)).reduce((a, k) => a + (k.dir ? (k.link ? 0 : countFiles(k, all)) : 1), 0);
  ops.kinds.duplicates = {
    prepare({ path, by = 'name', hidden: all = false }) {
      const n = dirAt(path);
      if (!n) return null;
      return { name: base(path), dest: path, count: 1, files: Math.max(1, countFiles(n, all)), total: 0, apply(j) { j.result = duplicates(path, by, all); } };
    },
  };

  // A text inside files: every hit the search window's other criteria find, read for the text, upper and lower case
  // alike, binary files passed over.
  ops.kinds.contentSearch = {
    prepare(p) {
      if (!String(p.text || '').trim()) return 'text is required';
      const found = fx.search(p.path, { ...p, limit: 10000 });
      if (!found) return null;
      if (found.unknown.length) return 'these fields are unknown: ' + found.unknown.join(', ');
      return {
        name: `„${p.text}“`, dest: p.path, count: 1, files: Math.max(1, found.items.length), total: 0,
        apply(j) {
          const want = String(p.text).toLowerCase(), items = [];
          for (const h of found.items) {
            const k = FS.nodeAt(FS.join(h.at, h.name));
            if (!k || k.dir || mk.isBinary(k)) continue;
            const lines = mk.textOf(h.at, k).split('\n'), at = lines.findIndex((l) => l.toLowerCase().includes(want));
            if (at >= 0) items.push({ ...h, line: at + 1 });
          }
          ops.send('search.hits', { job: j.id, items, total: items.length, scanned: found.scanned, done: true });
          j.result = { items: items.slice(0, 2000), total: items.length, scanned: found.scanned };
        },
      };
    },
  };

  // Without real contents the second file is the first with a few edits, so the window has something honest to show.
  function diff(left, right) {
    const a = fileAt(left), b = fileAt(right);
    if (!a || !b) return null;
    const binary = mk.isBinary(a) || mk.isBinary(b);
    const lines = (n, path) => mk.textOf(FS.parent(path), n).replace(/\n$/, '').split('\n');
    const ta = lines(a, left);
    let tb = lines(b, right);
    const identical = a.size === b.size && Math.abs(a.time - b.time) < 2000;
    if (!identical && tb.join('\n') === ta.join('\n')) tb = mk.variant(ta, right, binary);
    const rows = diffLines(ta, tb);
    const sideOf = (n, path, l) => ({ path, size: Math.round(n.size || 0), mtime: iso(n.time), lines: l });
    return { left: sideOf(a, left, ta), right: sideOf(b, right, tb), binary, hunks: rows.filter((r, i) => r.t !== 'eq' && (i === 0 || rows[i - 1].t === 'eq')).length, rows };
  }

  // One walk gives every entry its size, its file and folder count and its newest change.
  function usage(path, all, kind, largest = 200) {
    const root = dirAt(path);
    if (!root) return null;
    const memo = new Map();
    const stat = (n) => {
      if (memo.has(n)) return memo.get(n);
      const s = n.dir ? { size: 0, files: 0, dirs: 0, time: n.time || 0 } : { size: n.link ? 0 : n.size || 0, files: 1, dirs: 0, time: n.time || 0 };
      if (n.dir && !n.link) {
        for (const k of FS.kids(n).filter(show(all))) {
          const c = stat(k);
          s.size += c.size; s.files += c.files; s.dirs += c.dirs + (k.dir ? 1 : 0);
          if (c.time > s.time) s.time = c.time;
        }
      }
      memo.set(n, s);
      return s;
    };
    const totals = (s) => ({ size: Math.round(s.size), files: s.files, dirs: s.dirs, mtime: iso(s.time) });
    const files = [];
    const under = (n, at) => { for (const k of FS.kids(n).filter(show(all))) { if (k.dir) { if (!k.link) under(k, FS.join(at, k.name)); } else files.push({ k, at }); } };
    under(root, path);
    const kindOf = (k) => { const id = FS.kindOf(k); return KIND_OF[id] || (KIND_NAME[id] ? id : 'file'); };
    const types = new Map();
    for (const { k } of files) {
      const id = kindOf(k), e = FS.ext(k.name), t = types.get(id) || { kind: id, name: KIND_NAME[id], files: 0, size: 0, by: new Map() };
      t.files++; t.size += k.size || 0;
      if (e) t.by.set(e, (t.by.get(e) || 0) + (k.size || 0));
      types.set(id, t);
    }
    return {
      path, ...totals(stat(root)),
      items: FS.kids(root).filter(show(all)).map((k) => ({ name: k.name, dir: !!k.dir, ...totals(stat(k)) })).sort((a, b) => b.size - a.size),
      types: [...types.values()].sort((a, b) => b.size - a.size).map(({ by, ...t }) => ({ ...t, size: Math.round(t.size), exts: [...by].sort((a, b) => b[1] - a[1]).map(([e]) => e) })),
      largest: files.filter((f) => !kind || kindOf(f.k) === kind).sort((a, b) => (b.k.size || 0) - (a.k.size || 0)).slice(0, largest)
        .map((f) => ({ name: f.k.name, at: f.at, size: Math.round(f.k.size || 0), mtime: iso(f.k.time) })),
      indexed: true,
    };
  }

  const routes = [
    ['GET', /^\/search\/saved$/, () => [200, { items: saved }]],
    ['POST', /^\/search\/saved$/, (b) => {
      const bad = savedInput(b);
      if (bad) return [400, bad];
      if (saved.length >= 200) return [409, 'There are 200 saved searches already'];
      const s = { id: ++lastId, name: b.name.trim(), query: b.query };
      saved.push(s);
      return [201, s];
    }],
    ['PUT', /^\/search\/saved\/(\d+)$/, (b, [, id]) => {
      const s = saved.find((x) => x.id === Number(id));
      if (!s) return [404, 'Not found'];
      const bad = savedInput(b);
      if (bad) return [400, bad];
      Object.assign(s, { name: b.name.trim(), query: b.query });
      return [200, s];
    }],
    ['DELETE', /^\/search\/saved\/(\d+)$/, (_b, [, id]) => {
      const at = saved.findIndex((x) => x.id === Number(id));
      if (at < 0) return [404, 'Not found'];
      saved.splice(at, 1);
      return [204];
    }],
    ['POST', /^\/tools\/compare$/, (b) => {
      if (!dirAt(b.left) || !dirAt(b.right)) return [404, 'Not found'];
      const items = compare(b.left, b.right, b).map(compared);
      const counts = {};
      for (const it of items) counts[it.state] = (counts[it.state] || 0) + 1;
      return [200, { items, counts }];
    }],
    ['POST', /^\/tools\/sync\/plan$/, (b) => {
      if (!['right', 'left', 'both'].includes(b.direction)) return [400, 'direction is right, left or both'];
      if (!dirAt(b.left) || !dirAt(b.right)) return [404, 'Not found'];
      return [200, { steps: plan(compare(b.left, b.right, { content: b.content, recursive: b.recursive, hidden: b.hidden }), b.direction, b.mirror && b.direction !== 'both') }];
    }],
    ['POST', /^\/tools\/diff$/, (b) => { const d = diff(b.left, b.right); return d ? [200, d] : [404, 'Not found']; }],
    ['POST', /^\/tools\/duplicates$/, (b) => ops.startJob('duplicates', b)],
    ['GET', /^\/tools\/diskusage$/, (_b, _m, q) => {
      const u = usage(q.get('path') || '/', q.get('hidden') === 'true', q.get('kind') || '', Number(q.get('largest') ?? 200));
      return u ? [200, u] : [404, 'Not found'];
    }],
    ['POST', /^\/tools\/arrowloop$/, (b) => {
      if (!b.name || !dirAt(b.left) || !dirAt(b.right)) return [400, 'a name and two folders'];
      return [201, { job: { id: 1, name: b.name, source: b.left, target: b.right, direction: b.direction, mirror: !!b.mirror, every: b.every, at: b.at || '03:00' }, url: 'http://arrowloop.local/#/jobs/1' }];
    }],
  ];

  const answer = (res, status, body) => {
    res.statusCode = status;
    if (status === 204) return res.end();
    res.setHeader('Content-Type', status >= 400 ? 'application/problem+json' : 'application/json');
    res.end(JSON.stringify(status >= 400 ? { type: 'about:blank', title: body, status } : body));
  };
  const bodyOf = (req) => new Promise((resolve) => {
    let raw = '';
    req.on('data', (c) => { raw += c; });
    req.on('end', () => { try { resolve(JSON.parse(raw || '{}')); } catch { resolve({}); } });
  });

  return async (req, res, next) => {
    const url = new URL(req.url, 'http://fixture');
    if (!url.pathname.startsWith('/api/v1/')) return next();
    const path = url.pathname.slice('/api/v1'.length);
    for (const [method, re, fn] of routes) {
      const m = req.method === method && path.match(re);
      if (!m) continue;
      const body = method === 'GET' || method === 'DELETE' ? {} : await bodyOf(req);
      const [status, payload] = fn(body, m, url.searchParams);
      return answer(res, status, payload);
    }
    return next();
  };
}

export const toolsFixtureMiddleware = () => toolsFixture();
