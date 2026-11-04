// Saved searches, comparing and syncing folders, comparing files, duplicates, disk usage, batch rename, links and
// rights, archives, checksums, splitting, clean up, loading from an address, the hand over to ArrowLoop, share links
// and rename presets, played on the prototype's tree in the shapes of the contracts in
// backend/internal/{prefs,search,tools,fileops,fsapi,shares}. It is a plugin of fixture-ops.js, so the long runs are
// jobs on its queue that show in the jobs window and end with job.done like the server's.
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import vm from 'node:vm';
import { clock } from './fixture-clock.js';
import { applyMask, rowErrors } from './rename-mask.js';

const KB = 1024, MB = KB * 1024, GB = MB * 1024, DAY = 86400000;
const kit = readFileSync(join(import.meta.dirname, '..', '..', 'prototype', 'src', 'js', 'features', 'b-00-kit.js'), 'utf8');
const NOW = JSON.parse(readFileSync(join(import.meta.dirname, '..', '..', 'testing', 'fixture', 'prototype.json'), 'utf8')).now;

// The prototype's made-up contents and checksums: the text of a file it only knows by name, a hex dump of a binary
// one, a second version of a text with a few believable edits, and hashes on which copies with the same name and size
// agree. Only the functions are taken; the files b-00-kit.js adds to the tree are in the exported tree already.
function madeUp(FS) {
  const from = kit.indexOf('  const strHash'), to = kit.indexOf('  CC.bk = {');
  const context = vm.createContext({ FS, NOW, KB, MB, GB, DAY, CC: { prefs: {} }, Files: {}, esc: String, g: () => '', win: () => ({}), document: { addEventListener() {} } });
  return vm.runInContext(`(() => {\n${kit.slice(from, to)}\nreturn { textOf, variant, isBinary, hex, rng, hashOf };\n})()`, context);
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

// The names sha256sum and shasum --tag write, and how long each hash is.
const ALGOS = { md5: ['MD5', 32], sha1: ['SHA1', 40], sha256: ['SHA256', 64], sha512: ['SHA512', 128], b3: ['BLAKE3', 64] };
const SUMFILE = /\.(md5|sha1|sha256|sha512|b3)$|^(SHA256SUMS|SHA1SUMS|MD5SUMS|SHA512SUMS|B3SUMS)$/i;

// GNU lines as sha256sum writes them and BSD lines as shasum --tag does.
function parseSums(text, listName = '') {
  const ext = listName.split('.').pop().toLowerCase();
  const algoFor = (len) => (ALGOS[ext]?.[1] === len ? ext : Object.keys(ALGOS).find((a) => ALGOS[a][1] === len) || '');
  return String(text).split('\n').map((l) => l.replace(/\r$/, '')).filter((l) => l.trim() && !l.trim().startsWith('#')).map((l) => {
    const bsd = l.trim().match(/^(MD5|SHA1|SHA256|SHA512|BLAKE3)\s*\((.+)\)\s*=\s*([0-9a-fA-F]+)$/);
    if (bsd) return { name: bsd[2], hash: bsd[3].toLowerCase(), algo: Object.keys(ALGOS).find((a) => ALGOS[a][0] === bsd[1]) };
    const gnu = l.match(/^\\?([0-9a-fA-F]{32,128})\s[ *](.+)$/);
    if (gnu) return { name: gnu[2], hash: gnu[1].toLowerCase(), algo: algoFor(gnu[1].length) };
    return { name: '', bad: l };
  });
}

const stripArchive = (name) => name.replace(/\.(tar\.(zst|gz|xz|bz2)|zip|7z|rar|tgz|tar|zst|gz)$/i, '');
const formatOf = (name) => (name.match(/\.(tar\.(?:zst|gz|xz|bz2)|zip|7z|rar|tgz|tar)$/i)?.[1] || '').toLowerCase();
const PACK = ['zip', 'tar.gz', 'tar.zst', '7z', 'rar'];

// The smallest zip there is, so a download from the fixture saves a file that opens.
const EMPTY_ZIP = Buffer.from([0x50, 0x4b, 0x05, 0x06, ...Array(18).fill(0)]);

export function toolsFixture(engine) {
  const { FS, fx, send, changed, startJob, dirAt, taken, freeName, ensureDir, detach, toTrash, pushStep, nextId, iso, base, clone, raw } = engine;
  const mk = madeUp(FS);
  const now = () => clock.now();
  const fileAt = (path) => { const n = FS.nodeAt(path || ''); return n && !n.dir ? n : null; };
  const hidden = (k) => k.hidden || k.name.startsWith('.');
  const show = (all) => (k) => all || !hidden(k);
  const walk = (n, fn) => { fn(n); if (n.dir && !n.link) FS.kids(n).forEach((k) => walk(k, fn)); };
  const fileNode = (name, size, extra = {}) => ({ name, size: Math.round(size), time: now(), owner: 'nobody', group: 'users', mode: '0664', ...extra });
  const dirNode = (name, children = [], extra = {}) => ({ name, dir: true, children, time: now(), owner: 'nobody', group: 'users', mode: '0775', ...extra });
  const sameDir = (paths) => paths.length && paths.every((p) => FS.parent(p) === FS.parent(paths[0]));
  const missingOf = (paths) => paths.find((p) => !FS.nodeAt(p));
  const isArchive = (n) => !!n && !n.dir && (!!n.zip || FS.kindOf(n) === 'arch' || /\.(7z|rar|zip)\.0*1$/i.test(n.name));
  const result = (j, value) => { j.result = value; };

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
    const step = (ln, rn, rel) => {
      const names = [...new Set([...FS.kids(ln).filter(show(all)), ...FS.kids(rn).filter(show(all))].map((k) => k.name))].sort((a, b) => a.localeCompare(b, 'de', { numeric: true }));
      for (const name of names) {
        const l = FS.kids(ln).find((k) => k.name === name), r = FS.kids(rn).find((k) => k.name === name);
        const p = rel ? rel + '/' + name : name;
        if (!top && l?.dir && r?.dir) { if (recursive) step(l, r, p); continue; }
        out.push({ rel: p, name, l, r, state: state(l, r, content) });
      }
    };
    step(dirAt(L), dirAt(R), '');
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

  // Files grouped by name and size, or by size alone; the copy with the newest time comes first, on equal times the
  // one with the shorter path.
  function duplicates(path, by, all) {
    const groups = new Map();
    let files = 0;
    const visit = (n, at) => {
      for (const k of FS.kids(n).filter(show(all))) {
        if (k.dir) { if (!k.link) visit(k, FS.join(at, k.name)); continue; }
        if (k.link || !k.size) continue;
        files++;
        const key = by === 'size' ? `${k.size}` : `${k.name}|${k.size}`;
        if (!groups.has(key)) groups.set(key, []);
        groups.get(key).push({ k, at });
      }
    };
    visit(dirAt(path), path);
    const list = [...groups.values()].filter((g) => g.length > 1).map((copies) => {
      copies.sort((a, b) => b.k.time - a.k.time || a.at.length - b.at.length);
      const size = Math.round(copies[0].k.size);
      return { name: copies[0].k.name, size, sha256: mk.hashOf('sha256', copies[0].k), wasted: size * (copies.length - 1),
        copies: copies.map((c) => ({ path: FS.join(c.at, c.k.name), at: c.at, mtime: iso(c.k.time) })) };
    }).sort((a, b) => b.wasted - a.wasted);
    return { groups: list.slice(0, 5000), ...(list.length > 5000 ? { more: true } : {}), wasted: list.reduce((a, g) => a + g.wasted, 0), files };
  }
  const countFiles = (n, all) => FS.kids(n).filter(show(all)).reduce((a, k) => a + (k.dir ? (k.link ? 0 : countFiles(k, all)) : 1), 0);

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

  // The values the metadata tokens read, from the same columns the list shows.
  const META = { A: 'taken', K: 'camera', I: 'artist', T: 'title', R: 'dimensions', S: 'season', F: 'episode' };
  const metaOf = (path) => {
    let values;
    return (id) => {
      values ??= fx.meta({ paths: [path], columns: Object.values(META) }).items[path] || {};
      const v = values[META[id]];
      if (v == null || v === '') return '';
      if (id === 'A') return new Date(v).toISOString().slice(0, 10);
      if (id === 'R') return String(v).replace(' × ', 'x');
      return String(v);
    };
  };

  function renameBatch(b) {
    const paths = b.paths || [];
    if (!paths.length) return [400, 'paths are required'];
    if (!b.mask && !b.names) return [400, 'a mask or the names are required'];
    if (b.names && b.names.length !== paths.length) return [400, 'there has to be one name for every path'];
    if (missingOf(paths)) return [404, 'Not found'];
    const nodes = paths.map((p) => FS.nodeAt(p));
    let made;
    try {
      made = b.mask
        ? applyMask(b.mask, nodes.map((n, i) => ({ name: n.name, dir: !!n.dir, time: n.time, folder: base(FS.parent(paths[i])), meta: metaOf(paths[i]) })))
        : b.names.map((name) => ({ name: String(name).trim(), missing: [] }));
    } catch {
      return [400, 'find is no valid expression'];
    }
    const moving = new Set(nodes), cache = new Map();
    const takenIn = (folder) => cache.get(folder) || cache.set(folder, new Set(FS.kids(dirAt(folder)).filter((k) => !moving.has(k)).map((k) => k.name.toLowerCase()))).get(folder);
    const rows = paths.map((p, i) => ({ path: p, old: nodes[i].name, new: made[i].name, folder: FS.parent(p), missing: made[i].missing }));
    const errors = rowErrors(rows, takenIn);
    const out = rows.map((r, i) => ({ path: r.path, old: r.old, new: r.new, ...(errors[i] ? { error: errors[i] } : {}), ...(r.missing.length ? { missing: r.missing } : {}) }));
    const done = { rows: out, changed: out.filter((r) => r.new !== r.old).length, errors: errors.filter(Boolean).length, done: false };
    if (b.dryRun) return [200, done];
    if (done.errors) return [409, { title: 'A row has an error, so nothing was renamed', ...done }];
    const todo = rows.map((r, i) => [nodes[i], r]).filter(([, r]) => r.new !== r.old);
    if (!todo.length) return [200, done];
    todo.forEach(([n, r]) => { n.name = r.new; });
    const dirs = [...new Set(todo.map(([, r]) => r.folder))];
    changed(...dirs);
    const undo = pushStep({ kind: 'renameBatch', name: todo[0][1].old, count: todo.length, path: dirs[0] }, () => { todo.forEach(([n, r]) => { n.name = r.old; }); changed(...dirs); });
    return [200, { ...done, done: true, undo }];
  }

  function link({ target, path, hard }) {
    const dir = dirAt(FS.parent(path || '')), name = base(path || '');
    if (!dir) return [404, 'Not found'];
    if (!name || !String(target || '').startsWith('/')) return [400, 'the target has to be a full path'];
    if (path === target) return [400, 'a link cannot point to itself'];
    if (taken(dir, name)) return [409, 'an entry with this name exists already'];
    const to = FS.nodeAt(target);
    let node;
    if (hard) {
      if (!to || to.dir) return [404, 'Not found'];
      node = { ...clone(to), name };
    } else node = to?.dir ? dirNode(name, [], { link: target, mode: '0777' }) : fileNode(name, to ? to.size : target.length, { link: target, mode: '0777' });
    dir.children.push(node);
    const at = FS.parent(path);
    changed(at);
    const undo = pushStep({ kind: 'link', name, path: at }, () => { dir.children = dir.children.filter((k) => k !== node); changed(at); });
    return [201, { path, undo }];
  }

  function attrsOf(b) {
    if (b.mode && !/^[0-7]{3,4}$/.test(b.mode)) return [400, 'mode must be three or four octal digits'];
    if (!b.mode && !b.owner && !b.group && !b.mtime) return [400, 'nothing to change'];
    if (!b.paths?.length) return [400, 'paths are required'];
    if (missingOf(b.paths)) return [404, 'Not found'];
    const set = (k) => {
      if (b.mode) k.mode = b.mode.padStart(4, '0');
      if (b.owner) k.owner = b.owner;
      if (b.group) k.group = b.group;
      if (b.mtime) k.time = Date.parse(b.mtime);
    };
    return { set, nodes: b.paths.map((p) => FS.nodeAt(p)) };
  }

  // Aufräumen as backend/internal/tools/cleanup.go finds things, with its notes as codes the client puts into words.
  const SIG = { png: 'png', jpg: 'jpg', pdf: 'pdf', zip: 'zip' };
  const TEMP = /(\.part|\.!qb|\.tmp|\.crdownload|\.partial)$/i;
  const JUNK = ['.ds_store', 'thumbs.db', 'desktop.ini'];
  const BAD = /[\\:*?"<>|]/;
  const badName = (n) => (n.trim() !== n ? 'edgeSpace' : n.endsWith('.') ? 'trailingDot' : BAD.test(n) ? 'windowsChars' : n.length > 255 ? 'tooLong' : '');
  const cleanName = (n) => n.trim().replace(/\.+$/, '').replace(new RegExp(BAD.source, 'g'), '_');
  const emptyDir = (n) => n.dir && !n.link && FS.kids(n).every(emptyDir);
  function cleanup(path) {
    const root = dirAt(path);
    if (!root || path.includes('::')) return null;
    const groups = { emptyDir: [], emptyFile: [], temp: [], link: [], ext: [], name: [] };
    let scanned = 0;
    const visit = (n, at) => {
      for (const k of FS.kids(n)) {
        scanned++;
        const p = FS.join(at, k.name);
        const why = badName(k.name);
        if (why) groups.name.push({ path: p, note: why, to: cleanName(k.name) });
        if (k.link) {
          if (!FS.nodeAt(k.link)) groups.link.push({ path: p, note: k.link });
        } else if (k.dir) {
          if (emptyDir(k)) groups.emptyDir.push({ path: p, note: FS.kids(k).length ? 'onlyEmptyDirs' : 'empty' });
          else visit(k, p);
        } else {
          if (TEMP.test(k.name) || k.name.startsWith('~$') || JUNK.includes(k.name.toLowerCase())) groups.temp.push({ path: p, size: Math.round(k.size || 0) });
          else if (!k.size) groups.emptyFile.push({ path: p });
          if (k.size && k.sig && SIG[k.sig] && FS.ext(k.name) !== SIG[k.sig]) {
            const dot = k.name.lastIndexOf('.');
            groups.ext.push({ path: p, note: k.sig, to: (dot > 0 ? k.name.slice(0, dot) : k.name) + '.' + SIG[k.sig] });
          }
        }
      }
    };
    visit(root, path);
    return { path, scanned, groups };
  }

  // What an archive holds; the zips of the data list their own, the others get a list that fits their name.
  function contentsOf(n) {
    if (n.zip) return n.zip.map(clone);
    const r = mk.rng(n.name), name = stripArchive(n.name), size = n.size || MB;
    if (/appdata/i.test(n.name)) {
      const apps = ['bombvault', 'knightloader', 'plex', 'nginx', 'homarr'];
      return [dirNode('appdata', apps.map((a, i) => dirNode(a, [fileNode(i % 2 ? 'config.xml' : 'settings.json', 8 * KB), fileNode(`${a}.db`, size * 1.4 * (0.1 + r() * 0.3))], { time: n.time })), { time: n.time })];
    }
    if (/flash/i.test(n.name)) return [dirNode('config', [fileNode('ident.cfg', KB), fileNode('go', KB), fileNode('super.dat', 4 * KB)], { time: n.time }), fileNode('bzimage', 9 * MB, { time: n.time }), fileNode('bzroot', 140 * MB, { time: n.time })];
    const count = 3 + Math.floor(r() * 4);
    return [dirNode(name, Array.from({ length: count }, (_, i) => fileNode(`${name}-${String(i + 1).padStart(2, '0')}.${['jpg', 'pdf', 'txt', 'mkv'][Math.floor(r() * 4)]}`, (size * 1.05) / count, { time: n.time })), { time: n.time })];
  }
  const sizeOf = (list) => list.reduce((a, k) => a + FS.totalSize(k), 0);
  const entriesOf = (list) => list.reduce((a, k) => a + 1 + (k.dir ? entriesOf(FS.kids(k)) : 0), 0);
  // The archive at the front of a path such as /x/a.zip::inner/folder, and the folder inside it.
  function archiveAt(path) {
    const at = path.indexOf('::');
    const n = FS.nodeAt(at < 0 ? path : path.slice(0, at));
    if (!isArchive(n)) return null;
    let list = contentsOf(n);
    for (const part of (at < 0 ? '' : path.slice(at + 2)).split('/').filter(Boolean)) list = FS.kids(list.find((k) => k.name === part && k.dir) || null);
    return { node: n, list };
  }
  const locked = (n, password) => n.locked && !password;

  function packedSize(nodes, fmt, level) {
    const squeezed = (n) => (['img', 'video', 'audio', 'arch', 'disk'].includes(FS.kindOf(n)) ? 0.97 : FS.kindOf(n) === 'pdf' || FS.kindOf(n) === 'office' ? 0.82 : 0.3);
    const rawOf = (n) => (n.dir ? FS.kids(n).reduce((a, k) => a + rawOf(k), 0) : (n.size || 0) * (level === 0 ? 1 : squeezed(n) * (1.06 - level * 0.012) * (fmt === '7z' ? 0.94 : fmt === 'tar.zst' ? 0.96 : 1)));
    return Math.max(120, nodes.reduce((a, n) => a + rawOf(n), 0));
  }

  const filesUnder = (path, n) => (n.dir ? FS.kids(n).flatMap((k) => filesUnder(FS.join(path, k.name), k)) : [[n, path]]);

  const kinds = {
    sync({ left, right, steps = [] }) {
      if (!dirAt(left) || !dirAt(right)) return [404, 'Not found'];
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
              const victim = detach(FS.join(into, s.rel));
              if (victim) toTrash(victim, FS.join(into, s.rel));
            } else {
              const src = FS.nodeAt(FS.join(from, s.rel));
              if (!src) continue;
              const dir = ensureDir(parent);
              dir.children = dir.children.filter((c) => c.name !== src.name);
              dir.children.push(clone(src));
            }
            changed(parent);
          }
        },
      };
    },
    duplicates({ path, by = 'name', hidden: all = false }) {
      const n = dirAt(path);
      if (!n) return [404, 'Not found'];
      return { name: base(path), dest: path, count: 1, files: Math.max(1, countFiles(n, all)), total: 0, apply(j) { result(j, duplicates(path, by, all)); } };
    },
    // A text inside files: every hit the search window's other criteria find, read for the text, upper and lower case
    // alike, binary files passed over.
    contentSearch(p) {
      if (!String(p.text || '').trim()) return [400, 'text is required'];
      const found = fx.search(p.path, { ...p, limit: 10000 });
      if (!found) return [404, 'Not found'];
      if (found.unknown.length) return [400, 'these fields are unknown: ' + found.unknown.join(', ')];
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
          send('search.hits', { job: j.id, items, total: items.length, scanned: found.scanned, done: true });
          result(j, { items: items.slice(0, 2000), total: items.length, scanned: found.scanned });
        },
      };
    },
    pack(p) {
      const sources = p.sources || [], level = p.level ?? 6;
      if (!PACK.includes(p.format)) return [400, 'format must be zip, tar.gz, tar.zst, 7z or rar'];
      if (p.password && p.format.startsWith('tar')) return [400, 'tar archives cannot have a password'];
      if (!String(p.name || '').trim() || /[/\0]/.test(p.name)) return [400, 'invalid name'];
      if (!sameDir(sources)) return [400, 'the sources have to lie in one folder'];
      if (missingOf(sources) || !dirAt(p.dest)) return [404, 'Not found'];
      const file = `${p.name}.${p.format}`, nodes = sources.map((s) => FS.nodeAt(s));
      return {
        paths: sources, name: file, dest: p.dest, total: sizeOf(nodes),
        apply: (j) => {
          const dir = dirAt(p.dest), name = freeName(dir, file);
          dir.children.push(fileNode(name, packedSize(nodes, p.format, level), { zip: nodes.map(clone), ...(p.password ? { locked: true } : {}) }));
          changed(p.dest);
          result(j, { path: FS.join(p.dest, name) });
        },
      };
    },
    unpack(p) {
      const archives = p.archives || [], clash = p.clash || 'both';
      if (!archives.length) return [400, 'archives are required'];
      if (!dirAt(p.dest) || p.dest.includes('::')) return [404, 'Not found'];
      const found = archives.map((a) => archiveAt(a));
      if (found.some((f) => !f)) return [400, 'this is not an archive that can be unpacked'];
      const jobs = found.map((f, i) => ({ ...f, list: p.items?.length ? f.list.filter((k) => p.items.includes(k.name)) : f.list, path: archives[i] }));
      return {
        paths: archives, name: found[0].node.name, count: archives.length, dest: p.dest, total: jobs.reduce((a, j) => a + sizeOf(j.list), 0),
        files: jobs.reduce((a, j) => a + entriesOf(j.list), 0), names: jobs.map((j) => j.node.name),
        apply: (job) => {
          if (jobs.some((j) => locked(j.node, p.password))) throw new Error('the archive needs its password');
          const landed = [];
          let skipped = 0;
          for (const j of jobs) {
            let target = dirAt(p.dest), into = p.dest;
            if (p.own) {
              const folder = stripArchive(j.node.name);
              let d = FS.kids(target).find((k) => k.name === folder && k.dir);
              if (!d) target.children.push(d = dirNode(folder));
              target = d; into = FS.join(p.dest, folder);
              landed.push(into);
            }
            for (const item of j.list.map(clone)) {
              const there = FS.kids(target).find((k) => k.name === item.name);
              if (there && clash === 'skip') { skipped++; continue; }
              if (there && clash === 'replace') target.children = target.children.filter((k) => k !== there);
              if (there && clash === 'both') item.name = freeName(target, item.name);
              item.time ||= now();
              target.children.push(item);
              if (!p.own) landed.push(FS.join(into, item.name));
            }
          }
          changed(p.dest);
          result(job, { paths: landed, skipped });
        },
      };
    },
    testArchive(p) {
      const paths = p.paths || [];
      if (!paths.length) return [400, 'paths are required'];
      if (missingOf(paths)) return [404, 'Not found'];
      const bad = paths.map((s) => FS.nodeAt(s)).find((n) => !isArchive(n));
      if (bad) return [400, `${bad.name} is not an archive that can be tested`];
      const nodes = paths.map((s) => FS.nodeAt(s));
      // A test goes through the members, as the prototype plays it, so its time follows their number, not the bytes.
      const members = nodes.map((n) => 6 + (fx.hashOf(n.name) % 140));
      return {
        paths, total: members.reduce((a, m) => a + m, 0) * 80 * MB, names: nodes.map((n) => n.name),
        apply: (j) => result(j, {
          archives: nodes.map((n, i) => {
            if (locked(n, p.password)) return { path: paths[i], members: 0, ok: false, password: true };
            if (!n.flaw) return { path: paths[i], members: members[i], ok: true };
            const [reason, entry] = n.flaw.split(' bei ');
            return { path: paths[i], members: Math.ceil(members[i] * 0.6), ok: false, damage: { entry, reason } };
          }),
        }),
      };
    },
    checksum(p) {
      const sources = p.sources || [], algo = p.algo || 'sha256';
      if (!ALGOS[algo]) return [400, 'algo must be md5, sha1, sha256, sha512 or b3'];
      if (!sameDir(sources)) return [400, 'the sources have to lie in one folder'];
      if (missingOf(sources)) return [404, 'Not found'];
      if (p.save && !dirAt(FS.parent(p.save))) return [404, 'Not found'];
      const folder = FS.parent(sources[0]);
      const files = sources.flatMap((s) => filesUnder(s, FS.nodeAt(s))).filter(([n]) => !SUMFILE.test(n.name)).map(([n, path]) => ({ n, path, name: path.slice(folder.length + 1) }));
      return {
        paths: sources, total: files.reduce((a, f) => a + (f.n.size || 0), 0), files: files.length, names: files.map((f) => f.name), dest: p.save ? FS.parent(p.save) : undefined,
        apply: (j) => {
          const sums = files.map((f) => ({ path: f.path, name: f.name, hash: mk.hashOf(algo, f.n) }));
          if (p.save) {
            const at = FS.parent(p.save), dir = dirAt(at);
            const text = sums.map((s) => (p.bsd ? `${ALGOS[algo][0]} (${s.path.startsWith(at + '/') ? s.path.slice(at.length + 1) : s.path}) = ${s.hash}` : `${s.hash}  ${s.path.startsWith(at + '/') ? s.path.slice(at.length + 1) : s.path}`)).join('\n') + '\n';
            dir.children = dir.children.filter((k) => k.name !== base(p.save));
            dir.children.push(fileNode(base(p.save), text.length, { text }));
            changed(at);
          }
          result(j, { algo, files: sums, ...(p.save ? { saved: p.save } : {}) });
        },
      };
    },
    verify(p) {
      let text = p.text || '', dir = p.dir, listName = '';
      if (p.file) {
        const n = FS.nodeAt(p.file);
        if (!n || n.dir) return [404, 'Not found'];
        text = n.text ?? ''; dir = FS.parent(p.file); listName = n.name;
      } else if (!dirAt(dir || '')) return [404, 'Not found'];
      const lines = parseSums(text, listName);
      if (!lines.length) return [400, 'there are no checksums in it'];
      const found = lines.map((l) => {
        if (l.bad) return null;
        const path = l.name.startsWith('/') ? l.name : FS.join(dir, l.name), n = FS.nodeAt(path);
        return n && !n.dir ? { n, path } : null;
      });
      return {
        paths: [], name: listName || lines[0].name || lines[0].bad, count: lines.length, files: lines.length, names: lines.map((l) => l.name || l.bad),
        total: found.reduce((a, f) => a + (f?.n.size || 0), 0),
        apply: (j) => result(j, {
          lines: lines.map((l, i) => (l.bad ? { name: l.bad, state: 'bad' }
            : !found[i] ? { name: l.name, algo: l.algo, state: 'missing' }
              : { name: l.name, path: found[i].path, algo: l.algo, state: mk.hashOf(l.algo, found[i].n) === l.hash ? 'ok' : 'mismatch' })),
        }),
      };
    },
    split(p) {
      const n = FS.nodeAt(p.path || '');
      if (!n || n.dir || !dirAt(p.dest || '')) return [404, 'Not found'];
      if (!(p.partSize >= MB)) return [400, 'a part has to be at least 1 MiB'];
      if (n.size <= p.partSize) return [400, 'the file is smaller than one part'];
      const count = Math.ceil(n.size / p.partSize), pad = Math.max(3, String(count).length);
      const names = Array.from({ length: count }, (_, i) => `${n.name}.${String(i + 1).padStart(pad, '0')}`);
      return {
        paths: [p.path], dest: p.dest, total: n.size,
        apply: (j) => {
          const d = dirAt(p.dest);
          // Whole bytes, so the parts add up to the size the sum file was made for.
          const size = Math.round(n.size);
          names.forEach((name, i) => {
            d.children = d.children.filter((k) => k.name !== name);
            d.children.push(fileNode(name, i < count - 1 ? p.partSize : size - p.partSize * (count - 1)));
          });
          if (p.sum) {
            const text = `${mk.hashOf('sha256', { name: n.name, size })}  ${n.name}\n`;
            d.children = d.children.filter((k) => k.name !== n.name + '.sha256');
            d.children.push(fileNode(n.name + '.sha256', text.length, { text }));
          }
          changed(p.dest);
          result(j, { parts: count, first: FS.join(p.dest, names[0]) });
        },
      };
    },
    combine(p) {
      const at = FS.parent(p.path || ''), pick = base(p.path || '');
      if (!/\.\d{3,}$/.test(pick)) return [400, 'this is not a part such as file.001'];
      if (!dirAt(at) || !dirAt(p.dest || '')) return [404, 'Not found'];
      const stem = pick.replace(/\.\d{3,}$/, '');
      const parts = FS.kids(dirAt(at)).filter((k) => !k.dir && k.name.startsWith(stem + '.') && /^\d{3,}$/.test(k.name.slice(stem.length + 1)))
        .sort((a, b) => a.name.localeCompare(b.name, 'de', { numeric: true }));
      const nums = parts.map((k) => +k.name.slice(stem.length + 1));
      const gap = Array.from({ length: Math.max(...nums) }, (_, i) => i + 1).find((i) => !nums.includes(i));
      if (gap) return [400, `part ${String(gap).padStart(3, '0')} is missing`];
      if (parts.length < 2) return [400, 'there is only one part'];
      const name = (p.name || stem).trim();
      if (!name || /[/\0]/.test(name)) return [400, 'invalid name'];
      const sum = FS.kids(dirAt(at)).find((k) => k.name === stem + '.sha256');
      if (p.check && !sum) return [400, `there is no ${stem}.sha256 beside the parts to check against`];
      const total = parts.reduce((a, k) => a + k.size, 0);
      return {
        paths: parts.map((k) => FS.join(at, k.name)), name, count: 1, dest: p.dest, total,
        apply: (j) => {
          const d = dirAt(p.dest), file = freeName(d, name);
          d.children.push(fileNode(file, total));
          const good = !p.check || mk.hashOf('sha256', { name: stem, size: total }) === String(sum.text || '').trim().split(/\s+/)[0];
          if (good && p.drop) {
            const src = dirAt(at);
            parts.forEach((k) => toTrash(k, FS.join(at, k.name)));
            src.children = src.children.filter((k) => !parts.includes(k));
          }
          changed(p.dest, at);
          if (!good) throw new Error('the checksum of the result does not match, the parts stay where they are');
          result(j, { path: FS.join(p.dest, file), checked: !!p.check });
        },
      };
    },
    attrsRecursive(p) {
      const got = attrsOf(p);
      if (Array.isArray(got)) return got;
      let files = 0;
      got.nodes.forEach((n) => walk(n, () => files++));
      return { paths: p.paths, total: 0, files, apply: () => { got.nodes.forEach((n) => walk(n, got.set)); changed(...p.paths.map(FS.parent)); } };
    },
    fetchUrl(p) {
      let u;
      try { u = new URL(String(p.url || '').trim()); } catch { return [400, 'this is not a web address']; }
      if (!/^https?:$/.test(u.protocol)) return [400, 'only http and https addresses can be loaded'];
      if (u.username || u.password) return [400, 'an address with a user name or password cannot be loaded'];
      if (/^(localhost|127\.|10\.|192\.168\.|172\.(1[6-9]|2\d|3[01])\.|169\.254\.|\[::1\]|\[f[cd])/i.test(u.hostname) || u.hostname.endsWith('.local')) {
        return [400, 'addresses in the local network or on the server itself cannot be loaded'];
      }
      const last = u.pathname.split('/').filter(Boolean).pop();
      let wanted = p.name;
      if (!wanted) { try { wanted = decodeURIComponent(last || '') || u.hostname; } catch { wanted = last; } }
      if (!wanted || /[/\0]/.test(wanted)) return [400, 'the file name is missing or holds a slash'];
      if (!dirAt(p.dest || '') || p.dest.includes('::')) return [404, 'Not found'];
      const size = (['iso', 'img'].includes(FS.ext(wanted)) ? 900 + (fx.hashOf(u.href) % 5200) : 2 + (fx.hashOf(u.href) % 400)) * MB;
      return {
        paths: [], name: wanted, count: 1, dest: p.dest, total: size, files: 1, names: [wanted], speed: 0.45,
        apply: (j) => {
          const d = dirAt(p.dest), name = freeName(d, wanted);
          d.children.push(fileNode(name, size));
          changed(p.dest);
          result(j, { path: FS.join(p.dest, name) });
        },
      };
    },
  };

  // A QR code's look for the fixture: finder squares and modules seeded by the address, as the prototype draws it.
  function qrSvg(text) {
    const N = 29, r = mk.rng(text), on = Array.from({ length: N }, () => Array(N).fill(false)), fixed = Array.from({ length: N }, () => Array(N).fill(false));
    const finder = (x, y) => {
      for (let i = -1; i <= 7; i++) for (let j = -1; j <= 7; j++) {
        const a = x + i, b = y + j;
        if (a < 0 || b < 0 || a >= N || b >= N) continue;
        fixed[b][a] = true;
        const ring = Math.max(Math.abs(i - 3), Math.abs(j - 3));
        on[b][a] = ring !== 2 && ring <= 3;
      }
    };
    finder(0, 0); finder(N - 7, 0); finder(0, N - 7);
    for (let i = 8; i < N - 8; i++) { on[6][i] = on[i][6] = i % 2 === 0; fixed[6][i] = fixed[i][6] = true; }
    for (let y = 0; y < N; y++) for (let x = 0; x < N; x++) if (!fixed[y][x]) on[y][x] = r() < 0.48;
    let d = '';
    for (let y = 0; y < N; y++) for (let x = 0; x < N; x++) if (on[y][x]) d += `M${x + 2} ${y + 2}h1v1h-1z`;
    return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${N + 4} ${N + 4}" shape-rendering="crispEdges" role="img" aria-label="QR code"><rect width="${N + 4}" height="${N + 4}" fill="#fff"/><path d="${d}" fill="#161616"/></svg>`;
  }

  // Share links in the shape of backend/internal/shares, starting with two of the prototype's links.
  const SLUG = /^[a-z0-9][a-z0-9-]{2,39}$/;
  const t0 = now();
  const shares = [
    { id: 's1', slug: 'k3x9qa-urlaub', kind: 'folder', name: 'Urlaub', paths: ['/mnt/user/fotos/Urlaub'], owner: 'jdp', created: t0 - 5 * DAY, expires: t0 + 25 * DAY, password: true, limit: 0, downloads: 3, views: 14, uploads: false, uploadLimit: 0, uploaded: 0, notify: false },
    { id: 's2', slug: 'p7q2mv-rechnung', kind: 'file', name: 'Rechnung-Oktober.pdf', paths: ['/mnt/user/dokumente/Rechnung-Oktober.pdf'], owner: 'jdp', created: t0 - DAY, expires: t0 + 6 * DAY, password: false, limit: 5, downloads: 1, views: 2, uploads: false, uploadLimit: 0, uploaded: 0, notify: true },
  ];
  const shareOut = (s) => ({
    ...s, created: iso(s.created), expires: s.expires == null ? null : iso(s.expires),
    state: s.expires != null && s.expires < now() ? 'expired' : s.limit && s.downloads >= s.limit ? 'exhausted' : 'active',
  });
  const shareAt = (id) => shares.find((s) => s.id === id);
  const freshSlug = (name) => {
    const words = name.toLowerCase().replace(/\.[^.]+$/, '').normalize('NFD').replace(/[^\w]+/g, '-').replace(/^-|-$/g, '').slice(0, 18) || 'link';
    for (;;) {
      const slug = `${Math.random().toString(36).slice(2, 8).padEnd(6, '0')}-${words}`;
      if (!shares.some((s) => s.slug === slug)) return slug;
    }
  };
  const slugProblem = (slug, self) => (!SLUG.test(slug) ? [400, 'the address takes 3 to 40 small letters, digits and dashes'] : shares.some((s) => s.slug === slug && s !== self) ? [409, 'The address is taken'] : null);
  function createShare(b) {
    const paths = b.paths || [];
    if (!paths.length) return [400, 'paths are required'];
    if (missingOf(paths) || paths.some((p) => p.includes('::'))) return [404, 'Not found'];
    const kind = paths.length > 1 ? 'collection' : FS.nodeAt(paths[0]).dir ? 'folder' : 'file';
    if (kind === 'collection' && !String(b.name || '').trim()) return [400, 'a collection needs a name'];
    if (b.password != null && b.password !== '' && b.password.length < 6) return [400, 'the password needs at least 6 characters'];
    if (b.uploads && kind !== 'folder') return [400, 'only a link to one folder takes uploads'];
    const slug = b.slug ? String(b.slug).toLowerCase() : freshSlug(kind === 'collection' ? b.name : base(paths[0]));
    const bad = slugProblem(slug);
    if (bad) return bad;
    const s = {
      id: nextId('s'), slug, kind, name: kind === 'collection' ? b.name.trim() : base(paths[0]), paths, owner: 'jdp', created: now(),
      expires: b.expires ? Date.parse(b.expires) : null, password: !!b.password, limit: b.limit || 0, downloads: 0, views: 0,
      uploads: !!b.uploads, uploadLimit: b.uploadLimit || 0, uploaded: 0, notify: !!b.notify,
    };
    shares.unshift(s);
    // The fixture has no visitors, so someone opens a link that asks for a note a few seconds later.
    if (s.notify) {
      clock.setTimeout(() => {
        if (!shares.includes(s)) return;
        s.views++;
        send('share.opened', { id: s.id, slug: s.slug, name: s.name, views: s.views });
      }, 6000).unref?.();
    }
    return [201, shareOut(s)];
  }
  function updateShare(s, b) {
    if (b.reroll) s.slug = freshSlug(s.name);
    if (b.slug != null) {
      const bad = slugProblem(String(b.slug).toLowerCase(), s);
      if (bad) return bad;
      s.slug = String(b.slug).toLowerCase();
    }
    if ('expires' in b) s.expires = b.expires ? Date.parse(b.expires) : null;
    if ('password' in b) {
      if (b.password && b.password.length < 6) return [400, 'the password needs at least 6 characters'];
      s.password = !!b.password;
    }
    for (const k of ['name', 'limit', 'uploads', 'uploadLimit', 'notify']) if (k in b) s[k] = b[k];
    return [200, shareOut(s)];
  }

  const presets = { items: [] };
  const profiles = [];
  let profileId = 0;

  async function archiveStream(req, res) {
    const text = (await raw(req)).toString('utf8');
    const form = /urlencoded/.test(req.headers['content-type'] || '') ? new URLSearchParams(text) : null;
    let body = {};
    if (!form) try { body = JSON.parse(text || '{}'); } catch { /* an empty request asks for nothing */ }
    const paths = form ? form.getAll('paths') : body.paths || [];
    const format = (form ? form.get('format') : body.format) || 'zip';
    if (!paths.length) return [400, 'paths are required'];
    if (missingOf(paths)) return [404, 'Not found'];
    const name = (form ? form.get('name') : body.name) || `${paths.length === 1 ? base(paths[0]) : base(FS.parent(paths[0])) || 'download'}.${format === 'tgz' ? 'tar.gz' : 'zip'}`;
    res.statusCode = 200;
    res.setHeader('Content-Type', format === 'tgz' ? 'application/gzip' : 'application/zip');
    res.setHeader('Content-Disposition', `attachment; filename*=UTF-8''${encodeURIComponent(name)}`);
    res.end(EMPTY_ZIP);
    return true;
  }
  archiveStream.raw = true;

  const routes = [
    ['GET', /^\/search\/saved$/, () => [200, { items: saved }]],
    ['POST', /^\/search\/saved$/, (_r, _s, b) => {
      const bad = savedInput(b);
      if (bad) return [400, bad];
      if (saved.length >= 200) return [409, 'There are 200 saved searches already'];
      const s = { id: ++lastId, name: b.name.trim(), query: b.query };
      saved.push(s);
      return [201, s];
    }],
    ['PUT', /^\/search\/saved\/(\d+)$/, (_r, _s, b, [, id]) => {
      const s = saved.find((x) => x.id === Number(id));
      if (!s) return [404, 'Not found'];
      const bad = savedInput(b);
      if (bad) return [400, bad];
      Object.assign(s, { name: b.name.trim(), query: b.query });
      return [200, s];
    }],
    ['DELETE', /^\/search\/saved\/(\d+)$/, (_r, _s, _b, [, id]) => {
      const at = saved.findIndex((x) => x.id === Number(id));
      if (at < 0) return [404, 'Not found'];
      saved.splice(at, 1);
      return [204];
    }],

    ['POST', /^\/tools\/compare$/, (_r, _s, b) => {
      if (!dirAt(b.left) || !dirAt(b.right)) return [404, 'Not found'];
      const items = compare(b.left, b.right, b).map(compared);
      const counts = {};
      for (const it of items) counts[it.state] = (counts[it.state] || 0) + 1;
      return [200, { items, counts }];
    }],
    ['POST', /^\/tools\/sync\/plan$/, (_r, _s, b) => {
      if (!['right', 'left', 'both'].includes(b.direction)) return [400, 'direction is right, left or both'];
      if (!dirAt(b.left) || !dirAt(b.right)) return [404, 'Not found'];
      return [200, { steps: plan(compare(b.left, b.right, { content: b.content, recursive: b.recursive, hidden: b.hidden }), b.direction, b.mirror && b.direction !== 'both') }];
    }],
    ['POST', /^\/tools\/diff$/, (_r, _s, b) => { const d = diff(b.left, b.right); return d ? [200, d] : [404, 'Not found']; }],
    ['POST', /^\/tools\/duplicates$/, (_r, _s, b) => startJob('duplicates', b)],
    ['GET', /^\/tools\/diskusage$/, (_r, _s, _b, _m, q) => {
      const u = usage(q.get('path') || '/', q.get('hidden') === 'true', q.get('kind') || '', Number(q.get('largest') ?? 200));
      return u ? [200, u] : [404, 'Not found'];
    }],
    ['POST', /^\/tools\/arrowloop$/, (_r, _s, b) => {
      if (!b.name || !dirAt(b.left) || !dirAt(b.right) || !['right', 'left', 'both'].includes(b.direction) || !['hourly', 'daily', 'weekly'].includes(b.every)) {
        return [400, 'a name, two folders, the direction and how often are required'];
      }
      if (b.at && !/^([01]\d|2[0-3]):[0-5]\d$/.test(b.at)) return [400, 'at is a time such as 03:00'];
      return [201, { job: { id: nextId('al'), ...b, enabled: true }, url: 'https://arrowloop.example.org' }];
    }],
    ['POST', /^\/tools\/cleanup\/scan$/, (_r, _s, b) => { const got = cleanup(b.path || ''); return got ? [200, got] : [404, 'Not found']; }],
    ['POST', /^\/tools\/checksum\/parse$/, (_r, _s, b) => {
      if (!b.path) return [200, { lines: parseSums(b.text || '') }];
      const n = FS.nodeAt(b.path);
      return n && !n.dir ? [200, { lines: parseSums(n.text ?? '', n.name) }] : [404, 'Not found'];
    }],
    ['GET', /^\/tools\/archives$/, () => [200, { read: ['zip', 'tar', 'tar.gz', 'tar.zst', 'tar.xz', 'tar.bz2', '7z', 'rar'], pack: PACK }]],
    ['GET', /^\/tools\/archive$/, (req, _s, _b, _m, q) => {
      const path = q.get('path') || '', found = archiveAt(path);
      if (!FS.nodeAt(path.split('::')[0])) return [404, 'Not found'];
      if (!found) return [422, 'this is not an archive'];
      if (locked(found.node, req.headers['x-archive-password'])) return [428, 'the archive needs its password'];
      return [200, { path, format: formatOf(found.node.name) || 'zip', entries: entriesOf(found.list), unpacked: Math.round(sizeOf(found.list)), encrypted: !!found.node.locked }];
    }],

    ['POST', /^\/fs\/rename\/batch$/, (_r, _s, b) => renameBatch(b)],
    ['POST', /^\/fs\/link$/, (_r, _s, b) => link(b)],
    ['PATCH', /^\/fs\/attrs$/, (_r, _s, b) => {
      if (b.recursive) return startJob('attrsRecursive', b);
      const got = attrsOf(b);
      if (Array.isArray(got)) return got;
      got.nodes.forEach(got.set);
      changed(...b.paths.map(FS.parent));
      return [200, { changed: got.nodes.length }];
    }],
    ['POST', /^\/fs\/archive-stream$/, archiveStream],

    ['GET', /^\/shares$/, () => [200, shares.filter((s) => s.owner === 'jdp').map(shareOut)]],
    ['GET', /^\/admin\/shares$/, () => [200, shares.map(shareOut)]],
    ['POST', /^\/shares$/, (_r, _s, b) => createShare(b)],
    ['GET', /^\/shares\/([^/]+)\/qr$/, (_r, _s, _b, [, id], q) => {
      const s = shareAt(id);
      if (!s) return [404, 'there is no such link'];
      const origin = q.get('origin') || 'http://localhost:5173';
      if (!/^https?:\/\/[^/@?#]+\/?$/.test(origin)) return [400, 'origin has to be a scheme and a host, such as https://files.example.org'];
      const url = origin.replace(/\/$/, '') + '/s/' + s.slug;
      return [200, { url, qr: qrSvg(url) }];
    }],
    ['GET', /^\/shares\/([^/]+)$/, (_r, _s, _b, [, id]) => (shareAt(id) ? [200, shareOut(shareAt(id))] : [404, 'there is no such link'])],
    ['PATCH', /^\/shares\/([^/]+)$/, (_r, _s, b, [, id]) => (shareAt(id) ? updateShare(shareAt(id), b) : [404, 'there is no such link'])],
    ['DELETE', /^\/shares\/([^/]+)$/, (_r, _s, _b, [, id]) => {
      const s = shareAt(id);
      if (!s) return [404, 'there is no such link'];
      shares.splice(shares.indexOf(s), 1);
      return [204];
    }],

    ['GET', /^\/me\/rename-presets$/, () => [200, presets]],
    ['PUT', /^\/me\/rename-presets$/, (_r, _s, b) => {
      const items = b.items || [];
      if (new Set(items.map((p) => p.name)).size !== items.length) return [409, 'Two presets share a name'];
      presets.items = items;
      return [204];
    }],
    ['GET', /^\/me\/profiles$/, () => [200, { items: profiles }]],
    ['POST', /^\/me\/profiles$/, (_r, _s, b) => {
      if (!String(b.name || '').trim()) return [400, 'a profile needs a name'];
      profiles.push({ id: ++profileId, name: b.name.trim(), dual: !!b.dual, columns: b.columns || [], panels: b.panels || [] });
      return [201, profiles.at(-1)];
    }],
    ['DELETE', /^\/me\/profiles\/(\d+)$/, (_r, _s, _b, [, id]) => {
      const at = profiles.findIndex((p) => p.id === +id);
      if (at < 0) return [404, 'there is no such profile'];
      profiles.splice(at, 1);
      return [204];
    }],
  ];
  return { routes, kinds };
}
