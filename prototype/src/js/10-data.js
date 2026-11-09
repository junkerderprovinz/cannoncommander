// Made-up data shaped like an Unraid server, so every screen has something real to show.
const FORMAT_GLYPH = Object.fromEntries(FORMATS.flatMap(f => f.ext.map(e => [e, f.glyph])));
const F = (name, size, extra = {}) => ({ name, size, time: NOW - Math.floor(rnd() * 300) * DAY - Math.floor(rnd() * DAY), owner: 'nobody', group: 'users', mode: '0664', ...extra });
const D = (name, children = [], extra = {}) => ({ name, dir: true, children, time: NOW - Math.floor(rnd() * 200) * DAY, owner: 'nobody', group: 'users', mode: '0775', ...extra });
const photos = (n, prefix, start = 1) => Array.from({ length: n }, (_, i) => F(`${prefix}_${String(start + i).padStart(4, '0')}.jpg`, Math.floor(2.2 * MB + rnd() * 5 * MB)));

const ROOT = D('', [
  D('mnt', [
    D('user', [
      D('appdata', [
        D('bombvault', [F('bombvault.db', 48 * MB), D('logs', [F('engine.log', 3.4 * MB), F('engine.log.1', 9.8 * MB)])]),
        D('knightloader', [F('settings.json', 9 * KB), F('history.db', 12 * MB)]),
        D('plex', [D('Library', [F('Preferences.xml', 6 * KB)])]),
        D('nginx', [F('database.sqlite', 2 * MB), D('proxy_host', [F('1.conf', 3 * KB), F('44.conf', 3 * KB)])]),
        D('homarr', [F('db.sqlite', 4 * MB)]),
        F('.DS_Store', 6 * KB, { hidden: true }),
      ]),
      D('downloads', [
        F('ubuntu-24.04.3-desktop-amd64.iso', 6.1 * GB),
        F('Fotos-Urlaub-2026.zip', 842 * MB, { zip: [F('IMG_0001.jpg', 4.1 * MB), F('IMG_0002.jpg', 3.8 * MB), F('IMG_0003.jpg', 4.4 * MB), D('Strand', [F('IMG_0101.jpg', 4.0 * MB), F('IMG_0102.jpg', 3.6 * MB)])] }),
        F('Rechnung-Oktober.pdf', 212 * KB), F('setup-notes.md', 4 * KB), F('Big.Buck.Bunny.2008.1080p.mkv', 1.4 * GB),
        F('album.flac', 41 * MB), F('ubuntu-24.04.3-desktop-amd64.iso.sha256', 1 * KB),
        D('unfertig', [F('teil1.part', 700 * MB)]),
        F('.partial-download', 120 * MB, { hidden: true }),
      ]),
      D('filme', [
        D('Big Buck Bunny (2008)', [F('Big Buck Bunny (2008).mkv', 1.4 * GB), F('Big Buck Bunny (2008).de.srt', 52 * KB), F('poster.jpg', 380 * KB)]),
        D('Sintel (2010)', [F('Sintel (2010).mkv', 2.2 * GB), F('poster.jpg', 410 * KB)]),
        D('Tears of Steel (2012)', [F('Tears of Steel (2012).mkv', 3.1 * GB), F('poster.jpg', 395 * KB)]),
        D('Elephants Dream (2006)', [F('Elephants Dream (2006).mkv', 0.9 * GB)]),
      ]),
      D('fotos', [D('2025', photos(14, 'IMG')), D('2026', photos(22, 'IMG', 200)), D('Urlaub', photos(9, 'DSC')), F('beste-bilder.txt', 2 * KB)]),
      D('musik', [D('Kevin MacLeod', [F('Sneaky Snitch.mp3', 6 * MB), F('Monkeys Spinning Monkeys.mp3', 5 * MB), F('Cipher.mp3', 7 * MB)]), D('Podcasts', [])]),
      D('backups', [F('flash-2026-10-31.zip', 412 * MB), F('appdata-2026-10-31.tar.zst', 18.4 * GB), F('appdata-2026-10-24.tar.zst', 18.1 * GB), F('vm-win11-2026-10-01.img', 64 * GB), F('ubuntu-24.04.3-desktop-amd64.iso', 6.1 * GB)]),
      D('isos', [F('Win11_24H2_German_x64.iso', 5.8 * GB), F('debian-13.1.0-amd64-netinst.iso', 754 * MB), F('ubuntu-24.04.3-desktop-amd64.iso', 6.1 * GB)]),
      D('dokumente', [
        F('Mietvertrag.pdf', 1.2 * MB), F('Steuer 2025.xlsx', 88 * KB), F('Einkaufsliste.md', 1 * KB), F('Bewerbung.docx', 140 * KB),
        F('docker-compose.yml', 3 * KB), F('server.go', 18 * KB), F('notizen.txt', 3 * KB), F('Präsentation.pptx', 2.4 * MB),
        F('aktuell.md', 2 * KB, { link: '/mnt/user/dokumente/Einkaufsliste.md' }),
      ]),
    ]),
    D('cache', [D('appdata', []), D('system', [F('docker.img', 40 * GB), F('libvirt.img', 1 * GB)])]),
    D('disk1', [D('filme', []), D('backups', [])]),
    D('disk2', [D('fotos', []), D('musik', [])]),
  ]),
  D('remotes', [D('gdrive', [D('Dokumente', [F('Lebenslauf.pdf', 300 * KB)]), F('Notizbuch.md', 6 * KB)], { remote: 'rclone: gdrive' }), D('nas-keller', [D('archiv', [])], { remote: 'SMB: //keller/archiv' })]),
  D('boot', [F('bzimage', 9 * MB), F('bzroot', 140 * MB), D('config', [F('ident.cfg', 1 * KB), F('go', 1 * KB)])], { owner: 'root', group: 'root' }),
]);

