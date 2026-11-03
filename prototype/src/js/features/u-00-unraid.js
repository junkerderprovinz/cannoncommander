// Unraid underneath /mnt/user: which disk or pool holds each file, how each share is set up and which disks sleep.
// The user shares stay the tree everyone works on; /mnt/cache and /mnt/diskN hold the very same file nodes, so a change
// through one path shows through the other, as it does on the server.
(() => {
  const bk = CC.bk;
  const mnt = FS.nodeAt('/mnt');
  mnt.children.splice(mnt.children.findIndex(k => k.name === 'disk2') + 1, 0, bk.dirNode('disk3', [], { time: NOW - 90 * DAY }));
  FS.FREE['/mnt/disk3'] = 1.1 * TB;
  FS.TOTAL['/mnt/disk3'] = 4 * TB;
  FS.LABEL['/mnt/disk3'] = 'Disk 3';

  const DISKS = ['disk1', 'disk2', 'disk3'];
  const DEVS = ['cache', ...DISKS];
  // mover: 'down' is Cache → Array, 'up' is Array → Cache, as the share settings in Unraid name them.
  const SHARES = {
    appdata: { primary: 'cache', secondary: 'array', mover: 'up', disks: ['disk1'] },
    system: { primary: 'cache', disks: [] },
    downloads: { primary: 'cache', secondary: 'array', mover: 'down', disks: ['disk1'] },
    filme: { primary: 'array', disks: DISKS },
    fotos: { primary: 'cache', secondary: 'array', mover: 'down', disks: ['disk2', 'disk3'] },
    musik: { primary: 'cache', secondary: 'array', mover: 'down', disks: ['disk2'] },
    backups: { primary: 'array', disks: DISKS },
    isos: { primary: 'array', disks: ['disk2', 'disk3'] },
    dokumente: { primary: 'cache', secondary: 'array', mover: 'down', disks: ['disk3'] },
  };
  const shareOf = (name) => SHARES[name] || { primary: 'array', disks: DISKS };
  const hasArray = (s) => s.primary === 'array' || s.secondary === 'array';
  const devName = (dev) => FS.LABEL['/mnt/' + dev] || dev;
  const names = (devs) => devs.map(devName).join(devs.length === 2 ? ' und ' : ', ');

  // Where the made-up files sit when the page loads, so films, hard links and a split folder show up where they should.
  const START = {
    'downloads/Big.Buck.Bunny.2008.1080p.mkv': 'disk1', 'downloads/ubuntu-24.04.3-desktop-amd64.iso': 'disk1', 'downloads/album.flac': 'cache',
    'downloads/Rechnung-Oktober.pdf': 'cache', 'downloads/setup-notes.md': 'cache', 'downloads/unfertig': 'cache', 'downloads/.partial-download': 'cache',
    'filme/Big Buck Bunny (2008)': 'disk1', 'filme/Sintel (2010)': 'disk2', 'filme/Elephants Dream (2006)': 'disk2',
    'filme/Tears of Steel (2012)/Tears of Steel (2012).mkv': 'disk3', 'filme/Tears of Steel (2012)/poster.jpg': 'disk1', 'musik/album.flac': 'cache',
  };
  let booting = true;
  function allocate(rel, node) {
    const start = booting && Object.keys(START).find(p => rel === p || rel.startsWith(p + '/'));
    if (start) return START[start];
    const s = shareOf(FS.split(rel)[0]);
    if (s.primary === 'cache' && (!s.disks.length || !s.secondary || s.mover === 'up' || node.time > NOW - 10 * DAY)) return 'cache';
    return s.disks[Math.floor(bk.rng(rel)() * s.disks.length)];
  }
  const home = (rel) => { const s = shareOf(FS.split(rel)[0]); return s.primary === 'cache' ? 'cache' : s.disks[0] || 'disk1'; };

  // { vol: 'user' | 'cache' | 'diskN', rel, share } for a path on the array, null anywhere else.
  function where(path) {
    if (!path || FS.inArchive(path)) return null;
    const p = FS.split(path);
    if (p[0] !== 'mnt' || !(p[1] === 'user' || DEVS.includes(p[1]))) return null;
    return { vol: p[1], rel: p.slice(2).join('/'), share: p[2] || '' };
  }
  const isFile = (n) => !n.dir || !!n.link;
  // Every file in a node with its path relative to the node's folder, so a folder and a file go through the same loop.
  function filesOf(node, name = node.name) {
    if (isFile(node)) return [[node, name]];
    const out = [];
    FS.walk((k, p) => { if (isFile(k)) out.push([k, name + p]); }, node);
    return out;
  }
  const devOf = (node, path) => { const w = where(path); return w && w.vol !== 'user' ? w.vol : node.dev || null; };
  function linksOf(node) {
    if (!node.ino) return [];
    const out = [];
    FS.walk((k, p) => { if (k !== node && k.ino === node.ino) out.push(p); }, FS.nodeAt('/mnt/user'), '/mnt/user');
    return out;
  }
  // A hard link stays on the disk of its source, so it only works where the target can live on that disk too.
  const allowed = (dev, share) => { const s = shareOf(share); return dev === 'cache' ? s.primary === 'cache' : hasArray(s) && s.disks.includes(dev); };
  // The disk a file ends up on: a move inside one share keeps its disk, anything else follows the target share's rules.
  function landsOn(dest, kind, node) {
    const w = where(dest);
    if (!w) return null;
    if (w.vol !== 'user') return w.vol;
    if (!w.share) return null;
    if (kind === 'move' && node.share === w.share && node.dev) return node.dev;
    return allocate(w.rel ? `${w.rel}/${node.name}` : node.name, { time: NOW });
  }
  const onCache = (node) => filesOf(node).some(([k]) => k.dev === 'cache');

  // Disks that sleep; the cache is an SSD and never does.
  const spin = { disk1: false, disk2: true, disk3: false };
  function wake(devs) {
    const sleepy = [...new Set(devs)].filter(d => spin[d] === false);
    if (!sleepy.length) return;
    sleepy.forEach(d => { spin[d] = true; });
    const one = sleepy.length === 1;
    CC.toast(`${names(sleepy)} ${one ? 'schläft und fährt' : 'schlafen und fahren'} jetzt hoch, das dauert ein paar Sekunden`, 'warn');
    if ($('#tree')) renderSide();
  }

  // Keeps the disk trees and the user shares in step after anything changed either of them.
  const known = new Set();
  let userDirs = new Set(), devDirs = Object.fromEntries(DEVS.map(d => [d, new Set()]));
  let uids = 0;
  const root = (vol) => '/mnt/' + vol;
  function scan(vol) {
    const files = new Map(), dirs = new Set(), base = root(vol);
    FS.walk((k, p) => { const rel = p.slice(base.length + 1); if (isFile(k)) files.set(k, rel); else dirs.add(rel); }, FS.nodeAt(base), base);
    return { files, dirs };
  }
  const scanAll = () => ({ u: scan('user'), d: Object.fromEntries(DEVS.map(dev => [dev, scan(dev)])) });
  const forget = (n) => { known.delete(n); if (!isFile(n)) FS.walk(k => known.delete(k), n); };
  function cut(vol, rel, node) {
    const parent = FS.nodeAt(FS.parent(`${root(vol)}/${rel}`)), name = FS.split(rel).pop();
    if (parent?.children) parent.children = parent.children.filter(c => node ? c !== node : c.name !== name);
  }
  function put(vol, rel, node) {
    const dir = bk.mkdirs(FS.parent(`${root(vol)}/${rel}`));
    if (dir.children.some(c => c.name === node.name)) return false;
    dir.children.push(node);
    return true;
  }

  function sync() {
    let { u, d } = scanAll();
    for (const rel of userDirs) {
      if (u.dirs.has(rel)) continue;
      for (const dev of DEVS) { const n = d[dev].dirs.has(rel) && FS.nodeAt(`${root(dev)}/${rel}`); if (n) { forget(n); cut(dev, rel); } }
    }
    for (const dev of DEVS) for (const rel of devDirs[dev]) {
      if (d[dev].dirs.has(rel) || !u.dirs.has(rel) || DEVS.some(o => d[o].dirs.has(rel))) continue;
      const n = FS.nodeAt('/mnt/user/' + rel);
      if (n) { forget(n); cut('user', rel); }
    }
    ({ u, d } = scanAll());
    for (const n of [...known]) {
      const inUser = u.files.has(n), dev = DEVS.find(x => d[x].files.has(n));
      if (inUser && dev) {
        if (u.files.get(n) !== d[dev].files.get(n)) { cut('user', u.files.get(n), n); put('user', d[dev].files.get(n), n); }
        continue;
      }
      known.delete(n);
      if (inUser) cut('user', u.files.get(n), n);
      if (dev) cut(dev, d[dev].files.get(n), n);
    }
    ({ u, d } = scanAll());
    // A node that turns up with the id of a file still in place is a copy: a new file with its own disk and no links.
    const live = new Set([...known].map(n => n.uid));
    const adopt = (n, rel, dev) => {
      const share = FS.split(rel)[0];
      if (live.has(n.uid)) { delete n.uid; delete n.ino; delete n.dev; }
      if (dev) { if (n.dev && n.dev !== dev) delete n.ino; n.dev = dev; }
      else if (!n.dev || n.share !== share) { if (n.dev) delete n.ino; n.dev = allocate(rel, n); }
      n.share = share;
      n.uid ??= ++uids;
      live.add(n.uid);
    };
    for (const [n, rel] of u.files) if (!known.has(n)) { adopt(n, rel); if (put(n.dev, rel, n)) known.add(n); }
    for (const dev of DEVS) for (const [n, rel] of d[dev].files) if (!known.has(n)) { adopt(n, rel, dev); if (put('user', rel, n)) known.add(n); }
    ({ u, d } = scanAll());
    for (const dev of DEVS) for (const rel of d[dev].dirs) if (!u.dirs.has(rel)) bk.mkdirs('/mnt/user/' + rel);
    for (const rel of u.dirs) if (!DEVS.some(dev => d[dev].dirs.has(rel))) bk.mkdirs(`${root(home(rel))}/${rel}`);
    ({ u, d } = scanAll());
    userDirs = u.dirs;
    devDirs = Object.fromEntries(DEVS.map(dev => [dev, d[dev].dirs]));
  }

  let inos = 0;
  const newIno = () => ++inos;
  const hardlink = (a, b) => {
    const x = FS.nodeAt('/mnt/user/' + a), y = FS.nodeAt('/mnt/user/' + b);
    if (!x || !y) return;
    x.ino = y.ino = newIno();
    Object.assign(y, { size: x.size, time: x.time });
  };
  hardlink('downloads/Big.Buck.Bunny.2008.1080p.mkv', 'filme/Big Buck Bunny (2008)/Big Buck Bunny (2008).mkv');
  hardlink('downloads/album.flac', 'musik/album.flac');
  sync();
  booting = false;

  const render = CC.render;
  CC.render = () => { sync(); render(); };

  CC.unraid = { DISKS, DEVS, SHARES, shareOf, hasArray, devName, names, where, filesOf, devOf, linksOf, allowed, landsOn, onCache, spin, wake, sync, newIno, put, cut };
})();
