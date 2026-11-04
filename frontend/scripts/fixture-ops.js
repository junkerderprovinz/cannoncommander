// Jobs, the check before a transfer, creating and renaming, the trash, undo, uploads, Unraid's gather plan and the
// event stream, played on the prototype's tree. Jobs run on a clock of their own and report over /events as the
// server does, so copying, conflicts, pausing and the queue can be tried without a server.
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { clock } from './fixture-clock.js';
import { toolsFixture } from './fixture-tools.js';
import { sharedFixture } from './fixture.js';
import { unraidFixture } from './fixture-unraid.js';

const KB = 1024, MB = KB * 1024, GB = MB * 1024;
const TICK = 250;
// The fixture's clock runs fast, as the prototype's does: a copy of many gigabytes ends in seconds while the rate and
// the time left read like the server's.
const LAPSE = 20;
const BIG = 2 * GB;
const CHUNK = 512 * KB;
const OPEN = ['queued', 'running', 'paused', 'asking'];

const exported = join(import.meta.dirname, '..', '..', 'testing', 'fixture', 'prototype.json');
const iso = (t) => new Date(t).toISOString();
const base = (p) => p.split('/').filter(Boolean).pop() || '';
const clone = (n) => JSON.parse(JSON.stringify(n));

// How each share picks its disk and how much room it leaves free, as in prototype/src/js/features/u-60-space.js.
const RULES = { filme: { alloc: 'fill', min: 30 * GB }, backups: { alloc: 'most', min: 20 * GB }, isos: { alloc: 'high', min: 10 * GB } };
const ALLOCATOR = { fill: 'fillup', most: 'mostfree', high: 'highwater' };
const ruleOf = (share) => RULES[share] || { alloc: 'high', min: 10 * GB };