const FS = {
  root: ROOT,
  FREE: { '/mnt/user': 1.4 * TB, '/mnt/cache': 420 * GB, '/mnt/disk1': 610 * GB, '/mnt/disk2': 790 * GB, '/boot': 24 * GB, '/remotes': 1.8 * TB },
  TOTAL: { '/mnt/user': 8 * TB, '/mnt/cache': 1 * TB, '/mnt/disk1': 4 * TB, '/mnt/disk2': 4 * TB, '/boot': 32 * GB, '/remotes': 2 * TB },
  LABEL: { '/mnt/user': 'Shares', '/mnt/cache': 'Cache', '/mnt/disk1': 'Disk 1', '/mnt/disk2': 'Disk 2', '/boot': 'USB-Stick', '/remotes': 'Fernziele' },
  split: (p) => p.split('/').filter(Boolean),
  join: (path, name) => (path === '/' ? '' : path) + '/' + name,
  parent(path) { const p = FS.split(path); p.pop(); return '/' + p.join('/'); },
  // An entry inside an archive carries "::" after the archive's own name.
  nodeAt(path) {
    let n = ROOT;
    for (const part of FS.split(path)) {
      const isZip = part.endsWith('::');
      const name = isZip ? part.slice(0, -2) : part;
      const next = (n.children || []).find(k => k.name === name);
      if (!next) return null;
      n = isZip ? { ...next, dir: true, children: next.zip, inZip: true } : next;
    }
    return n;
  },
  kids: (n) => (n && n.children) || [],
  totalSize: (n) => n.dir ? FS.kids(n).reduce((a, k) => a + FS.totalSize(k), 0) : (n.size || 0),
  volumeOf: (path) => Object.keys(FS.FREE).filter(v => path.startsWith(v)).sort((a, b) => b.length - a.length)[0] || '/mnt/user',
  inArchive: (path) => path.includes('::'),
  ext: (name) => name.includes('.') ? name.split('.').pop().toLowerCase() : '',
  kindOf(n) {
    if (n.dir) return 'dir';
    const e = FS.ext(n.name);
    if (['jpg', 'jpeg', 'png', 'webp', 'heic', 'gif'].includes(e)) return 'img';
    if (['mkv', 'mp4', 'webm', 'mov'].includes(e)) return 'video';
    if (['mp3', 'flac', 'ogg', 'wav'].includes(e)) return 'audio';
    if (['zip', 'zst', '7z', 'rar', 'tar', 'gz'].includes(e)) return 'arch';
    if (['iso', 'img'].includes(e)) return 'disk';
    if (e === 'pdf') return 'pdf';
    if (['docx', 'xlsx', 'pptx', 'odt'].includes(e)) return 'office';
    if (['md', 'txt', 'srt', 'xml', 'json', 'cfg', 'log', 'sha256', 'conf', '1'].includes(e)) return 'text';
    if (['yml', 'yaml', 'go', 'sh', 'js', 'ts'].includes(e)) return 'code';
    return 'file';
  },
  GLYPH: { dir: 'IconFolder', img: 'IconFileImage', video: 'IconFileVideo', audio: 'IconFileAudio', arch: 'IconArchive', disk: 'IconDatabase', pdf: 'IconFileText', office: 'IconFileText', text: 'IconFileText', code: 'IconCode', file: 'IconFile' },
  // The kind decides how a file is shown; the glyph comes from the format list, which is more precise than the kind.
  glyphOf(n) {
    if (n.dir) return 'IconFolder';
    const e = /\.log\.\d+$/.test(n.name) ? 'log' : n.name === 'go' ? 'cfg' : FS.ext(n.name);
    return FORMAT_GLYPH[e] || FS.GLYPH[FS.kindOf(n)];
  },
  walk(fn, n = ROOT, path = '') {
    for (const k of FS.kids(n)) { const p = path + '/' + k.name; fn(k, p, n); if (k.dir) FS.walk(fn, k, p); }
  },
};

