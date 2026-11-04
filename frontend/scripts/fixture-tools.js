// Batch rename, links and rights, archives, checksums, splitting, clean up, loading from an address, the hand over to
// ArrowLoop, share links and rename presets, played on the prototype's tree in the shapes of the contracts in
// backend/internal/{tools,fileops,fsapi,shares,prefs}. The long runs go through the job engine of fixture-ops.js.
import { applyMask, rowErrors } from './rename-mask.js';

const KB = 1024, MB = KB * 1024, DAY = 86400000;

// The prototype's made-up checksums (b-00-kit.js): copies with the same name and size hash the same.
const strHash = (s) => { let h = 2166136261; for (const c of String(s)) { h ^= c.codePointAt(0); h = Math.imul(h, 16777619); } return h >>> 0; };
const rng = (key) => {
  let a = strHash(key) || 1;
  return () => { a = (a + 0x6D2B79F5) | 0; let t = Math.imul(a ^ (a >>> 15), 1 | a); t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t; return ((t ^ (t >>> 14)) >>> 0) / 4294967296; };
};
const hex = (key, len) => { const r = rng(key); let s = ''; while (s.length < len) s += Math.floor(r() * 16).toString(16); return s; };
const ALGOS = { md5: ['MD5', 32], sha1: ['SHA1', 40], sha256: ['SHA256', 64], sha512: ['SHA512', 128], b3: ['BLAKE3', 64] };
const hashOf = (algo, n) => hex(`${algo}|${n.name}|${n.size}`, ALGOS[algo][1]);
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

