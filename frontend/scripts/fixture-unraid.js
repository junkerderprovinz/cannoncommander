// Unraid as the fixture plays it: the server state with its disks and shares, where a file lies, sleeping disks, a
// parity check that runs for a while and the rights repair as a job. A disk wakes when something reads from it and
// falls asleep again a while later, as it does on the server.
import { clock } from './fixture-clock.js';

const KB = 1024, MB = KB * 1024, GB = MB * 1024, TB = GB * 1024;
const iso = (t) => new Date(t).toISOString();

const PARITY_SIZE = 4 * TB;
// The parity check moves on two percent a second, so it ends within a minute and a half.
const PARITY_STEP = 0.02;
const PARITY_EVERY = 1000;
// How long a disk takes to spin up, and how long it stays awake after the last read.
const SPIN_UP = 1000;
const SLEEP_AFTER = 3 * 60_000;
const MOVER = '40 3 * * *';

const KEPT = { appdata: 'appdata', system: 'system', domains: 'domains' };
const READS = /^\/fs\/(raw|text|hex|page)$/;

export function unraidFixture({ fx, unraid, parity, send, createJob, view, filesOf, where, ruleOf, allocatorOf, resume }) {
  const { FS } = fx;
  const DISKS = unraid.disks, DEVS = unraid.devs, SHARES = unraid.shares, spin = unraid.spin;
  const free = (dev) => Math.round(FS.FREE['/mnt/' + dev] || 0);
  const total = (dev) => Math.round(FS.TOTAL['/mnt/' + dev] || 0);
  const asleep = (dev) => spin[dev] === false;
  // The timers never keep a test run or the dev server from ending.
  const later = (ms, fn) => {
    const t = clock.setTimeout(fn, ms);
    t.unref?.();
    return t;
  };

  function setSpin(devs, awake) {
    const changed = devs.filter((d) => d in spin && spin[d] !== awake);
    changed.forEach((d) => { spin[d] = awake; });
    if (changed.length) send('unraid.spin', { asleep: DISKS.filter(asleep), changed });
  }
  const naps = {}, ups = {};
  function wake(devs) {
    for (const d of new Set(devs.filter((x) => x in spin))) {
      clock.clear(naps[d]);
      naps[d] = later(SLEEP_AFTER, () => sleep([d]));
      if (asleep(d) && !ups[d]) ups[d] = later(SPIN_UP, () => { delete ups[d]; setSpin([d], true); });
    }
  }
  function sleep(devs) {
    for (const d of devs) {
      clock.clear(naps[d]);
      clock.clear(ups[d]);
      delete ups[d];
    }
    setSpin(devs, false);
  }
  // The disks a path lies on: its own volume on a disk share, the disks of its files in a user share.
  function devicesOf(path) {
    const w = where(path), n = FS.nodeAt(path);
    if (!w || !n) return [];
    if (w.vol !== 'user') return [w.vol];
    if (!w.share) return [];
    const found = new Set(filesOf(n, path).map(([k]) => k.dev).filter(Boolean));
    if (n.dir) for (const d of DEVS) if (FS.nodeAt(`/mnt/${d}/${w.rel}`)) found.add(d);
    return DEVS.filter((d) => found.has(d));
  }

  const store = (s) => (s === 'cache' ? { type: 'pool', name: 'cache' } : { type: 'array' });
  function shareOut(name) {
    const s = SHARES[name], r = ruleOf(name), used = Math.round(FS.totalSize(FS.nodeAt('/mnt/user/' + name) || { size: 0 }));
    const devs = [...(s.primary === 'cache' ? ['cache'] : []), ...(s.secondary === 'array' || s.primary === 'array' ? s.disks : [])];
    return {
      name, primary: store(s.primary), ...(s.secondary ? { secondary: store(s.secondary) } : {}),
      ...(s.mover ? { mover: s.mover === 'up' ? 'toPrimary' : 'toSecondary' } : {}),
      allocator: allocatorOf(r.alloc), splitLevel: '', floor: r.min, disks: s.disks,
      free: devs.reduce((a, d) => a + free(d), 0), used, exclusive: s.primary === 'cache' && !s.secondary,
    };
  }
  function diskOut(name, kind) {
    const sleeping = kind === 'data' && asleep(name);
    return {
      name, kind, mount: '/mnt/' + name, mounted: true, status: 'DISK_OK', fsType: kind === 'pool' ? 'btrfs' : 'xfs',
      size: total(name), fsSize: total(name), free: free(name), used: total(name) - free(name), spundown: sleeping,
      rotational: kind === 'data', ...(sleeping ? {} : { temp: kind === 'pool' ? 41 : 34 }), errors: 0, color: sleeping ? 'green-blink' : 'green-on',
    };
  }
  const state = () => ({
    available: true, version: '7.3.4', name: 'Bottich',
    array: { state: 'STARTED', started: true, disks: DISKS.length + 1, disabled: 0, invalid: 0, missing: 0, fsState: 'Started', unmountable: 0 },
    parity: { ...check },
    disks: [
      { name: 'parity', kind: 'parity', mounted: false, status: 'DISK_OK', size: PARITY_SIZE, fsSize: 0, free: 0, used: 0, spundown: false, rotational: true, errors: 0 },
      ...DISKS.map((d) => diskOut(d, 'data')),
      ...DEVS.filter((d) => !DISKS.includes(d)).map((d) => diskOut(d, 'pool')),
    ],
    shares: Object.keys(SHARES).sort().map(shareOut),
    write: true, spindownDelay: 45, mover: { schedule: MOVER, active: false },
  });

  // The parity check as /proc/mdstat tells it, played on a timer of its own.
  const check = { running: false, paused: false, pos: 0, size: PARITY_SIZE, percent: 0, lastSync: iso(clock.now() - 23 * 86400000), lastErrors: 0, lastExit: 0 };
  let ticking = null;
  function startCheck() {
    if (check.running) return;
    Object.assign(check, { running: true, kind: 'check', action: 'check P', correct: false, pos: 0, percent: 0, speed: 180 * MB });
    parity.busy = true;
    ticking = clock.setInterval(() => {
      check.pos = Math.min(PARITY_SIZE, check.pos + PARITY_STEP * PARITY_SIZE);
      check.percent = Math.round((check.pos / PARITY_SIZE) * 1000) / 10;
      if (check.pos >= PARITY_SIZE) stopCheck(true);
      else send('unraid.parity', { ...check });
    }, PARITY_EVERY);
    ticking.unref?.();
    send('unraid.parity', { ...check });
    resume();
  }
  function stopCheck(finished) {
    if (!check.running) return;
    clock.clear(ticking);
    ticking = null;
    for (const k of ['kind', 'action', 'correct', 'speed']) delete check[k];
    Object.assign(check, { running: false, pos: 0, percent: 0, lastExit: finished ? 0 : -4, ...(finished ? { lastSync: iso(clock.now()) } : {}) });
    parity.busy = false;
    send('unraid.parity', { ...check });
    resume();
  }

  function whereOut(path) {
    const n = FS.nodeAt(path), w = where(path);
    if (!n) return null;
    const out = { path, volume: w?.vol || '', share: w?.share || '', dir: !!n.dir, on: [], files: 0, size: 0 };
    if (!w || !w.share) return out;
    const list = filesOf(n, path), on = {};
    for (const [k] of list) {
      const dev = w.vol === 'user' ? k.dev : w.vol;
      if (!dev) continue;
      (on[dev] ||= { device: dev, files: 0, size: 0, asleep: asleep(dev) }).files++;
      on[dev].size += k.size || 0;
    }
    Object.assign(out, { on: DEVS.filter((d) => on[d]).map((d) => ({ ...on[d], size: Math.round(on[d].size) })), files: list.length, size: Math.round(list.reduce((a, [k]) => a + (k.size || 0), 0)) });
    if (!n.dir && n.ino) {
      const paths = filesOf(FS.nodeAt('/mnt/user'), '/mnt/user').filter(([k, p]) => k.ino === n.ino && k.dev === n.dev && p !== path).map(([, p]) => p);
      out.links = { count: paths.length + 1, paths, complete: true };
    }
    return out;
  }

  // Unraid's New Permissions: nobody:users, folders 0777 and files 0666, appdata, system, the VM folder and the stick
  // left alone.
  const keptWhy = (path) => {
    const p = FS.split(path);
    if (p[0] === 'boot') return 'flash';
    return p[0] === 'mnt' && p.length >= 3 ? KEPT[p[2]] || '' : '';
  };
  const isFolder = (k) => k.dir && !k.link;
  function walkPerms(path, visit) {
    const kept = [];
    const walk = (node, at) => {
      for (const k of FS.kids(node)) {
        const q = FS.join(at, k.name), why = keptWhy(q);
        if (why) { if (isFolder(k)) kept.push({ path: q, reason: why, ...(FS.split(q).length === 3 ? { size: Math.round(FS.totalSize(k)) } : {}) }); continue; }
        visit(k);
        if (isFolder(k)) walk(k, q);
      }
    };
    walk(FS.nodeAt(path), path);
    return kept;
  }
  const wrong = (k) => k.owner !== 'nobody' || k.group !== 'users' || k.mode !== (isFolder(k) ? '0777' : '0666');
  function permsScan(path) {
    const n = FS.nodeAt(path);
    if (!n) return null;
    const out = { path, owner: '99:100', files: 0, dirs: 0, wrong: 0, kept: [] };
    if (path.includes('::')) return { ...out, blocked: 'archive' };
    if (keptWhy(path)) return { ...out, blocked: keptWhy(path) };
    if (wrong(n)) out.wrong++;
    out.kept = walkPerms(path, (k) => { if (isFolder(k)) out.dirs++; else out.files++; if (wrong(k)) out.wrong++; });
    return out;
  }
  function fixPerms(path) {
    const scan = permsScan(path);
    if (!scan || scan.blocked) return [400, scan ? `${path} keeps its own owners` : 'Not found'];
    const set = (k) => Object.assign(k, { owner: 'nobody', group: 'users', mode: isFolder(k) ? '0777' : '0666' });
    const j = createJob('fixPerms', { path }, { name: FS.split(path).pop() || '', count: 1, files: scan.files + scan.dirs + 1, total: 0 });
    j._apply = () => {
      let changed = 0, entries = 0;
      const touch = (k) => { entries++; if (wrong(k)) changed++; set(k); };
      touch(FS.nodeAt(path));
      const kept = walkPerms(path, touch);
      j.result = { entries, changed, failed: 0, kept };
      send('fs.changed', { paths: [path] }, 'fs:' + path);
      return undefined;
    };
    return [202, view(j)];
  }

  // Paths of a job that reach a disk: the files it reads and the folder it writes to.
  function jobDisks({ kind, params = {} }) {
    const paths = kind === 'copy' || kind === 'move' ? [...(params.sources || []), params.dest] : kind === 'gather' || kind === 'scatter' ? [params.path, '/mnt/' + params.disk] : [];
    return paths.filter(Boolean).flatMap(devicesOf);
  }

  const routes = [
    ['GET', /^\/unraid\/state$/, () => [200, state()]],
    ['GET', /^\/unraid\/where$/, (req) => {
      const path = new URL(req.url, 'http://fixture').searchParams.get('path') || '';
      const out = whereOut(path);
      return out ? [200, out] : [404, 'Not found'];
    }],
    ['POST', /^\/unraid\/spindown$/, (_r, _s, { disks }) => {
      const want = disks?.length ? disks : DISKS;
      const bad = want.find((d) => !DISKS.includes(d));
      if (bad) return [400, `unknown disk ${bad}`];
      sleep(want);
      return [200, { disks: want }];
    }],
    ['POST', /^\/unraid\/fixperms\/scan$/, (_r, _s, { path }) => {
      if (!path?.startsWith('/mnt/') && !path?.startsWith('/boot')) return [400, 'rights are only set below /mnt'];
      const out = permsScan(path);
      return out ? [200, out] : [404, 'Not found'];
    }],
    ['POST', /^\/unraid\/notify\/test$/, () => [204]],
    ['POST', /^\/fs\/meta$/, (_r, _s, body) => {
      const got = fx.meta(body);
      if ((body.columns || []).includes('disk')) for (const [p, values] of Object.entries(got.items)) values.disk = devicesOf(p).join(', ') || null;
      return [200, got];
    }],
    ['POST', /^\/jobs$/, (_r, _s, b) => {
      if (b.kind === 'fixPerms') return fixPerms(b.params?.path || '');
      wake(jobDisks(b));
      return false;
    }],
    ['GET', READS, (req) => {
      wake(devicesOf(new URL(req.url, 'http://fixture').searchParams.get('path') || ''));
      return false;
    }],
    // Only the fixture has these: a parity check to start and stop, and a disk to send to sleep or wake by hand.
    ['PUT', /^\/fixture\/parity$/, (_r, _s, { busy, finished }) => {
      if (busy) startCheck();
      else stopCheck(!!finished);
      return [200, { ...check, busy: parity.busy }];
    }],
    ['PUT', /^\/fixture\/spin$/, (_r, _s, { disk, asleep: down }) => {
      if (!DISKS.includes(disk)) return [400, `unknown disk ${disk}`];
      if (down) sleep([disk]);
      else setSpin([disk], true);
      return [200, { disk, asleep: asleep(disk) }];
    }],
  ];

  return routes;
}