// The episodes get fixed dates instead of seeded ones, so every file above keeps the dates it has.
const fixedNode = (extra) => ({ time: NOW - 12 * DAY, owner: 'nobody', group: 'users', mode: extra.dir ? '0775' : '0664', ...extra });
FS.nodeAt('/mnt/user').children.splice(3, 0, fixedNode({ name: 'serien', dir: true, children: [
  fixedNode({ name: 'Pioneer One', dir: true, children: [
    fixedNode({ name: 'Staffel 1', dir: true, children: [1, 2, 3, 4, 5, 6].map(n => fixedNode({ name: `pioneer.one.s01e0${n}.720p.mkv`, size: (610 + n * 37) * MB, time: NOW - (60 - n * 7) * DAY })) }),
  ] }),
] }));

const DATA = {
  users: [
    { name: 'jdp', full: 'Junker der Provinz', role: 'Admin', groups: ['admins', 'familie'], scope: '/mnt/user', last: NOW - 3 * 60000, twofa: true },
    { name: 'anna', full: 'Anna', role: 'Nutzer', groups: ['familie'], scope: '/mnt/user/fotos', last: NOW - 2 * DAY, twofa: false },
    { name: 'gast', full: 'Gast', role: 'Nur lesen', groups: [], scope: '/mnt/user/filme', last: NOW - 40 * DAY, twofa: false },
  ],
  groups: [
    { name: 'admins', members: ['jdp'], note: 'Alles, auch Einstellungen' },
    { name: 'familie', members: ['jdp', 'anna'], note: 'Fotos, Musik, Filme' },
  ],
  sources: [
    { name: 'Shares', path: '/mnt/user', readOnly: false, indexed: true, files: 182344, size: 6.6 * TB },
    { name: 'Cache', path: '/mnt/cache', readOnly: false, indexed: true, files: 4021, size: 580 * GB },
    { name: 'USB-Stick', path: '/boot', readOnly: true, indexed: false, files: 211, size: 8 * GB },
    { name: 'Fernziele', path: '/remotes', readOnly: false, indexed: false, files: 0, size: 0 },
  ],
  rules: [
    { who: 'anna', path: '/mnt/user/backups', rule: 'verbergen' },
    { who: 'gast', path: '/mnt/user/filme', rule: 'nur lesen' },
    { who: 'alle', path: '*.partial*', rule: 'verbergen' },
  ],
  shares: [
    { path: '/mnt/user/fotos/Urlaub', by: 'jdp', created: NOW - 5 * DAY, expires: NOW + 25 * DAY, password: true, views: 14, slug: 'k3x9-urlaub' },
    { path: '/mnt/user/dokumente/Rechnung-Oktober.pdf', by: 'jdp', created: NOW - 1 * DAY, expires: NOW + 6 * DAY, password: false, views: 2, slug: 'p7q2-rechnung' },
  ],
  trash: [
    { name: 'alte-notizen.txt', from: '/mnt/user/dokumente', size: 3 * KB, deleted: NOW - 2 * DAY, by: 'jdp' },
    { name: 'IMG_0031.jpg', from: '/mnt/user/fotos/2025', size: 4.2 * MB, deleted: NOW - 6 * DAY, by: 'anna' },
    { name: 'test-ordner', from: '/mnt/user/downloads', size: 120 * MB, deleted: NOW - 12 * DAY, by: 'jdp', dir: true },
  ],
  activity: [
    { t: NOW - 4 * 60000, who: 'jdp', what: 'kopiert', obj: '2 Dateien nach /mnt/user/backups' },
    { t: NOW - 40 * 60000, who: 'jdp', what: 'umbenannt', obj: '22 Fotos in /mnt/user/fotos/2026' },
    { t: NOW - 3 * 3600000, who: 'anna', what: 'hochgeladen', obj: '9 Fotos nach /mnt/user/fotos/Urlaub' },
    { t: NOW - 5 * 3600000, who: 'jdp', what: 'freigegeben', obj: '/mnt/user/fotos/Urlaub' },
    { t: NOW - 1 * DAY, who: 'gast', what: 'angemeldet', obj: 'von 192.168.30.50' },
    { t: NOW - 2 * DAY, who: 'jdp', what: 'gelöscht', obj: 'alte-notizen.txt' },
  ],
  sessions: [
    { device: 'Firefox auf Windows', where: '192.168.10.16', last: NOW - 60000, current: true },
    { device: 'Firefox auf Android', where: '192.168.30.50', last: NOW - 2 * 3600000 },
    { device: 'API-Token „backup-skript“', where: '192.168.20.87', last: NOW - DAY },
  ],
  tokens: [{ name: 'backup-skript', created: NOW - 30 * DAY, last: NOW - DAY, rights: 'lesen, schreiben' }],
  remotes: [
    { name: 'gdrive', type: 'drive', provider: 'gdrive', mount: '/remotes/gdrive', state: 'verbunden' },
    { name: 'nas-keller', type: 'smb', provider: 'smb', path: 'archiv', mount: '/remotes/nas-keller', state: 'verbunden' },
  ],
  bookmarks: [
    { name: 'Downloads', path: '/mnt/user/downloads', g: 'IconDownload' },
    { name: 'appdata', path: '/mnt/user/appdata', g: 'IconConfig' },
    { name: 'Filme', path: '/mnt/user/filme', g: 'IconFileVideo' },
    { name: 'Fotos', path: '/mnt/user/fotos', g: 'IconFileImage' },
    { name: 'Backups', path: '/mnt/user/backups', g: 'IconBackupNow' },
  ],
  version: { app: '0.1.0', glimstone: '4.0.0', quantum: '2.1.4', build: 'Vorschau, Runde 1' },
};

