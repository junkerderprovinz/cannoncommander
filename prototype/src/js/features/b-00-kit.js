// Shared parts of the Krusader tools: real nodes behind the panel copies, a folder picker, made-up contents and hashes.
(() => {
  const { sw, frow, cancel } = CC.ui;

  const strHash = (s) => { let h = 2166136261; for (const c of String(s)) { h ^= c.codePointAt(0); h = Math.imul(h, 16777619); } return h >>> 0; };
  const rng = (key) => {
    let a = strHash(key) || 1;
    return () => { a = (a + 0x6D2B79F5) | 0; let t = Math.imul(a ^ (a >>> 15), 1 | a); t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t; return ((t ^ (t >>> 14)) >>> 0) / 4294967296; };
  };
  const hex = (key, len) => { const r = rng(key); let s = ''; while (s.length < len) s += Math.floor(r() * 16).toString(16); return s; };

  const ALGOS = [['md5', 'MD5', 32], ['sha1', 'SHA-1', 40], ['sha256', 'SHA-256', 64], ['sha512', 'SHA-512', 128], ['b3', 'BLAKE3', 64]];
  // Copies with the same name and size hash the same, so a duplicate really is one.
  const hashOf = (algo, node) => hex(`${algo}|${node.name}|${node.size}`, ALGOS.find(a => a[0] === algo)[2]);

  const here = () => Files.T().path;
  const kidsOf = (dir) => FS.kids(FS.nodeAt(dir));
  const nodeIn = (dir, name) => kidsOf(dir).find(c => c.name === name);
  // Files.sel() hands out copies; whatever gets changed has to be the node in the tree.
  const selNodes = () => Files.sel().map(k => nodeIn(here(), k.name)).filter(Boolean);
  const otherDir = () => CC.prefs.dual ? Files.T(Files.other()).path : here();
  const isDir = (p) => !!FS.nodeAt(p)?.dir && !FS.inArchive(p);
  const hidden = (n) => n.hidden || n.name.startsWith('.');
  const fileNode = (name, size, extra = {}) => ({ name, size: Math.round(size), time: NOW, owner: 'nobody', group: 'users', mode: '0664', ...extra });
  const dirNode = (name, children = [], extra = {}) => ({ name, dir: true, children, time: NOW, owner: 'nobody', group: 'users', mode: '0775', ...extra });
  const clone = (n) => { const c = JSON.parse(JSON.stringify(n)); delete c.kind; return c; };
  const baseExt = (name) => { const m = name.match(/^(.+?)((?:\.tar)?\.[^.]+)$/); return m && m[1] ? [m[1], m[2]] : [name, '']; };
  const mkdirs = (path) => {
    let n = FS.root;
    for (const part of FS.split(path)) {
      let next = FS.kids(n).find(k => k.name === part);
      if (!next) { next = dirNode(part); n.children.push(next); }
      n = next;
    }
    return n;
  };

  // Puts the cursor on a name in every panel that shows its folder.
  function reveal(dir, name) {
    if (CC.view !== 'files') return;
    for (let i = 0; i < (CC.prefs.dual ? 2 : 1); i++) {
      if (Files.T(i).path !== dir) continue;
      const idx = Files.entries(i).findIndex(e => e.name === name);
      if (idx >= 0) Files.T(i).cursor = idx;
    }
    CC.render();
  }

  const log = (what, obj) => DATA.activity.unshift({ t: Date.now(), who: CC.user.name, what, obj });

  function copyText(text) {
    try { navigator.clipboard?.writeText(text)?.catch(() => {}); } catch {}
    CC.toast('Kopiert');
  }
  const copyBtn = (text, tip = 'Kopieren') => `<button class="btn icon bk-ghost" data-bk-copy="${esc(text)}" data-tip="${esc(tip)}" aria-label="${esc(tip)}">${g('IconCopy')}</button>`;
  document.addEventListener('click', e => { const b = e.target.closest('[data-bk-copy]'); if (b) copyText(b.dataset.bkCopy); });
  const doneToast = (text, path) => CC.toast(text, 'ok', { label: 'Pfad kopieren', run: () => copyText(path) });

  const pathField = (id, value) => `<span class="bk-pathf"><input class="field grow mono" id="${id}" value="${esc(value)}" spellcheck="false" autocomplete="off"><button class="btn icon" data-bk-pick="${id}" data-tip="Ordner wählen" aria-label="Ordner wählen">${g('IconFolderOpen')}</button></span>`;
  function wirePicks(root) {
    root.addEventListener('click', async e => {
      const b = e.target.closest('[data-bk-pick]');
      if (!b) return;
      const input = root.querySelector('#' + b.dataset.bkPick);
      const p = await pickFolder(input.value.trim());
      if (p) { input.value = p; input.dispatchEvent(new Event('input', { bubbles: true })); }
      input.focus();
    });
  }

  function pickFolder(start) {
    return new Promise(resolve => {
      let cur = isDir(start) ? start : '/mnt/user', picked = null;
      const w = win({
        title: 'Ordner wählen', cls: 'bk bk-picker',
        body: `<div class="pathbar bk-pathbar" id="pf-crumbs"></div><div class="well bk-fill"><div class="bk-list" id="pf-list"></div></div>`,
        foot: [cancel, { label: 'Diesen Ordner wählen', g: 'IconCheck', primary: true, run: () => { picked = cur; } }],
        onClose: () => { steps.close(); resolve(picked); },
      });
      const steps = folderSteps(w, () => cur, (p) => { cur = p; draw(); });
      const draw = () => {
        $('#pf-crumbs', w).innerHTML = crumbs(cur, 'data-bk-cd');
        const dirs = FS.kids(FS.nodeAt(cur)).filter(k => k.dir && !k.link && (CC.prefs.showHidden || !hidden(k))).sort((a, b) => a.name.localeCompare(b.name, 'de', { numeric: true }));
        $('#pf-list', w).innerHTML = (cur !== '/' ? `<button class="bk-row" data-bk-cd="${esc(FS.parent(cur))}"><span class="bk-tile">${g('IconFolderUp')}</span><span class="bk-name muted">..</span></button>` : '')
          + (dirs.length ? dirs.map(k => `<button class="bk-row" data-bk-cd="${esc(FS.join(cur, k.name))}"><span class="bk-tile dir">${g('IconFolder')}</span><span class="bk-name">${esc(FS.LABEL[FS.join(cur, k.name)] || k.name)}</span><span class="pill glim-num">${FS.kids(k).length} Obj.</span></button>`).join('')
            : `<div class="empty">${g('IconFolder')}<div>Hier gibt es keine Unterordner</div></div>`);
      };
      w.addEventListener('click', e => { const c = e.target.closest('[data-bk-cd]'); if (c) steps.go(c.dataset.bkCd); });
      draw();
    });
  }

  const crumbs = (path, attr) => {
    const parts = FS.split(path);
    return `<button class="pseg ${parts.length ? '' : 'last'}" ${attr}="/">Server</button>` + parts.map((p, k) => `<span class="psep" aria-hidden="true">›</span><button class="pseg ${k === parts.length - 1 ? 'last' : ''}" ${attr}="${esc('/' + parts.slice(0, k + 1).join('/'))}">${esc(p.replace('::', ''))}</button>`).join('');
  };
  // Back and forward for a window that moves between folders, wired to the mouse's side buttons.
  function folderSteps(w, get, set) {
    const past = [], ahead = [];
    const nav = CC.winNav(w, {
      back: () => { if (!past.length) return; ahead.push(get()); set(past.pop()); },
      forward: () => { if (!ahead.length) return; past.push(get()); set(ahead.pop()); },
    });
    return {
      go: (p) => { if (p === get()) return; past.push(get()); ahead.length = 0; set(p); nav.push(); },
      close: nav.close,
    };
  }

  // A pattern without * or ? matches anywhere in the name, as people type it into a search field.
  function globRe(pattern) {
    const parts = String(pattern).trim().split(/[\s;]+/).filter(Boolean);
    if (!parts.length) return null;
    const one = (p) => {
      const body = p.replace(/[.+^${}()|[\]\\]/g, '\\$&').replace(/\*/g, '.*').replace(/\?/g, '.');
      return /[*?]/.test(p) ? `^${body}$` : body;
    };
    return new RegExp(parts.map(one).join('|'), 'i');
  }

  const primary = (w) => $('.win-foot .btn.primary', w);
  const setPrimary = (w, label, glyph, disabled = false) => { const b = primary(w); b.innerHTML = g(glyph) + esc(label); b.disabled = disabled; };
  const setStart = (w, text) => { const s = $('.win-foot .start', w); if (s) s.textContent = text; };
  const tile = (n) => `<span class="bk-tile ${n.dir ? 'dir' : ''}">${g(FS.glyphOf(n))}</span>`;
  const plural = (n, one, many) => `${n.toLocaleString('de-AT')} ${n === 1 ? one : many}`;

  // Made-up contents for files the data only knows by name, so viewers and comparisons have something to show.
  const LINES = {
    md: (n) => [`# ${n.replace(/\.md$/, '')}`, '', 'Stand: Oktober 2026', '', '## Offen', '', '- Parity-Check nach dem Update laufen lassen', '- appdata-Backup auf disk2 prüfen', '- Plex auf die neue Version heben', '- Cache-Pool auf 1 TB tauschen', '', '## Erledigt', '', '- Docker-Netz auf br0 umgestellt', '- USB-Stick gesichert (flash-2026-10-31.zip)', '- Freigaben für anna eingerichtet', '', '## Notizen', '', 'Die Lüfter laufen nachts auf 30 %, tagsüber auf 45 %.', 'Spin-down nach 45 Minuten, Cache schreibt um 3:40 Uhr auf das Array.', '', '| Platte | Größe | Temperatur |', '|---|---|---|', '| disk1 | 4 TB | 34 °C |', '| disk2 | 4 TB | 36 °C |', '| parity | 4 TB | 33 °C |'],
    txt: (n) => [n.replace(/\.txt$/, ''), '', 'Router: 192.168.10.1', 'Server: 192.168.10.20 (Bottich)', 'Drucker: 192.168.10.31', '', 'WLAN Gäste wechselt jeden Monat', 'Backup läuft sonntags um 2:00', 'Parity-Check am 1. jedes Monats', '', 'Ideen', '- Pi-hole in eine VM', '- Nextcloud oder lieber Syncthing?', '- zweiten Cache-Pool für VMs', '', 'Zuletzt geändert von jdp'],
    yml: () => ['services:', '  plex:', '    image: lscr.io/linuxserver/plex:latest', '    container_name: plex', '    network_mode: host', '    environment:', '      - PUID=99', '      - PGID=100', '      - TZ=Europe/Vienna', '      - VERSION=docker', '    volumes:', '      - /mnt/user/appdata/plex:/config', '      - /mnt/user/filme:/movies', '      - /mnt/user/musik:/music', '    restart: unless-stopped', '', '  homarr:', '    image: ghcr.io/homarr-labs/homarr:latest', '    container_name: homarr', '    ports:', '      - 7575:7575', '    volumes:', '      - /mnt/user/appdata/homarr:/appdata', '    restart: unless-stopped'],
    go: () => ['package main', '', 'import (', '\t"log"', '\t"net/http"', '\t"os"', '\t"time"', ')', '', 'func main() {', '\taddr := os.Getenv("LISTEN")', '\tif addr == "" {', '\t\taddr = ":8080"', '\t}', '\tmux := http.NewServeMux()', '\tmux.HandleFunc("/health", health)', '\tsrv := &http.Server{', '\t\tAddr:         addr,', '\t\tHandler:      mux,', '\t\tReadTimeout:  10 * time.Second,', '\t\tWriteTimeout: 30 * time.Second,', '\t}', '\tlog.Printf("listening on %s", addr)', '\tlog.Fatal(srv.ListenAndServe())', '}', '', 'func health(w http.ResponseWriter, r *http.Request) {', '\tw.WriteHeader(http.StatusOK)', '\tw.Write([]byte("ok"))', '}'],
    conf: () => ['server {', '  listen 443 ssl;', '  server_name files.bottich.lol;', '', '  ssl_certificate /data/custom_ssl/npm-1/fullchain.pem;', '  ssl_certificate_key /data/custom_ssl/npm-1/privkey.pem;', '', '  client_max_body_size 0;', '', '  location / {', '    proxy_pass http://192.168.10.20:8090;', '    proxy_set_header Host $host;', '    proxy_set_header X-Forwarded-For $remote_addr;', '    proxy_http_version 1.1;', '    proxy_set_header Upgrade $http_upgrade;', '    proxy_set_header Connection "upgrade";', '  }', '}'],
    json: () => ['{', '  "language": "de",', '  "theme": "dark",', '  "downloadDir": "/mnt/user/downloads",', '  "maxParallel": 3,', '  "speedLimit": 0,', '  "unpack": true,', '  "deleteArchives": false,', '  "notify": {', '    "onFinish": true,', '    "onError": true', '  },', '  "hosters": ["rapidgator", "1fichier"]', '}'],
    log: () => Array.from({ length: 30 }, (_, i) => `2026-11-0${1 + (i > 18)} ${String(3 + Math.floor(i / 3)).padStart(2, '0')}:${String((i * 7) % 60).padStart(2, '0')}:1${i % 10} ${['INFO', 'INFO', 'INFO', 'WARN'][i % 4]} ${['backup started', 'scanning /mnt/user/appdata', 'chunk store compacted', 'retry upload, attempt 2', 'snapshot written', 'verify ok'][i % 6]}`),
    srt: () => ['1', '00:00:12,000 --> 00:00:15,200', 'Wo bist du gewesen?', '', '2', '00:00:16,100 --> 00:00:18,900', 'Draußen im Wald. Ich habe etwas gefunden.', '', '3', '00:00:20,400 --> 00:00:23,000', 'Zeig es mir.', '', '4', '00:00:25,000 --> 00:00:28,600', 'Es ist größer, als ich dachte.', '', '5', '00:00:30,200 --> 00:00:32,800', 'Dann sollten wir uns beeilen.'],
    xml: () => ['<?xml version="1.0" encoding="utf-8"?>', '<Preferences', '  FriendlyName="Bottich"', '  TranscoderTempDirectory="/transcode"', '  ScheduledLibraryUpdateInterval="3600"', '  LanguageInCloud="1"', '  DlnaEnabled="0"', '  secureConnections="1"', '  ManualPortMappingPort="32400"', '/>'],
    cfg: () => ['NAME="Bottich"', 'COMMENT="Media and backups"', 'SYS_MODEL="Custom"', 'USE_SSL="auto"', 'PORT="80"', 'PORTSSL="443"', 'LOCAL_TLD="local"', 'USE_TELNET="no"', 'USE_SSH="yes"', 'PORTSSH="22"', 'USE_UPNP="no"', 'START_PAGE="Main"'],
  };
  LINES.yaml = LINES.yml; LINES['1'] = LINES.log;
  const MAGIC = { iso: 'CD001', zip: 'PK\x03\x04', pdf: '%PDF-1.7', jpg: '\xff\xd8\xff\xe0\x00\x10JFIF', mkv: '\x1a\x45\xdf\xa3', flac: 'fLaC', mp3: 'ID3\x04', img: 'QFI\xfb', '7z': "7z\xbc\xaf'\x1c", zst: '(\xb5/\xfd' };
  const hexLine = (off, bytes) => `${off.toString(16).padStart(8, '0')}  ${bytes.map(b => b.toString(16).padStart(2, '0')).join(' ')}  |${bytes.map(b => b >= 32 && b < 127 ? String.fromCharCode(b) : '.').join('')}|`;
  function hexdump(key, ext, rows = 40) {
    const r = rng(key), magic = [...(MAGIC[ext] || '')].map(c => c.charCodeAt(0));
    return Array.from({ length: rows }, (_, i) => hexLine(i * 16, Array.from({ length: 16 }, (_, k) => i === 0 && k < magic.length ? magic[k] : r() < .3 ? 0 : Math.floor(r() * 256))));
  }
  const isBinary = (n) => !['text', 'code'].includes(FS.kindOf(n)) && !LINES[FS.ext(n.name)] && !SUMS[FS.ext(n.name)];
  const SUMS = Object.fromEntries(ALGOS.map(a => [a[0], a[0]]));

  // The text of a checksum file: one "hash  name" line for the file it is named after.
  function sumText(dir, node) {
    const algo = SUMS[FS.ext(node.name)] || 'sha256';
    const target = node.name.replace(/\.[^.]+$/, '');
    const t = nodeIn(dir, target);
    return `${t ? hashOf(algo, t) : hex(target, 64)}  ${target}\n`;
  }

  function textOf(dir, node) {
    if (node.text != null) return node.text;
    if (node.content != null) return node.content;
    const e = FS.ext(node.name);
    if (SUMS[e]) return sumText(dir, node);
    if (LINES[e]) return LINES[e](node.name).join('\n');
    return hexdump(FS.join(dir, node.name), e).join('\n');
  }

  // A second version of a text with a few believable edits, for comparing two files that only exist as names.
  function variant(lines, key, binary) {
    const r = rng(key), out = lines.slice();
    const prose = ['erledigt', 'später', 'neu', 'doppelt', 'morgen'], code = ['true', 'false', '"debug"', '9090', 'v2'];
    const edits = 3 + Math.floor(r() * 4);
    const at = [...new Set(Array.from({ length: edits }, () => Math.floor(r() * out.length)))].sort((a, b) => b - a);
    for (const i of at) {
      if (binary) { const b = out[i].slice(10, 57).split(' ').map(x => parseInt(x, 16)); b[Math.floor(r() * 16)] = Math.floor(r() * 256); b[Math.floor(r() * 16)] = Math.floor(r() * 256); out[i] = hexLine(i * 16, b); continue; }
      const kind = r();
      if (kind < .2 && out.length > 4) out.splice(i, 1);
      else if (kind < .45) out.splice(i, 0, /^[\s{}()<>"\w]*[=:({<]/.test(out[i] || '') ? '  # TODO: nochmal prüfen' : '- Festplatte 3 bestellen');
      else if (/\d/.test(out[i])) out[i] = out[i].replace(/\d+/, m => String(+m + 1 + Math.floor(r() * 8)));
      else if (out[i].trim()) out[i] = out[i].replace(/(\w+)(\W*)$/, (m, w, t) => (/[{}();=:]/.test(out[i]) ? code : prose)[Math.floor(r() * 5)] + t);
      else out[i] = '# ' + prose[Math.floor(r() * 5)];
    }
    return out;
  }

  CC.bk = { folderSteps, rng, hex, hashOf, ALGOS, here, kidsOf, nodeIn, selNodes, otherDir, isDir, hidden, fileNode, dirNode, clone, baseExt, mkdirs, reveal, log, copyText, copyBtn, doneToast, pathField, wirePicks, pickFolder, crumbs, globRe, primary, setPrimary, setStart, tile, plural, textOf, variant, isBinary, SUMS, sumText };

  // A few more files so the tools have something to find: copies, a backup to compare against, split parts, a sum list.
  const at = (p) => FS.nodeAt(p);
  const copyOf = (from, toDir, extra = {}) => { const n = at(from); if (n) at(toDir).children.push({ ...clone(n), ...extra }); };
  copyOf('/mnt/user/downloads/Rechnung-Oktober.pdf', '/mnt/user/dokumente', { time: NOW - 20 * DAY });
  copyOf('/mnt/user/downloads/album.flac', '/mnt/user/musik', { time: NOW - 90 * DAY });
  copyOf('/mnt/user/fotos/2026/IMG_0203.jpg', '/mnt/user/fotos/Urlaub');
  copyOf('/mnt/user/isos/debian-13.1.0-amd64-netinst.iso', '/mnt/user/downloads', { time: NOW - 3 * DAY });
  const docs = at('/mnt/user/dokumente').children;
  const same = (name, extra = {}) => { const n = docs.find(k => k.name === name); return { ...clone(n), ...extra }; };
  at('/mnt/user/backups').children.push(dirNode('dokumente', [
    same('Mietvertrag.pdf'), same('Einkaufsliste.md'), same('notizen.txt'), same('Präsentation.pptx'),
    same('Steuer 2025.xlsx', { size: 61 * KB, time: NOW - 210 * DAY }),
    same('docker-compose.yml', { size: 2 * KB + 310, time: NOW - 160 * DAY }),
    same('Bewerbung.docx', { size: 152 * KB, time: NOW - 1 * DAY }),
    fileNode('alte-rechnung.pdf', 180 * KB, { time: NOW - 400 * DAY }),
  ], { time: NOW - 2 * DAY }));
  const parts = at('/mnt/user/downloads/unfertig').children;
  const whole = fileNode('Fotos-2019.7z', 2.4 * GB);
  parts.push(fileNode('Fotos-2019.7z.001', GB, { time: NOW - 6 * DAY }), fileNode('Fotos-2019.7z.002', GB, { time: NOW - 6 * DAY }), fileNode('Fotos-2019.7z.003', whole.size - 2 * GB, { time: NOW - 6 * DAY }),
    fileNode('Fotos-2019.7z.sha256', 82, { time: NOW - 6 * DAY, text: `${hashOf('sha256', whole)}  Fotos-2019.7z\n` }));
  const isos = at('/mnt/user/isos').children;
  const sumLine = (name, good = true) => { const n = isos.find(k => k.name === name); return `${good ? hashOf('sha256', n) : hex('kaputt' + name, 64)}  ${name}`; };
  isos.push(fileNode('checksums.sha256', 290, { time: NOW - 12 * DAY, text: [sumLine('Win11_24H2_German_x64.iso'), sumLine('debian-13.1.0-amd64-netinst.iso', false), sumLine('ubuntu-24.04.3-desktop-amd64.iso')].join('\n') + '\n' }));

  CC.css(`
.bk svg { fill: currentColor; }
.bk .well { min-width: 0; }
.bk .well .pill:not(.ok):not(.warn):not(.fault) { background: var(--carbon-surface3); }
.bk .well .tbl tr + tr td { box-shadow: inset 0 1px 0 color-mix(in srgb, var(--carbon-text) 6%, transparent); }
.bk .well .btn.bk-ghost, .bk .btn.bk-ghost { background: transparent; width: 1.75rem; height: 1.75rem; }
.bk .btn.bk-ghost:hover { background: var(--carbon-surface3); }
.bk .btn.bk-ghost:hover svg { color: var(--accent-glyph); }
.bk-pathf { display: flex; gap: .4rem; flex: 1; min-width: 0; max-width: 30rem; }
.bk-pathf .field { flex: 1; min-width: 0; }
.bk-form .frow > .lab { flex: 0 0 auto; min-width: 8.5rem; white-space: nowrap; }
.bk-form .frow .field.grow { flex: 1; }
.bk-form .frow > .seg, .bk-form .frow > .sw, .bk-form .frow > .a-pw { margin-left: auto; }
.bk-grid2 { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: .6rem 1.6rem; }
.bk-fill { flex: 1; min-height: 0; overflow: hidden; }
.bk-list { display: flex; flex-direction: column; gap: 1px; overflow: auto; min-height: 0; flex: 1; }
.pathbar.bk-pathbar { flex: none; height: 2.1rem; cursor: default; }
.bk-row { display: grid; grid-template-columns: 1.75rem minmax(0, 1fr) auto; align-items: center; gap: .6rem; height: 2.25rem; padding: 0 .45rem; border-radius: var(--radius-control); text-align: left; width: 100%; cursor: pointer; flex: none; }
.bk-row:hover, .bk-row:focus-visible, .bk-row.cur { background: var(--carbon-surface3); outline: none; }
.bk-row:hover .pill, .bk-row.cur .pill, .bk-step:hover .pill, .bk-copy:hover .pill:not(.warn), .bk-mrow:hover .pill:not(.fault) { background: var(--carbon-hover-raised); }
.bk-tile { width: 1.75rem; height: 1.75rem; border-radius: var(--radius-control); background: var(--carbon-surface3); display: grid; place-items: center; flex: none; }
.bk-tile svg { width: 1rem; height: 1rem; color: var(--carbon-text-sub); }
.bk-tile.dir svg { color: var(--accent-glyph); }
.bk-row:hover .bk-tile, .bk-row:focus-visible .bk-tile, .bk-row.cur .bk-tile { background: var(--accent); }
.bk-row:hover .bk-tile svg, .bk-row:focus-visible .bk-tile svg, .bk-row.cur .bk-tile svg { color: var(--accent-contrast); }
.bk-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; }
.bk.win.full { height: calc(100vh - 8.5rem); margin-bottom: 5rem; }
.bk-picker { height: min(34rem, calc(100vh - 3rem)); }
.bk-path { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; direction: rtl; text-align: left; min-width: 0; }
.bk-dot { width: .5rem; height: .5rem; border-radius: 50%; background: #6fdc8c; display: inline-block; flex: none; }
.bk-tools { display: flex; align-items: center; gap: .4rem; flex-wrap: wrap; }
.bk-tools .eyebrow { padding: 0; margin-right: auto; }
.bk-chip { height: 1.75rem; padding: 0 .65rem; border-radius: var(--radius-pill); background: var(--carbon-surface3); font-size: 13px; white-space: nowrap; }
.bk-chip:hover { background: color-mix(in srgb, var(--carbon-hover-raised) 78%, var(--btn-tint)); }
.bk-chip b { font-weight: 600; font-family: ui-monospace, "Cascadia Code", Menlo, monospace; font-size: 12px; margin-right: .35rem; }
.bk-scan { display: grid; gap: .6rem; justify-items: center; padding: 3rem 1rem; color: var(--carbon-text-muted); }
.bk-scan .bar { width: min(20rem, 80%); }
.bk .field:disabled, .bk .sw:disabled, .bk .seg.off { opacity: .45; cursor: not-allowed; }
.bk .seg.off { pointer-events: none; }
@media (max-width: 760px) {
  .bk.win { max-width: calc(100vw - 3rem); }
  .bk-grid2 { grid-template-columns: minmax(0, 1fr); }
  .bk-form .frow { flex-wrap: wrap; row-gap: .35rem; }
  .bk-form .frow > .lab { flex: 1 0 100%; white-space: normal; }
  .bk-form .frow:has(> .bk-val, > .sw) { flex-wrap: nowrap; }
  .bk-form .frow:has(> .bk-val, > .sw) > .lab { flex: 1 1 auto; min-width: 0; }
  .bk-form .seg { max-width: 100%; overflow-x: auto; scrollbar-width: none; margin-left: 0; }
  .bk-pathf, .bk-finalw { max-width: none; width: 100%; margin-left: 0; }
  .bk .win-foot .start { display: none; }
}
`);
})();