// A QR code's look for the fixture: finder squares and modules seeded by the address, as the prototype draws it.
function qrSvg(text) {
  const N = 29, r = rng(text), on = Array.from({ length: N }, () => Array(N).fill(false)), fixed = Array.from({ length: N }, () => Array(N).fill(false));
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

// The smallest zip there is, so a download from the fixture saves a file that opens.
const EMPTY_ZIP = Buffer.from([0x50, 0x4b, 0x05, 0x06, ...Array(18).fill(0)]);

export function toolsFixture(engine) {
  const { FS, fx, send, changed, createJob, view, dirAt, taken, freeName, toTrash, pushStep, nextId, iso, base, clone, raw } = engine;
  const now = () => Date.now();
  const fileNode = (name, size, extra = {}) => ({ name, size: Math.round(size), time: now(), owner: 'nobody', group: 'users', mode: '0664', ...extra });
  const dirNode = (name, children = [], extra = {}) => ({ name, dir: true, children, time: now(), owner: 'nobody', group: 'users', mode: '0775', ...extra });
  const sameDir = (paths) => paths.length && paths.every((p) => FS.parent(p) === FS.parent(paths[0]));
  const missingOf = (paths) => paths.find((p) => !FS.nodeAt(p));
  const isArchive = (n) => !!n && !n.dir && (!!n.zip || FS.kindOf(n) === 'arch' || /\.(7z|rar|zip)\.0*1$/i.test(n.name));
  const walk = (n, fn) => { fn(n); if (n.dir && !n.link) FS.kids(n).forEach((k) => walk(k, fn)); };

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
    const result = { rows: out, changed: out.filter((r) => r.new !== r.old).length, errors: errors.filter(Boolean).length, done: false };
    if (b.dryRun) return [200, result];
    if (result.errors) return [409, { title: 'A row has an error, so nothing was renamed', ...result }];
    const todo = rows.map((r, i) => [nodes[i], r]).filter(([, r]) => r.new !== r.old);
    if (!todo.length) return [200, result];
    todo.forEach(([n, r]) => { n.name = r.new; });
    const dirs = [...new Set(todo.map(([, r]) => r.folder))];
    changed(...dirs);
    const undo = pushStep({ kind: 'renameBatch', name: todo[0][1].old, count: todo.length, path: dirs[0] }, () => { todo.forEach(([n, r]) => { n.name = r.old; }); changed(...dirs); });
    return [200, { ...result, done: true, undo }];
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
  const attrsJob = (b) => {
    const got = attrsOf(b);
    if (Array.isArray(got)) return got;
    let files = 0;
    got.nodes.forEach((n) => walk(n, () => files++));
    return { paths: b.paths, total: 0, files, apply: () => { got.nodes.forEach((n) => walk(n, got.set)); changed(...b.paths.map(FS.parent)); } };
  };

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
    const r = rng(n.name), name = stripArchive(n.name), size = n.size || MB;
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

  const result = (j, value) => { j.result = value; };
  const filesUnder = (path, n) => (n.dir ? FS.kids(n).flatMap((k) => filesUnder(FS.join(path, k.name), k)) : [[n, path]]);

  const kinds = {
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
      return {
        paths, total: sizeOf(nodes), names: nodes.map((n) => n.name),
        apply: (j) => result(j, {
          archives: nodes.map((n, i) => {
            const members = 6 + (fx.hashOf(n.name) % 140);
            if (locked(n, p.password)) return { path: paths[i], members: 0, ok: false, password: true };
            if (!n.flaw) return { path: paths[i], members, ok: true };
            const [reason, entry] = n.flaw.split(' bei ');
            return { path: paths[i], members: Math.ceil(members * 0.6), ok: false, damage: { entry, reason } };
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
          const sums = files.map((f) => ({ path: f.path, name: f.name, hash: hashOf(algo, f.n) }));
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
              : { name: l.name, path: found[i].path, algo: l.algo, state: hashOf(l.algo, found[i].n) === l.hash ? 'ok' : 'mismatch' })),
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
            const text = `${hashOf('sha256', { name: n.name, size })}  ${n.name}\n`;
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
          const good = !p.check || hashOf('sha256', { name: stem, size: total }) === String(sum.text || '').trim().split(/\s+/)[0];
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
    attrsRecursive: (p) => attrsJob({ ...p, recursive: true }),
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
        paths: [], name: wanted, count: 1, dest: p.dest, total: size, files: 1, names: [wanted],
        apply: (j) => {
          const d = dirAt(p.dest), name = freeName(d, wanted);
          d.children.push(fileNode(name, size));
          changed(p.dest);
          result(j, { path: FS.join(p.dest, name) });
        },
      };
    },
  };

  // Share links in the shape of backend/internal/shares, starting with two of the prototype's links.
  const SLUG = /^[a-z0-9][a-z0-9-]{2,39}$/;
  const t0 = now();
  const shares = [
    { id: 's1', slug: 'k3x9-urlaub', kind: 'folder', name: 'Urlaub', paths: ['/mnt/user/fotos/Urlaub'], owner: 'jdp', created: t0 - 5 * DAY, expires: t0 + 25 * DAY, password: true, limit: 0, downloads: 3, views: 14, uploads: false, uploadLimit: 0, uploaded: 0, notify: false },
    { id: 's2', slug: 'p7q2-rechnung', kind: 'file', name: 'Rechnung-Oktober.pdf', paths: ['/mnt/user/dokumente/Rechnung-Oktober.pdf'], owner: 'jdp', created: t0 - DAY, expires: t0 + 6 * DAY, password: false, limit: 0, downloads: 1, views: 2, uploads: false, uploadLimit: 0, uploaded: 0, notify: false },
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
      setTimeout(() => {
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

  async function archiveStream(req, res) {
    const text = (await raw(req)).toString('utf8');
    const form = /urlencoded/.test(req.headers['content-type'] || '') ? new URLSearchParams(text) : null;
    let body = {};
    if (!form) try { body = JSON.parse(text || '{}'); } catch { /* an empty request asks for nothing */ }
    const paths = form ? form.getAll('paths') : body.paths || [];
    const format = (form ? form.get('format') : body.format) || 'zip';
    if (!paths.length) return [400, 'paths are required'];
    if (missingOf(paths)) return [404, 'Not found'];
    const name = (form ? form.get('name') : body.name) ||`${paths.length === 1 ? base(paths[0]) : base(FS.parent(paths[0])) || 'download'}.${format === 'tgz' ? 'tar.gz' : 'zip'}`;
    res.statusCode = 200;
    res.setHeader('Content-Type', format === 'tgz' ? 'application/gzip' : 'application/zip');
    res.setHeader('Content-Disposition', `attachment; filename*=UTF-8''${encodeURIComponent(name)}`);
    res.end(EMPTY_ZIP);
    return true;
  }
  archiveStream.raw = true;

  const routes = [
    ['POST', /^\/fs\/rename\/batch$/, (_r, _s, b) => renameBatch(b)],
    ['POST', /^\/fs\/link$/, (_r, _s, b) => link(b)],
    ['PATCH', /^\/fs\/attrs$/, (_r, _s, b) => {
      if (b.recursive) {
        const extra = attrsJob(b);
        return Array.isArray(extra) ? extra : [202, view(createJob('attrsRecursive', b, extra))];
      }
      const got = attrsOf(b);
      if (Array.isArray(got)) return got;
      got.nodes.forEach(got.set);
      changed(...b.paths.map(FS.parent));
      return [200, { changed: got.nodes.length }];
    }],
    ['POST', /^\/fs\/archive-stream$/, archiveStream],
    ['POST', /^\/tools\/cleanup\/scan$/, (_r, _s, b) => { const got = cleanup(b.path || ''); return got ? [200, got] : [404, 'Not found']; }],
    ['POST', /^\/tools\/checksum\/parse$/, (_r, _s, b) => {
      if (!b.path) return [200, { lines: parseSums(b.text || '') }];
      const n = FS.nodeAt(b.path);
      return n && !n.dir ? [200, { lines: parseSums(n.text ?? '', n.name) }] : [404, 'Not found'];
    }],
    // Only the totals the property window reads; the treemap's types and largest files stay empty here.
    ['GET', /^\/tools\/diskusage$/, (_r, _s, _b, _m, q) => {
      const path = q.get('path') || '/', root = dirAt(path);
      if (!root) return [404, 'Not found'];
      const totals = (n) => {
        let size = 0, files = 0, dirs = 0, mtime = n.time || 0;
        walk(n, (k) => {
          if (k === n) return;
          if (k.dir) dirs++; else { files++; size += k.size || 0; }
          mtime = Math.max(mtime, k.time || 0);
        });
        return { size: Math.round(size), files, dirs, mtime: iso(mtime) };
      };
      const items = FS.kids(root).map((k) => ({ name: k.name, dir: !!k.dir, ...(k.dir ? totals(k) : { size: Math.round(k.size || 0), files: 1, dirs: 0, mtime: iso(k.time) }) }));
      return [200, { path, ...totals(root), items: items.sort((a, b) => b.size - a.size), types: [], largest: [], indexed: true }];
    }],
    ['GET', /^\/tools\/archives$/, () => [200, { read: ['zip', 'tar', 'tar.gz', 'tar.zst', 'tar.xz', 'tar.bz2', '7z', 'rar'], pack: PACK }]],
    ['GET', /^\/tools\/archive$/, (req, _s, _b, _m, q) => {
      const path = q.get('path') || '', found = archiveAt(path);
      if (!FS.nodeAt(path.split('::')[0])) return [404, 'Not found'];
      if (!found) return [422, 'this is not an archive'];
      if (locked(found.node, req.headers['x-archive-password'])) return [428, 'the archive needs its password'];
      return [200, { path, format: formatOf(found.node.name) || 'zip', entries: entriesOf(found.list), unpacked: Math.round(sizeOf(found.list)), encrypted: !!found.node.locked }];
    }],
    ['POST', /^\/tools\/arrowloop$/, (_r, _s, b) => {
      if (!b.name || !b.left || !b.right || !['right', 'left', 'both'].includes(b.direction) || !['hourly', 'daily', 'weekly'].includes(b.every)) return [400, 'name, left, right, direction and every are required'];
      if (b.at && !/^([01]\d|2[0-3]):[0-5]\d$/.test(b.at)) return [400, 'at is a time such as 03:00'];
      return [201, { job: { id: nextId('al'), ...b, enabled: true }, url: 'https://arrowloop.example.org' }];
    }],

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
  ];
  return { routes, kinds };
}