// Background jobs: copy, move, delete, pack and friends run here and report into the Aufgaben window.
// The queue works in list order. Nacheinander moves only the first job that is not paused, Gleichzeitig moves all of them.
const TICK = 260;
// The prototype runs its clock fast, so a copy of many gigabytes ends in seconds while rate and time left read like the server's.
const LAPSE = 20;
const Jobs = {
  list: [],
  timer: null,
  serial: () => CC.prefs.jobMode !== 'parallel',
  start({ kind, label, total = 0, speed = 1, files = [], done }) {
    const job = { id: Date.now() + Math.random(), kind, label, total, speed, files, done, queue: true, pct: 0, rate: 0, state: 'wartet', started: Date.now() };
    Jobs.list.push(job);
    Jobs.timer ||= setInterval(Jobs.tick, TICK);
    Jobs.tick(false);
    return job;
  },
  tick(advance = true) {
    const live = Jobs.list.filter(j => j.queue && (j.state === 'läuft' || j.state === 'wartet'));
    const free = live.filter(j => !Jobs.held(j));
    const moving = Jobs.serial() ? free.slice(0, 1) : free;
    // Jobs side by side share the disks, so together they are only a little faster than one alone.
    const share = moving.length > 1 ? 1.4 / moving.length : 1;
    for (const j of live) {
      j.state = moving.includes(j) ? 'läuft' : 'wartet';
      if (j.state === 'wartet') j.rate = 0;
    }
    if (advance) for (const j of moving) {
      if (j.total) {
        j.rate = Math.min((150 + rnd() * 110) * MB * j.speed * share, Jobs.limit(j, moving.length));
        j.pct = Math.min(100, j.pct + j.rate * TICK / 1000 * LAPSE / j.total * 100);
      } else j.pct = Math.min(100, j.pct + (5 + rnd() * 12) * j.speed * share);
      if (j.pct >= 100) {
        j.state = 'fertig';
        j.rate = 0;
        j.done && j.done(j);
      }
    }
    if (!Jobs.list.some(j => j.queue && ['läuft', 'wartet', 'pausiert'].includes(j.state))) { clearInterval(Jobs.timer); Jobs.timer = null; }
    Jobs.changed();
  },
  // Every file below the items, named from the item on, so a job can say which one it is at.
  filesOf: (items) => items.flatMap(k => {
    if (!k.dir) return [{ name: k.name, size: k.size || 0 }];
    const out = [];
    FS.walk((c, p) => { if (!c.dir) out.push({ name: k.name + p, size: c.size || 0 }); }, k);
    return out;
  }),
  // Seconds left at the current rate, in the server's time.
  left: (j) => j.rate && j.total ? j.total * (100 - j.pct) / 100 / j.rate : null,
  // The file a job works on: the first one whose share of the total is not yet done.
  current(j) {
    let at = j.total * j.pct / 100;
    for (const f of j.files) { if (at < f.size) return f.name; at -= f.size; }
    return j.files.at(-1)?.name || '';
  },
  pause(job) {
    job.state = job.state === 'pausiert' ? (job.queue ? 'wartet' : 'läuft') : 'pausiert';
    if (job.queue) { Jobs.timer ||= setInterval(Jobs.tick, TICK); Jobs.tick(false); } else Jobs.changed();
  },
  cancel(job) { job.state = 'abgebrochen'; job.rate = 0; Jobs.tick(false); },
  // Moves a job to another place in the queue; finished ones keep their place below.
  move(job, to) {
    const open = Jobs.running(), from = open.indexOf(job);
    if (from < 0 || to < 0 || to >= open.length || to === from) return;
    open.splice(to, 0, open.splice(from, 1)[0]);
    Jobs.list = [...open, ...Jobs.list.filter(j => !open.includes(j))];
    Jobs.tick(false);
  },
  running: () => Jobs.list.filter(j => ['läuft', 'wartet', 'pausiert'].includes(j.state)),
  // Bytes per second a job may use, and whether it has to wait; features narrow both.
  limit: () => Infinity,
  held: () => false,
  listeners: [],
  changed() { Jobs.listeners.forEach(f => f()); Shell.updateJobs(); },
};