// A plugin adds routes and job kinds of its own: it gets the tree and the engine's helpers, and each kind turns the
// params of POST /jobs into what the job works on and what it does to the tree once it is through, or into the status
// and reason it refuses with.
export function opsFixture(fx = sharedFixture(), plugins = []) {
  const { FS, DATA } = fx;
  const unraid = JSON.parse(readFileSync(exported, 'utf8')).unraid;
  let seq = 0, eventId = 0, rolled = 7;
  const nextId = (p) => `${p}${(++seq).toString(36)}`;
  const rnd = () => (rolled = (rolled * 16807) % 2147483647) / 2147483647;

  const clients = new Set();
  function send(type, data, topic) {
    const frame = { id: ++eventId, type, time: iso(clock.now()), ...(topic ? { topic } : {}), ...(data === undefined ? {} : { data }) };
    for (const res of clients) res.write(`id: ${frame.id}\nevent: ${type}\ndata: ${JSON.stringify(frame)}\n\n`);
  }
  const changed = (...paths) => { for (const p of new Set(paths.filter(Boolean))) send('fs.changed', { paths: [p] }, 'fs:' + p); };

  const dirAt = (path) => { const n = FS.nodeAt(path || ''); return n && n.dir ? n : null; };
  const taken = (dir, name) => FS.kids(dir).some((k) => k.name === name);
  const freeName = (dir, name) => {
    if (!taken(dir, name)) return name;
    const dot = name.lastIndexOf('.'), stem = dot > 0 ? name.slice(0, dot) : name, ext = dot > 0 ? name.slice(dot) : '';
    for (let n = 1; ; n++) if (!taken(dir, `${stem} (${n})${ext}`)) return `${stem} (${n})${ext}`;
  };
  const ensureDir = (path) => {
    let n = FS.root;
    for (const part of FS.split(path)) {
      let next = FS.kids(n).find((k) => k.name === part && k.dir);
      if (!next) { next = { name: part, dir: true, children: [], time: clock.now(), owner: 'nobody', group: 'users', mode: '0775' }; n.children.push(next); }
      n = next;
    }
    return n;
  };
  const detach = (path) => {
    const dir = dirAt(FS.parent(path)), n = dir && FS.kids(dir).find((k) => k.name === base(path));
    if (n) dir.children = dir.children.filter((k) => k !== n);
    return n;
  };
  const countOf = (n) => (n.dir ? 1 + FS.kids(n).reduce((a, k) => a + countOf(k), 0) : 1);
  const side = (n, path) => ({ path, dir: !!n.dir, size: Math.round(FS.totalSize(n)), mtime: iso(n.time), ...(n.dir ? { entries: FS.kids(n).length } : {}) });

  // Unraid: which volume and share a path is on, every file below a node, and the disk new files would land on.
  const DISKS = unraid.disks, DEVS = unraid.devs, SHARES = unraid.shares;
  const free = (dev) => FS.FREE['/mnt/' + dev] || 0;
  const where = (path) => {
    const m = path.match(/^\/mnt\/([^/]+)(?:\/([^/]+))?(?:\/(.*))?$/);
    if (!m || !['user', ...DEVS].includes(m[1])) return null;
    return { vol: m[1], share: m[2] || '', rel: [m[2], m[3]].filter(Boolean).join('/') };
  };
  const filesOf = (n, path) => {
    if (!n) return [];
    if (!n.dir) return [[n, path]];
    return FS.kids(n).flatMap((k) => filesOf(k, FS.join(path, k.name)));
  };
  const allowed = (dev, share) => !!SHARES[share] && (SHARES[share].disks.includes(dev) || (dev === 'cache' && SHARES[share].primary === 'cache'));
  function pick(share) {
    const s = SHARES[share], r = ruleOf(share);
    if (!s) return null;
    const disks = s.disks.filter((d) => free(d) > r.min);
    if (!disks.length) return null;
    if (r.alloc === 'fill') return disks[0];
    if (r.alloc === 'most') return disks.reduce((a, d) => (free(d) > free(a) ? d : a));
    let mark = Math.max(...s.disks.map((d) => FS.TOTAL['/mnt/' + d] || 0)) / 2;
    while (mark > r.min) { const d = disks.find((x) => free(x) >= mark); if (d) return d; mark /= 2; }
    return disks[0];
  }
  // Hard links in the user share, by device and inode.
  const links = new Map();
  for (const [n, path] of filesOf(dirAt('/mnt/user'), '/mnt/user')) if (n.ino) links.set(`${n.dev}:${n.ino}`, [...(links.get(`${n.dev}:${n.ino}`) || []), path]);

  function unraidReport(kind, sources, dest) {
    const from = FS.parent(sources[0] || dest), a = where(from), b = where(dest);
    if (!a || !b) return undefined;
    const files = sources.flatMap((p) => filesOf(FS.nodeAt(p), p).map(([n, path]) => [n, path, path.slice(from.length + 1)]));
    const report = { linkable: false, wakes: [] };
    if ((a.vol === 'user') !== (b.vol === 'user')) {
      const same = files.filter(([n, , rel]) => {
        const other = FS.nodeAt(FS.join(dest, rel));
        return other && !other.dir && (a.vol === 'user' ? n.dev === b.vol : other.dev === a.vol);
      });
      if (same.length) {
        const count = {};
        same.forEach(([n]) => { count[n.dev || a.vol] = (count[n.dev || a.vol] || 0) + 1; });
        const device = a.vol === 'user' ? Object.keys(count).sort((x, y) => count[y] - count[x])[0] : a.vol;
        report.trap = {
          device, count: same.length, size: same.reduce((s, [n]) => s + (n.size || 0), 0),
          files: same.map(([n, , rel]) => ({ path: rel, device: n.dev || a.vol, size: n.size || 0 })),
          targets: DISKS.filter((d) => d !== device).sort((x, y) => free(y) - free(x)).map((d) => ({ disk: d, free: Math.round(free(d)) })),
        };
      }
    }
    const broken = files.map(([n, path]) => ({ n, path, others: (links.get(`${n.dev}:${n.ino}`) || []).filter((p) => p !== path) }))
      .filter((x) => x.n.ino && x.others.length && (kind === 'copy' || (b.vol === 'user' ? b.share !== x.n.share : b.vol !== x.n.dev)));
    if (broken.length) {
      report.links = { count: broken.length, extra: broken.reduce((s, x) => s + (x.n.size || 0), 0),
        files: broken.map((x) => ({ path: x.path, size: x.n.size || 0, names: x.others.length + 1, others: x.others, complete: true })) };
    }
    report.linkable = kind === 'copy' && from !== dest && files.length > 0 && files.every(([n]) => (b.vol === 'user' ? b.share && allowed(n.dev, b.share) : b.vol === n.dev));
    const lands = b.vol === 'user' ? (SHARES[b.share]?.primary === 'cache' ? 'cache' : pick(b.share)) : b.vol;
    report.lands = lands || undefined;
    report.wakes = [...new Set([...files.map(([n]) => n.dev), lands])].filter((d) => d && unraid.spin[d] === false);
    const space = spaceCheck(kind, files, a, b);
    if (space) report.space = space;
    return report;
  }

  function spaceCheck(kind, files, src, w) {
    if (w.vol !== 'user' || !w.share || !SHARES[w.share] || !files.length) return null;
    if (kind === 'move' && src.vol === 'user' && src.share === w.share) return null;
    const s = SHARES[w.share], r = ruleOf(w.share);
    const total = files.reduce((a, [n]) => a + (n.size || 0), 0);
    const [big, bigPath] = files.reduce((a, f) => ((f[0].size || 0) > (a[0].size || 0) ? f : a));
    if (s.primary === 'cache' && free('cache') - total > 0) return null;
    const room = s.disks.reduce((a, d) => a + Math.max(0, free(d) - r.min), 0), disk = pick(w.share);
    const roomy = s.disks.filter((d) => free(d) >= big.size).sort((a, b) => free(b) - free(a))[0];
    const out = { share: w.share, allocator: ALLOCATOR[r.alloc], floor: r.min, total, room: Math.round(room), biggest: { path: bigPath, size: big.size }, disk: disk || undefined,
      free: Math.round(disk ? free(disk) : 0), ...(roomy ? { roomy, roomyFree: Math.round(free(roomy)) } : {}) };
    if (!disk) return { problem: 'noDisk', ...out };
    if (total > room) return { problem: 'tooMuch', ...out };
    if (big.size > free(disk)) return { problem: 'bigFile', ...out };
    return null;
  }

  function preflight({ kind, sources = [], dest }) {
    const target = dirAt(dest);
    if (!target) return null;
    const nodes = sources.map((p) => [FS.nodeAt(p), p]).filter(([n]) => n);
    const bytes = nodes.reduce((a, [n]) => a + FS.totalSize(n), 0);
    const conflicts = nodes.filter(([n]) => taken(target, n.name)).map(([n, p]) => {
      const old = FS.kids(target).find((k) => k.name === n.name);
      return { key: n.name, source: side(n, p), target: side(old, FS.join(dest, n.name)), ...(n.time > old.time + 1000 ? { newer: 'source' } : old.time > n.time + 1000 ? { newer: 'target' } : {}) };
    });
    const vol = FS.volumeOf(dest);
    return {
      dest, count: nodes.length, bytes: Math.round(bytes), files: nodes.reduce((a, [n]) => a + countOf(n), 0), conflicts,
      sameDevice: kind === 'move' && nodes.every(([, p]) => FS.volumeOf(p) === vol),
      space: { free: Math.round(FS.FREE[vol] || 0), need: Math.round(bytes), enough: bytes <= (FS.FREE[vol] || Infinity) },
      big: bytes >= BIG, parityBusy: parity.busy, unraid: unraidReport(kind, sources, dest),
    };
  }

  // Undo steps; each knows how to put the tree back.
  const steps = [];
  const pushStep = (step, revert) => {
    const s = { id: nextId('u'), at: iso(clock.now()), count: 1, ...step };
    steps.unshift({ ...s, revert });
    steps.length = Math.min(steps.length, 30);
    send('undo.changed', { count: steps.length });
    return s.id;
  };
  const stepOut = ({ revert, ...s }) => s;

  // The trash, seeded from the prototype's list; an entry restored without its node gets a stand-in.
  const trash = DATA.trash.map((e) => ({ id: nextId('t'), name: e.name, origin: FS.join(e.from, e.name), from: e.from, dir: !!e.dir, size: Math.round(e.size), files: 1, by: e.by,
    deletedAt: iso(e.deleted), expiresAt: iso(e.deleted + 30 * 86400000) }));
  const toTrash = (n, path) => {
    const e = { id: nextId('t'), name: n.name, origin: path, from: FS.parent(path), dir: !!n.dir, size: Math.round(FS.totalSize(n)), files: countOf(n), by: 'jdp',
      deletedAt: iso(clock.now()), expiresAt: iso(clock.now() + 30 * 86400000), node: n };
    trash.unshift(e);
    send('trash.changed', {});
    return e;
  };
  const rebuild = (e) => (e.dir
    ? { name: e.name, dir: true, time: Date.parse(e.deletedAt), owner: 'nobody', group: 'users', mode: '0775', children: [{ name: 'daten.bin', size: e.size, time: Date.parse(e.deletedAt), owner: 'nobody', group: 'users', mode: '0664' }] }
    : { name: e.name, size: e.size, time: Date.parse(e.deletedAt), owner: 'nobody', group: 'users', mode: '0664' });
  function restoreEntries(list) {
    for (const e of list) {
      const dir = ensureDir(e.from), node = e.node || rebuild(e);
      node.name = freeName(dir, node.name);
      dir.children.push(node);
      changed(e.from);
    }
    const ids = new Set(list.map((e) => e.id));
    trash.splice(0, trash.length, ...trash.filter((e) => !ids.has(e.id)));
    send('trash.changed', {});
  }

  // Unraid's gather plan, as in prototype/src/js/features/u-50-gather.js.
  function survey(path) {
    const files = filesOf(dirAt(path), path).map(([n, p]) => ({ n, rel: p.slice(path.length + 1), unit: p.slice(path.length + 1).split('/')[0] }));
    const on = Object.fromEntries(DEVS.map((d) => [d, { size: 0, count: 0 }]));
    files.forEach((f) => { if (on[f.n.dev]) { on[f.n.dev].size += f.n.size || 0; on[f.n.dev].count++; } });
    return { files, on, array: files.filter((f) => f.n.dev && f.n.dev !== 'cache') };
  }
  const gatherMoves = (s, to) => {
    const moves = s.array.filter((f) => f.n.dev !== to).map((f) => ({ ...f, from: f.n.dev, to }));
    const size = moves.reduce((a, m) => a + (m.n.size || 0), 0);
    return { moves, size, short: Math.max(0, size - free(to)) };
  };
  function scatterMoves(s, from) {
    const units = {};
    s.array.filter((f) => f.n.dev === from).forEach((f) => { (units[f.unit] ||= []).push(f); });
    const room = Object.fromEntries(DISKS.filter((d) => d !== from).map((d) => [d, free(d)]));
    const moves = [], short = [];
    Object.values(units).map((list) => ({ list, size: list.reduce((a, f) => a + (f.n.size || 0), 0) })).sort((a, b) => b.size - a.size).forEach((u) => {
      const to = Object.keys(room).sort((a, b) => room[b] - room[a])[0];
      if (!to || room[to] < u.size) { short.push(u); return; }
      room[to] -= u.size;
      u.list.forEach((f) => moves.push({ ...f, from, to }));
    });
    return { moves, size: moves.reduce((a, m) => a + (m.n.size || 0), 0), short: short.reduce((a, u) => a + u.size, 0) };
  }
  function gatherPlan({ path, mode, disk }) {
    if (!/^\/mnt\/user\/[^/]+/.test(path || '') || !dirAt(path)) return null;
    const s = survey(path);
    const used = DISKS.filter((d) => s.on[d].count);
    let options, chosen;
    if (mode === 'gather') {
      const cost = Object.fromEntries(DISKS.map((d) => [d, gatherMoves(s, d)]));
      options = DISKS.map((d) => ({ disk: d, move: Math.round(cost[d].size), short: Math.round(cost[d].short) }));
      chosen = DISKS.includes(disk) ? disk : DISKS.filter((d) => !cost[d].short).sort((a, b) => cost[a].size - cost[b].size)[0] || DISKS[0];
    } else {
      options = used.map((d) => ({ disk: d, move: Math.round(scatterMoves(s, d).size), short: Math.round(scatterMoves(s, d).short) }));
      chosen = used.includes(disk) ? disk : used.sort((a, b) => s.on[b].size - s.on[a].size)[0];
    }
    const plan = chosen ? (mode === 'gather' ? gatherMoves(s, chosen) : scatterMoves(s, chosen)) : { moves: [], size: 0, short: 0 };
    const after = Object.fromEntries(DEVS.map((d) => [d, s.on[d].size]));
    plan.moves.forEach((m) => { after[m.from] -= m.n.size || 0; after[m.to] += m.n.size || 0; });
    return {
      out: {
        path, mode, ...(chosen ? { disk: chosen } : {}), files: s.files.length, size: Math.round(s.files.reduce((a, f) => a + (f.n.size || 0), 0)),
        devices: DEVS.map((d) => ({ device: d, now: Math.round(s.on[d].size), files: s.on[d].count, after: Math.max(0, Math.round(after[d])), free: Math.round(free(d)), asleep: unraid.spin[d] === false })),
        options, moves: plan.moves.length, move: Math.round(plan.size), short: Math.round(plan.short), cached: s.on.cache?.count || 0,
      },
      moves: plan.moves,
    };
  }

  const settings = { mode: 'serial', cap: 0, parityWait: true, notifyUnraid: false, notifyAfter: 60 };
  const parity = { busy: false };
  const jobs = [];
  let timer = null;
  const view = (j) => Object.fromEntries(Object.entries(j).filter(([k]) => !k.startsWith('_')));
  const publish = (type, j) => send(type, view(j));
  const open = () => jobs.filter((j) => OPEN.includes(j.state)).sort((a, b) => a.position - b.position);
  const renumber = () => open().forEach((j, k) => { j.position = k; });

  function createJob(kind, params, extra = {}) {
    const paths = extra.paths ? extra.paths : kind === 'copy' || kind === 'move' ? params.sources : kind === 'delete' || kind === 'trash' ? params.paths : kind === 'gather' || kind === 'scatter' ? [params.path] : extra.paths || [];
    const nodes = (paths || []).map((p) => FS.nodeAt(p)).filter(Boolean);
    const total = extra.total ?? nodes.reduce((a, n) => a + FS.totalSize(n), 0);
    const j = {
      id: nextId('j'), kind, owner: 'jdp', state: 'queued', position: open().length, name: extra.name ?? base(paths?.[0] || ''), count: extra.count ?? paths?.length ?? 0,
      dest: extra.dest ?? params.dest, totalBytes: Math.round(kind === 'trash' || kind === 'delete' || kind === 'restore' || kind === 'emptyTrash' || kind === 'undo' ? 0 : total),
      doneBytes: 0, totalFiles: extra.files ?? nodes.reduce((a, n) => a + countOf(n), 0), doneFiles: 0, rate: 0, left: null, cap: extra.cap || 0,
      waitForParity: !!extra.waitForParity, failedItems: 0, createdAt: iso(clock.now()),
      _params: params, _apply: extra.apply, _names: extra.names, _big: total >= BIG, _speed: extra.speed || 1, _decisions: { ...(params.decisions || {}) }, _rule: params.rule || 'ask',
    };
    if (kind === 'copy' || kind === 'move') j._ask = (params.sources || []).filter((p) => { const n = FS.nodeAt(p), d = dirAt(params.dest); return n && d && taken(d, n.name); }).map((p) => base(p));
    jobs.push(j);
    publish('job.created', j);
    timer ||= clock.setInterval(tick, TICK);
    timer.unref?.();
    // A job that moves no bytes, such as one into the trash, ends at once, as it does on a server for a handful of
    // names; the prototype's own deletes take no time either, so both reach the same point after the same wait.
    if (!j.totalBytes) setImmediate(tick);
    return j;
  }

  // The bytes move LAPSE times faster than the clock, so a finished job tells the time it would have taken a server.
  const endOf = (j) => { const start = Date.parse(j.startedAt || j.createdAt); return iso(start + (clock.now() - start) * LAPSE); };
  const held = (j) => settings.parityWait && parity.busy && ['copy', 'move'].includes(j.kind) && (j.waitForParity || j._big);
  function tick() {
    const live = open().filter((j) => j.state === 'queued' || j.state === 'running');
    const free = live.filter((j) => !held(j));
    const moving = settings.mode === 'serial' ? free.slice(0, 1) : free;
    const share = moving.length > 1 ? 1.4 / moving.length : 1;
    let place = 0;
    for (const j of live) {
      const was = [j.state, j.place, j.reason].join();
      if (moving.includes(j)) j.state = 'running';
      else { j.state = 'queued'; j.rate = 0; j.left = null; j.place = ++place; if (held(j)) j.reason = 'parity'; else delete j.reason; }
      if (j.state === 'running') { delete j.place; delete j.reason; j.startedAt ||= iso(clock.now()); }
      if (was !== [j.state, j.place, j.reason].join()) publish('job.state', j);
    }
    for (const j of moving) advance(j, share, moving.length);
    if (!open().some((j) => j.state === 'queued' || j.state === 'running')) { clock.clear(timer); timer = null; }
  }

  function advance(j, share, n) {
    // A name that exists at the target and has no answer yet makes the job wait for one.
    const want = (j._ask || []).find((name) => !(name in j._decisions) && j._rule === 'ask');
    if (want) {
      const src = j._params.sources.find((p) => base(p) === want), n0 = FS.nodeAt(src), old = FS.kids(dirAt(j.dest)).find((k) => k.name === want);
      if (n0 && old) {
        j.conflict = { key: want, source: side(n0, src), target: side(old, FS.join(j.dest, want)), ...(n0.time > old.time + 1000 ? { newer: 'source' } : old.time > n0.time + 1000 ? { newer: 'target' } : {}) };
        Object.assign(j, { state: 'asking', rate: 0, left: null });
        send('job.conflict', { id: j.id, conflict: j.conflict });
        publish('job.state', j);
        return;
      }
      j._decisions[want] = 'both';
    }
    const caps = [j.cap || Infinity, settings.cap ? settings.cap / n : Infinity];
    if (j.totalBytes) {
      j.rate = Math.min((150 + rnd() * 110) * MB * j._speed * share, ...caps);
      j.doneBytes = Math.min(j.totalBytes, j.doneBytes + (j.rate * TICK / 1000) * LAPSE);
      j.left = (j.totalBytes - j.doneBytes) / j.rate;
      j.doneFiles = Math.min(j.totalFiles, Math.floor((j.doneBytes / j.totalBytes) * j.totalFiles));
      const names = j._names || (j._params.sources || []).map(base);
      j.current = names[Math.min(names.length - 1, Math.floor((j.doneBytes / j.totalBytes) * names.length))];
    } else j.doneFiles = j.totalFiles;
    const finished = j.totalBytes ? j.doneBytes >= j.totalBytes : j.doneFiles >= j.totalFiles;
    send('job.progress', { id: j.id, totalBytes: j.totalBytes, doneBytes: Math.round(j.doneBytes), totalFiles: j.totalFiles, doneFiles: j.doneFiles, rate: j.rate, left: j.left, current: j.current });
    if (!finished) return;
    try {
      const undo = run(j);
      Object.assign(j, { state: 'done', rate: 0, left: null, doneBytes: j.totalBytes, doneFiles: j.totalFiles, finishedAt: endOf(j), ...(undo ? { undo } : {}) });
    } catch (e) {
      Object.assign(j, { state: 'failed', rate: 0, left: null, error: String(e.message || e), failedItems: j.count, finishedAt: endOf(j) });
    }
    delete j.current;
    publish('job.state', j);
    publish('job.done', j);
  }

  // What a finished job does to the tree; it hands back the id of its undo step where it has one.
  function run(j) {
    const p = j._params;
    if (j._apply) return j._apply(j);
    if (j.kind === 'copy' || j.kind === 'move') return transfer(j, p);
    if (j.kind === 'delete' || j.kind === 'trash') {
      const gone = p.paths.map((path) => [detach(path), path]).filter(([n]) => n);
      changed(...p.paths.map(FS.parent));
      if (j.kind === 'delete') return undefined;
      const entries = gone.map(([n, path]) => toTrash(n, path));
      return pushStep({ kind: 'trash', name: entries[0]?.name || '', count: entries.length, path: FS.parent(p.paths[0]) }, () => restoreEntries(entries));
    }
    if (j.kind === 'gather' || j.kind === 'scatter') {
      const plan = gatherPlan({ path: p.path, mode: j.kind, disk: p.disk });
      for (const m of plan?.moves || []) {
        FS.FREE['/mnt/' + m.from] += m.n.size || 0;
        FS.FREE['/mnt/' + m.to] -= m.n.size || 0;
        m.n.dev = m.to;
      }
      changed(p.path);
      return undefined;
    }
    return undefined;
  }

  function transfer(j, p) {
    const target = dirAt(p.dest);
    if (!target) throw new Error('Den Zielordner gibt es nicht mehr');
    const placed = [], replaced = [], moved = [];
    for (const [i, src] of p.sources.entries()) {
      const n = FS.nodeAt(src);
      if (!n) continue;
      const name = p.names?.[i] || n.name;
      const old = FS.kids(target).find((k) => k.name === name);
      let choice = old ? j._decisions[name] || (j._rule === 'ask' ? 'both' : j._rule) : 'new';
      if (choice === 'newer') choice = n.time > old.time ? 'replace' : 'skip';
      if (choice === 'skip') continue;
      const copy = { ...clone(n), name };
      if (p.keepAttrs === false) Object.assign(copy, { owner: 'nobody', group: 'users', time: clock.now() });
      if (choice === 'both') copy.name = freeName(target, name);
      else if (old) { target.children = target.children.filter((k) => k !== old); replaced.push(toTrash(old, FS.join(p.dest, old.name))); }
      target.children.push(copy);
      placed.push(copy);
      if (j.kind === 'move') moved.push([FS.parent(src), detach(src)]);
    }
    changed(p.dest, ...moved.map(([dir]) => dir));
    if (!placed.length) return undefined;
    return pushStep({ kind: j.kind, name: placed[0].name, count: placed.length, path: FS.parent(p.sources[0]), to: p.dest }, () => {
      target.children = target.children.filter((k) => !placed.includes(k));
      restoreEntries(replaced);
      for (const [dir, n] of moved) if (n) ensureDir(dir).children.push(n);
      changed(p.dest, ...moved.map(([dir]) => dir));
    });
  }

  const uploads = new Map();
  function landUpload(u) {
    const dir = ensureDir(FS.parent(u.path));
    let name = base(u.path);
    const old = FS.kids(dir).find((k) => k.name === name);
    let rule = u.rule === 'ask' ? 'both' : u.rule;
    if (old && rule === 'newer') rule = Date.parse(u.mtime || '') > old.time ? 'replace' : 'skip';
    if (old && rule === 'skip') { u.state = 'skipped'; return; }
    if (old && rule === 'both') name = freeName(dir, name);
    else if (old) { dir.children = dir.children.filter((k) => k !== old); toTrash(old, u.path); }
    dir.children.push({ name, size: u.size, time: Date.parse(u.mtime || '') || clock.now(), owner: 'nobody', group: 'users', mode: '0664' });
    u.path = FS.join(FS.parent(u.path), name);
    u.state = 'done';
    changed(FS.parent(u.path));
  }
  const uploadOut = (u) => ({ id: u.id, path: u.path, size: u.size, offset: u.offset, state: u.state, rule: u.rule, mtime: u.mtime, chunk: CHUNK });

  const json = (res, status, body, headers = {}) => {
    res.statusCode = status;
    for (const [k, v] of Object.entries(headers)) res.setHeader(k, v);
    if (status === 204) return res.end();
    res.setHeader('Content-Type', status >= 400 ? 'application/problem+json' : 'application/json');
    res.end(JSON.stringify(status >= 400 ? { type: 'about:blank', title: body.title ?? body, status, ...(typeof body === 'object' ? body : {}) } : body));
  };
  const raw = (req) => new Promise((resolve) => { const parts = []; req.on('data', (c) => parts.push(c)); req.on('end', () => resolve(Buffer.concat(parts))); });
  const bodyOf = async (req) => { try { const b = (await raw(req)).toString('utf8'); return b ? JSON.parse(b) : {}; } catch { return {}; } };
  const jobAt = (id) => jobs.find((j) => j.id === id);
  const resume = () => { timer ||= clock.setInterval(tick, TICK); timer.unref?.(); tick(); };
  const unraidRoutes = unraidFixture({ fx, unraid, parity, send, createJob, view, filesOf, where, ruleOf, allocatorOf: (a) => ALLOCATOR[a], resume });

  const kinds = {};
  function startJob(kind, params = {}, extra = {}) {
    const own = kinds[kind](params);
    return Array.isArray(own) ? own : [202, view(createJob(kind, params, { ...extra, ...own }))];
  }

  // Only the fixture has this: a download client that writes into downloads every four seconds, a part file that
  // grows and then gets its real name, as the prototype plays it.
  const FILMS = ['Cosmos Laundromat (2015)', 'Spring (2019)', 'Agent 327 (2017)'];
  let client = null, step = 0;
  function download() {
    const dir = '/mnt/user/downloads', n = dirAt(dir), film = FILMS[Math.floor(step / 3) % FILMS.length], phase = step % 3;
    const part = `${film}.mkv.part`, done = `${film}.mkv`, k = FS.kids(n).find((x) => x.name === part);
    if (phase === 0) n.children.push({ name: part, size: 300 * MB, time: clock.now(), owner: 'nobody', group: 'users', mode: '0664' });
    if (phase === 1 && k) Object.assign(k, { size: 900 * MB, time: clock.now() });
    if (phase === 2 && k) Object.assign(k, { name: FS.kids(n).some((x) => x.name === done) ? freeName(n, done) : done, size: 1.6 * GB, time: clock.now() });
    step += 1;
    changed(dir);
  }

  // A route that answers false only looks on, and the next one for the same request answers it.
  const routes = [
    ...unraidRoutes,
    ['PUT', /^\/fixture\/download$/, (_r, _s, b) => {
      clock.clear(client);
      client = b.run ? clock.setInterval(download, 4000) : null;
      if (b.run) download();
      return [204];
    }],
    ['GET', /^\/events$/, (req, res) => {
      res.writeHead(200, { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache', Connection: 'keep-alive' });
      res.write(': fixture\n\n');
      clients.add(res);
      const beat = setInterval(() => res.write(`event: heartbeat\ndata: {"type":"heartbeat"}\n\n`), 15000);
      req.on('close', () => { clearInterval(beat); clients.delete(res); });
      return true;
    }],
    ['GET', /^\/jobs$/, () => [200, { jobs: [...open(), ...jobs.filter((j) => !OPEN.includes(j.state)).reverse()].map(view) }]],
    ['POST', /^\/jobs$/, (_r, _s, b) => {
      if (['gather', 'scatter'].includes(b.kind) && !gatherPlan({ ...b.params, mode: b.kind })) return [400, 'pick a folder in a user share'];
      if (kinds[b.kind]) return startJob(b.kind, b.params || {}, { cap: b.cap, waitForParity: b.waitForParity });
      return [202, view(createJob(b.kind, b.params || {}, { cap: b.cap, waitForParity: b.waitForParity, ...(b.kind === 'gather' || b.kind === 'scatter' ? { dest: '/mnt/' + b.params.disk, total: gatherPlan({ ...b.params, mode: b.kind }).out.move } : {}) }))];
    }],
    ['DELETE', /^\/jobs$/, () => {
      const ended = jobs.filter((j) => !OPEN.includes(j.state));
      jobs.splice(0, jobs.length, ...jobs.filter((j) => OPEN.includes(j.state)));
      return [200, { removed: ended.length }];
    }],
    ['GET', /^\/jobs\/settings$/, () => [200, settings]],
    ['GET', /^\/jobs\/([^/]+)$/, (_r, _s, _b, [, id]) => { const j = jobAt(id); return j ? [200, view(j)] : [404, 'Not found']; }],
    ['PUT', /^\/jobs\/settings$/, (_r, _s, b) => { Object.assign(settings, b); timer ||= clock.setInterval(tick, TICK); return [200, settings]; }],
    ['PATCH', /^\/jobs\/([^/]+)$/, (_r, _s, b, [, id]) => {
      const j = jobAt(id);
      if (!j) return [404, 'Not found'];
      if (b.cap != null) j.cap = b.cap;
      if (b.position != null && OPEN.includes(j.state)) {
        const list = open().filter((x) => x !== j);
        list.splice(Math.max(0, Math.min(list.length, b.position)), 0, j);
        list.forEach((x, k) => { if (x.position !== k) { x.position = k; if (x !== j) publish('job.state', x); } });
      }
      tick();
      publish('job.state', j);
      return [200, view(j)];
    }],
    ['DELETE', /^\/jobs\/([^/]+)$/, (_r, _s, _b, [, id]) => {
      const j = jobAt(id);
      if (!j) return [404, 'Not found'];
      if (OPEN.includes(j.state)) return [409, 'The job is still open'];
      jobs.splice(jobs.indexOf(j), 1);
      return [204];
    }],
    ['POST', /^\/jobs\/([^/]+)\/(pause|resume|cancel|retry|decide)$/, (_r, _s, b, [, id, step]) => {
      const j = jobAt(id);
      if (!j) return [404, 'Not found'];
      if (step === 'pause' && ['queued', 'running'].includes(j.state)) Object.assign(j, { state: 'paused', rate: 0, left: null });
      else if (step === 'resume' && j.state === 'paused') j.state = 'queued';
      else if (step === 'cancel' && OPEN.includes(j.state)) Object.assign(j, { state: 'cancelled', rate: 0, left: null, finishedAt: iso(clock.now()) });
      else if (step === 'retry' && ['failed', 'cancelled'].includes(j.state)) { Object.assign(j, { state: 'queued', doneBytes: 0, doneFiles: 0, failedItems: 0, position: open().length }); delete j.error; delete j.finishedAt; }
      else if (step === 'decide' && j.state === 'asking') {
        const key = b.key || j.conflict?.key;
        j._decisions[key] = b.choice;
        if (b.all) j._rule = b.choice;
        delete j.conflict;
        j.state = 'running';
      } else return [409, 'That does not fit the state of the job'];
      renumber();
      publish('job.state', j);
      if (j.state === 'cancelled') publish('job.done', j);
      timer ||= clock.setInterval(tick, TICK);
      tick();
      return [200, view(j)];
    }],

    ['POST', /^\/ops\/preflight$/, (_r, _s, b) => { const pf = preflight(b); return pf ? [200, pf] : [404, 'Not found']; }],
    ['POST', /^\/fs\/mkdir$/, (_r, _s, { path }) => {
      const dir = dirAt(FS.parent(path || ''));
      if (!dir) return [404, 'Not found'];
      if (taken(dir, base(path))) return [409, 'This name exists already'];
      const n = { name: base(path), dir: true, children: [], time: clock.now(), owner: 'nobody', group: 'users', mode: '0775' };
      dir.children.push(n);
      changed(FS.parent(path));
      const undo = pushStep({ kind: 'mkdir', name: n.name, path: FS.parent(path) }, () => { dir.children = dir.children.filter((k) => k !== n); changed(FS.parent(path)); });
      return [201, { path, created: true, undo }];
    }],
    ['POST', /^\/fs\/touch$/, (_r, _s, { path, mtime }) => {
      const dir = dirAt(FS.parent(path || ''));
      if (!dir) return [404, 'Not found'];
      const old = FS.kids(dir).find((k) => k.name === base(path));
      if (old) { old.time = mtime ? Date.parse(mtime) : clock.now(); changed(FS.parent(path)); return [200, { path }]; }
      const n = { name: base(path), size: 0, time: clock.now(), owner: 'nobody', group: 'users', mode: '0664', text: '' };
      dir.children.push(n);
      changed(FS.parent(path));
      const undo = pushStep({ kind: 'touch', name: n.name, path: FS.parent(path) }, () => { dir.children = dir.children.filter((k) => k !== n); changed(FS.parent(path)); });
      return [201, { path, created: true, undo }];
    }],
    ['POST', /^\/fs\/rename$/, (_r, _s, { path, name }) => {
      const n = FS.nodeAt(path || ''), dir = dirAt(FS.parent(path || ''));
      if (!n || !dir) return [404, 'Not found'];
      if (!name || name.includes('/')) return [400, 'A name without a folder'];
      if (taken(dir, name)) return [409, 'This name exists already'];
      const was = n.name;
      n.name = name;
      changed(FS.parent(path));
      const undo = pushStep({ kind: 'rename', name: was, to: name, path: FS.parent(path) }, () => { n.name = was; changed(FS.parent(path)); });
      return [200, { path: FS.join(FS.parent(path), name), undo }];
    }],

    ['GET', /^\/trash$/, () => [200, { items: trash.map(({ node, ...e }) => e), count: trash.length, bytes: trash.reduce((a, e) => a + e.size, 0), days: 30 }]],
    ['POST', /^\/trash\/restore$/, (_r, _s, { ids = [] }) => {
      const list = trash.filter((e) => ids.includes(e.id));
      if (!list.length) return [404, 'Not found'];
      return [202, view(createJob('restore', {}, { name: list[0].name, count: list.length, files: list.length, dest: list[0].from, apply: () => { restoreEntries(list); } }))];
    }],
    ['POST', /^\/trash\/empty$/, (_r, _s, { ids }) => {
      const list = ids ? trash.filter((e) => ids.includes(e.id)) : [...trash];
      return [202, view(createJob('emptyTrash', {}, { name: list[0]?.name || '', count: list.length, files: Math.max(1, list.length), apply: () => {
        const gone = new Set(list.map((e) => e.id));
        trash.splice(0, trash.length, ...trash.filter((e) => !gone.has(e.id)));
        send('trash.changed', {});
      } }))];
    }],
    ['DELETE', /^\/trash\/([^/]+)$/, (_r, _s, _b, [, id]) => {
      const e = trash.find((x) => x.id === id);
      if (!e) return [404, 'Not found'];
      return [202, view(createJob('emptyTrash', {}, { name: e.name, count: 1, files: 1, apply: () => { trash.splice(trash.indexOf(e), 1); send('trash.changed', {}); } }))];
    }],
    ['GET', /^\/trash\/settings$/, () => [200, { days: 30 }]],

    ['GET', /^\/undo$/, () => [200, { steps: steps.map(stepOut) }]],
    ['POST', /^\/undo$/, (_r, _s, { id }) => {
      const step = id ? steps.find((s) => s.id === id) : steps[0];
      if (!step) return [404, 'Nothing to take back'];
      steps.splice(steps.indexOf(step), 1);
      send('undo.changed', { count: steps.length });
      if (['rename', 'renameBatch', 'mkdir', 'touch', 'link'].includes(step.kind)) { step.revert(); return [200, { step: stepOut(step) }]; }
      const job = createJob('undo', {}, { name: step.name, count: step.count, files: step.count, dest: step.path, apply: () => { step.revert(); } });
      return [202, { step: stepOut(step), job: view(job) }];
    }],

    ['POST', /^\/uploads$/, (_r, _s, { path, size = 0, mtime, rule = 'ask' }) => {
      const dir = FS.parent(path || '');
      if (!path || FS.inArchive?.(path)) return [400, 'Not a target'];
      const running = [...uploads.values()].find((u) => u.path === path && u.size === size && u.state === 'open');
      if (running) { running.rule = rule === 'ask' ? running.rule : rule; return [200, uploadOut(running)]; }
      const there = dirAt(dir) && FS.kids(dirAt(dir)).find((k) => k.name === base(path));
      if (there && rule === 'ask') {
        return [409, { title: 'The name exists already', conflict: { key: base(path), source: { path, dir: false, size, mtime: mtime || iso(clock.now()) }, target: side(there, path) } }];
      }
      const u = { id: nextId('up'), path, size, offset: 0, state: 'open', rule, mtime };
      uploads.set(u.id, u);
      if (!size) landUpload(u);
      return [201, uploadOut(u)];
    }],
    ['GET', /^\/uploads\/([^/]+)$/, (_r, _s, _b, [, id]) => {
      const u = uploads.get(id);
      return u ? [200, uploadOut(u), { 'Upload-Offset': String(u.offset) }] : [404, 'Not found'];
    }],
    ['PUT', /^\/uploads\/([^/]+)$/, async (req, _s, _b, [, id]) => {
      const u = uploads.get(id);
      const bytes = await raw(req);
      if (!u) return [404, 'Not found'];
      const at = Number(req.headers['upload-offset'] ?? -1);
      if (at !== u.offset) return [409, { title: 'The chunk does not start where the upload stands', upload: uploadOut(u) }];
      if (u.offset + bytes.length > u.size) return [413, 'The chunk goes past the announced size'];
      u.offset += bytes.length;
      // A pause per chunk, so the progress can be watched even on a fast machine.
      await new Promise((r) => setTimeout(r, 120));
      if (u.offset >= u.size) landUpload(u);
      return [200, uploadOut(u), { 'Upload-Offset': String(u.offset) }];
    }],
    ['DELETE', /^\/uploads\/([^/]+)$/, (_r, _s, _b, [, id]) => (uploads.delete(id) ? [204] : [404, 'Not found'])],

    ['POST', /^\/unraid\/gather\/plan$/, (_r, _s, b) => { const plan = gatherPlan(b); return plan ? [200, plan.out] : [400, 'pick a folder in a user share']; }],
  ];

  const engine = { FS, DATA, fx, send, changed, createJob, startJob, view, dirAt, taken, freeName, ensureDir, detach, pushStep, toTrash, countOf, nextId, iso, base, clone, raw };
  for (const plugin of plugins) {
    const got = plugin(engine);
    routes.unshift(...(got.routes || []));
    Object.assign(kinds, got.kinds);
  }

  return async (req, res, next) => {
    const url = new URL(req.url, 'http://fixture');
    if (!url.pathname.startsWith('/api/v1/')) return next();
    const path = url.pathname.slice('/api/v1'.length);
    let body;
    for (const [method, re, fn] of routes) {
      const m = req.method === method && path.match(re);
      if (!m) continue;
      if (!(method === 'GET' || method === 'DELETE' || (method === 'PUT' && path.startsWith('/uploads/')) || fn.raw)) body ??= await bodyOf(req);
      const out = await fn(req, res, body || {}, m, url.searchParams);
      if (out === false) continue;
      if (out === true) return;
      const [status, payload, headers] = out;
      return json(res, status, payload, headers);
    }
    return next();
  };
}

export const opsFixtureMiddleware = () => opsFixture(sharedFixture(), [toolsFixture]);
