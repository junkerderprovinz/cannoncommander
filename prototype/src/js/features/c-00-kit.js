// Shared by the viewer, editor, transfers and sign-in: stable made-up content and keys for a window on top.
(() => {
  const hash = (s) => { let h = 2166136261; for (const c of String(s)) h = Math.imul(h ^ c.codePointAt(0), 16777619); return h >>> 0; };
  const rng = (s) => {
    let a = hash(s) || 1;
    return () => { a = (a + 0x6D2B79F5) | 0; let t = Math.imul(a ^ (a >>> 15), 1 | a); t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t; return ((t ^ (t >>> 14)) >>> 0) / 4294967296; };
  };
  // Files.entries() hands out copies, so anything that writes goes through the node in the tree.
  const real = (folder, name) => FS.kids(FS.nodeAt(folder)).find(k => k.name === name);
  const fmtDur = (s) => {
    s = Math.max(0, Math.round(s));
    const h = Math.floor(s / 3600), m = Math.floor(s % 3600 / 60), x = String(s % 60).padStart(2, '0');
    return h ? `${h}:${String(m).padStart(2, '0')}:${x}` : `${m}:${x}`;
  };
  const plural = (n, one, many) => `${n.toLocaleString('de-AT')} ${n === 1 ? one : many}`;

  // The core passes only Escape to an open window, so each window listens for its own keys while it is on top.
  function winKeys(scrim, handler) {
    const fn = (e) => {
      if (!scrim.isConnected) return document.removeEventListener('keydown', fn, true);
      if (winStack.at(-1)?.el !== scrim) return;
      if (handler(e, comboOf(e)) === true) { e.preventDefault(); e.stopPropagation(); }
    };
    document.addEventListener('keydown', fn, true);
  }

  // Which renderer a file gets; a few extensionless config files are text too.
  const viewKind = (n) => {
    const k = FS.kindOf(n);
    if (k === 'file' && ['go', 'ident', 'Dockerfile', 'Makefile'].includes(n.name)) return 'code';
    if (k === 'text' && FS.ext(n.name) === 'md') return 'md';
    return k;
  };
  const editable = (n) => !n.dir && ['text', 'code', 'md'].includes(viewKind(n));

  const lines = (n, fn) => Array.from({ length: n }, (_, i) => fn(i)).join('\n');
  const stamp = (t) => new Date(t).toISOString().replace('T', ' ').slice(0, 19);

  const TEXTS = {
    'docker-compose.yml': `services:
  castlecommander:
    image: ghcr.io/junkerderprovinz/castlecommander:latest
    container_name: castlecommander
    restart: unless-stopped
    ports:
      - "8443:8080"
    environment:
      - PUID=99
      - PGID=100
      - TZ=Europe/Vienna
    volumes:
      - /mnt/user/appdata/castlecommander:/config
      - /mnt/user:/srv/user
      - /mnt/cache:/srv/cache
      - /boot:/srv/boot:ro

  onlyoffice:
    image: onlyoffice/documentserver:8.2
    restart: unless-stopped
    environment:
      - JWT_SECRET=\${ONLYOFFICE_JWT}
    # Nur im internen Netz, CastleCommander spricht ihn über den Namen an
    networks: [intern]

networks:
  intern:
`,
    'server.go': `package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

// version is set at build time.
var version = "dev"

func main() {
	addr := os.Getenv("LISTEN")
	if addr == "" {
		addr = ":8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/", http.FileServer(http.Dir("/srv")))

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("listening on %s, version %s", addr, version)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
`,
    'notizen.txt': `Server-Notizen

Parity-Check läuft jeden ersten Sonntag im Monat um 3 Uhr.
Disk 2 zeigt 8 umgeschriebene Sektoren, beobachten.
Cache-Pool ist ein Mirror aus zwei 1-TB-NVMe.

Zu erledigen
- USV-Akku tauschen (gekauft am 12.03.2023)
- appdata-Backup auf den Keller-NAS spiegeln
- Plex auf das neue Image umstellen

Ports
8443  CastleCommander
32400 Plex
81    Nginx Proxy Manager
`,
    'beste-bilder.txt': `Für das Fotobuch 2026

2026/IMG_0203.jpg  Sonnenaufgang am See
2026/IMG_0207.jpg  Berge, Blick nach Westen
2026/IMG_0215.jpg  Abendlicht
Urlaub/DSC_0004.jpg  Strand am ersten Tag
Urlaub/DSC_0008.jpg  Hafen bei Nacht
`,
    'setup-notes.md': `# Server neu aufsetzen

Kurze Liste für den Fall, dass der USB-Stick stirbt.

## Vorher

- [x] Flash-Backup liegt in \`/mnt/user/backups\`
- [x] Lizenzschlüssel im Passwortmanager
- [ ] Neuen Stick mit dem **USB Creator** vorbereiten

## Danach

1. Backup auf den neuen Stick entpacken
2. Lizenz auf den neuen Stick übertragen
3. Array starten, Docker-Container prüfen

> Die Disk-Zuordnung steht in \`config/super.dat\`. Ohne die Datei die Platten nach Seriennummer zuordnen.

| Platte | Seriennummer | Rolle |
| --- | --- | --- |
| WD 8 TB | WD-CA1234 | Parity |
| WD 4 TB | WD-CB5678 | Disk 1 |
| Seagate 4 TB | ZDH9QX01 | Disk 2 |

Mehr dazu im [Unraid-Handbuch](https://docs.unraid.net).
`,
    'Einkaufsliste.md': `# Einkaufsliste

- [ ] Milch
- [ ] Brot vom Bäcker
- [x] Kaffeebohnen
- [ ] Batterien AA für die Fernbedienung
- [ ] SATA-Kabel, 50 cm, 2 Stück
`,
    'Notizbuch.md': `# Notizbuch

## Ideen

- Fotos automatisch nach Jahr sortieren
- Backups einmal im Monat testweise zurückspielen

## Bücher

*Der Name des Windes*, angefangen im Oktober.
`,
    'settings.json': `{
  "downloadDir": "/downloads",
  "maxParallel": 3,
  "speedLimit": 0,
  "unpack": true,
  "notify": {
    "onFinish": true,
    "onError": true
  },
  "theme": "system"
}
`,
    'Preferences.xml': `<?xml version="1.0" encoding="utf-8"?>
<Preferences
  FriendlyName="Bottich"
  TranscoderTempDirectory="/transcode"
  ScheduledLibraryUpdatesEnabled="1"
  ScheduledLibraryUpdateInterval="3600"
  LanguageInCloud="1"
  PublishServerOnPlexOnlineKey="0"
/>
`,
    'ident.cfg': `NAME="Bottich"
timeZone="Europe/Vienna"
USE_SSL="auto"
PORT="80"
PORTSSL="443"
LOCAL_TLD="local"
USE_TELNET="no"
USE_SSH="yes"
`,
    go: `#!/bin/bash
# Start the Management Utility
/usr/local/sbin/emhttp &
`,
  };
  TEXTS['aktuell.md'] = TEXTS['Einkaufsliste.md'];
  TEXTS['1.conf'] = TEXTS['44.conf'] = `server {
  set $forward_scheme http;
  set $server         "192.168.10.20";
  set $port           8080;

  listen 80;
  listen 443 ssl;
  server_name dateien.example.org;

  ssl_certificate /etc/letsencrypt/live/npm-1/fullchain.pem;
  ssl_certificate_key /etc/letsencrypt/live/npm-1/privkey.pem;

  client_max_body_size 0;

  location / {
    include conf.d/include/proxy.conf;
  }
}
`;

  const SUBS = ['Hörst du das?', 'Da ist jemand im Wald.', 'Bleib dicht hinter mir.', 'Wir sollten umkehren, bevor es dunkel wird.', 'Nein. Wir gehen weiter.', 'Ich habe so etwas noch nie gesehen.', 'Schnell, hier entlang!', 'Ist es vorbei?', 'Für heute schon.', 'Dann lass uns nach Hause gehen.'];

  function generated(n) {
    const e = FS.ext(n.name), r = rng(n.name);
    if (n.name.startsWith('engine.log')) {
      const start = n.time - 220 * 60000;
      const msgs = ['job started', 'chunk uploaded', 'index refreshed', 'snapshot written', 'retention pass done', 'remote answered slowly', 'job finished'];
      return lines(220, i => {
        const lvl = r() < .06 ? 'WARN' : r() < .015 ? 'ERROR' : 'INFO ';
        return `${stamp(start + i * 60000 + Math.floor(r() * 50000))} ${lvl} ${msgs[Math.floor(r() * msgs.length)]} id=${(1000 + i).toString(16)} took=${Math.floor(r() * 900)}ms`;
      }) + '\n';
    }
    if (e === 'srt') {
      return SUBS.map((s, i) => `${i + 1}\n00:00:${String(4 + i * 4).padStart(2, '0')},200 --> 00:00:${String(7 + i * 4).padStart(2, '0')},400\n${s}\n`).join('\n');
    }
    if (e === 'sha256') return `${Array.from({ length: 64 }, () => '0123456789abcdef'[Math.floor(r() * 16)]).join('')}  ${n.name.replace(/\.sha256$/, '')}\n`;
    if (e === 'md') return `# ${n.name.replace(/\.md$/, '')}\n\nNoch leer.\n`;
    if (e === 'json') return '{\n  "version": 1\n}\n';
    if (e === 'log' || e === '1') return lines(40, i => `${stamp(n.time - (40 - i) * 90000)} INFO  ok`) + '\n';
    return `${n.name}\n`;
  }

  // Edited text lives on the node itself; a link shows and edits the file it points to.
  const target = (n) => (n.link && FS.nodeAt(n.link)) || n;
  const textFor = (n) => { const t = target(n); return t.text ?? TEXTS[t.name] ?? generated(t); };

  // A small markdown renderer, enough for notes, task lists, tables and code.
  function md(src) {
    const inline = (s) => esc(s)
      .replace(/`([^`]+)`/g, '<code>$1</code>')
      .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
      .replace(/\*([^*]+)\*/g, '<em>$1</em>')
      .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');
    const out = [], L = src.replace(/\r/g, '').split('\n');
    for (let i = 0; i < L.length; i++) {
      const l = L[i];
      if (/^```/.test(l)) { const buf = []; while (++i < L.length && !/^```/.test(L[i])) buf.push(L[i]); out.push(`<pre><code>${esc(buf.join('\n'))}</code></pre>`); continue; }
      const h = l.match(/^(#{1,4})\s+(.*)/);
      if (h) { out.push(`<h${h[1].length}>${inline(h[2])}</h${h[1].length}>`); continue; }
      if (/^\s*(-{3,}|\*{3,})\s*$/.test(l)) { out.push('<hr>'); continue; }
      if (/^>\s?/.test(l)) { const buf = []; while (i < L.length && /^>\s?/.test(L[i])) buf.push(L[i++].replace(/^>\s?/, '')); i--; out.push(`<blockquote>${inline(buf.join(' '))}</blockquote>`); continue; }
      if (/^\|/.test(l)) {
        const rows = []; while (i < L.length && /^\|/.test(L[i])) rows.push(L[i++]); i--;
        const cells = (r) => r.replace(/^\||\|$/g, '').split('|').map(c => c.trim());
        const body = rows.filter((r, k) => k !== 1 || !/^[\s|:-]+$/.test(r));
        out.push(`<table><thead><tr>${cells(body[0]).map(c => `<th>${inline(c)}</th>`).join('')}</tr></thead><tbody>${body.slice(1).map(r => `<tr>${cells(r).map(c => `<td>${inline(c)}</td>`).join('')}</tr>`).join('')}</tbody></table>`);
        continue;
      }
      const li = l.match(/^\s*([-*]|\d+\.)\s+(.*)/);
      if (li) {
        const ordered = /\d/.test(li[1]), items = [];
        while (i < L.length) { const m = L[i].match(/^\s*([-*]|\d+\.)\s+(.*)/); if (!m) break; items.push(m[2]); i++; }
        i--;
        out.push(`<${ordered ? 'ol' : 'ul'}>${items.map(t => {
          const task = t.match(/^\[( |x)\]\s+(.*)/i);
          return task ? `<li class="task ${task[1] !== ' ' ? 'done' : ''}"><span class="mark">${task[1] !== ' ' ? g('IconCheck') : ''}</span>${inline(task[2])}</li>` : `<li>${inline(t)}</li>`;
        }).join('')}</${ordered ? 'ol' : 'ul'}>`);
        continue;
      }
      if (!l.trim()) continue;
      const para = [l]; while (i + 1 < L.length && L[i + 1].trim() && !/^(#|>|\||```|\s*([-*]|\d+\.)\s)/.test(L[i + 1])) para.push(L[++i]);
      out.push(`<p>${inline(para.join(' '))}</p>`);
    }
    return out.join('');
  }

  // Code gets a quiet highlight: comments step back, strings and keys a little.
  function highlight(text, kind, ext) {
    const e = esc(text);
    if (kind !== 'code' && !['json', 'xml', 'conf', 'cfg'].includes(ext)) return e;
    if (ext === 'go') return e.replace(/(\/\/.*$)|(&quot;[^&]*?&quot;|"[^"\n]*")|\b(package|import|func|var|const|return|if|else|for|range|type|struct|err|nil)\b/gm, (m, c, s, k) => c ? `<i class="c">${c}</i>` : s ? `<i class="s">${s}</i>` : `<i class="k">${k}</i>`);
    if (['yml', 'yaml'].includes(ext)) return e.replace(/(#.*$)|^(\s*-?\s*)([\w.-]+)(:)|(&quot;.*?&quot;|"[^"\n]*")/gm, (m, c, pre, key, col, s) => c ? `<i class="c">${c}</i>` : s ? `<i class="s">${s}</i>` : `${pre}<i class="k">${key}</i>${col}`);
    if (ext === 'json') return e.replace(/(&quot;[^&]*?&quot;)(\s*:)?/g, (m, s, col) => col ? `<i class="k">${s}</i>${col}` : `<i class="s">${s}</i>`);
    return e.replace(/(#.*$)/gm, '<i class="c">$1</i>').replace(/(&quot;.*?&quot;|"[^"\n]*")/g, '<i class="s">$1</i>');
  }

  // Made-up pictures: a landscape seeded by the file name, so every photo stays the same photo.
  const PALETTES = [
    ['#3b5fa8', '#bfe0f5', '#fff4c4', '#7aa36f', '#4f7d55', '#2e5238', '#6aa6c8'],
    ['#26306b', '#f2a07b', '#ffe0a8', '#6c4f7f', '#46345e', '#271e3b', '#c97a74'],
    ['#14204a', '#e06f55', '#ffc47a', '#7d3d5a', '#4f2a49', '#2a1730', '#b85b55'],
    ['#0b1026', '#33477d', '#f3f1e2', '#29385f', '#1b2645', '#10172c', '#2a3d6a'],
    ['#78ade0', '#eaf4fb', '#ffffff', '#a8bccd', '#71889d', '#43586b', '#9cc3df'],
    ['#4ea1cf', '#fbe6b8', '#fff3c6', '#c7a26b', '#8b9b5a', '#566c3e', '#3f8fbf'],
  ];
  let sceneN = 0;
  function scene(seed, { w = 1600, h = 1200, title = '', sub = '', palette } = {}) {
    const r = rng(seed), p = PALETTES[palette ?? Math.floor(r() * PALETTES.length)], id = 'sc' + (++sceneN);
    const sunX = w * (.2 + r() * .6), sunY = h * (.18 + r() * .25), sunR = Math.min(w, h) * (.05 + r() * .05);
    const water = r() < .4, horizon = h * (water ? .66 : .78);
    const ridge = (base, amp, k) => {
      const n = 7 + Math.floor(r() * 5), pts = Array.from({ length: n + 1 }, (_, i) => [w * i / n, base - amp * (.25 + r() * .75)]);
      let d = `M0 ${h} L0 ${pts[0][1].toFixed(0)}`;
      for (let i = 1; i < pts.length; i++) { const [x0, y0] = pts[i - 1], [x1, y1] = pts[i]; d += ` Q${(x0 + (x1 - x0) / 2).toFixed(0)} ${(Math.min(y0, y1) - amp * .15 * r()).toFixed(0)} ${x1.toFixed(0)} ${y1.toFixed(0)}`; }
      return `<path d="${d} L${w} ${h} Z" fill="${p[k]}"/>`;
    };
    const stars = p[0] === '#0b1026' ? Array.from({ length: 60 }, () => `<circle cx="${(r() * w).toFixed(0)}" cy="${(r() * h * .5).toFixed(0)}" r="${(1 + r() * 2).toFixed(1)}" fill="#fff" opacity="${(.3 + r() * .6).toFixed(2)}"/>`).join('') : '';
    return `<svg viewBox="0 0 ${w} ${h}" preserveAspectRatio="xMidYMid slice" xmlns="http://www.w3.org/2000/svg">
      <defs><linearGradient id="${id}s" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="${p[0]}"/><stop offset="1" stop-color="${p[1]}"/></linearGradient>
      <linearGradient id="${id}w" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="${p[6]}"/><stop offset="1" stop-color="${p[5]}"/></linearGradient>
      <radialGradient id="${id}g"><stop offset="0" stop-color="${p[2]}" stop-opacity=".9"/><stop offset="1" stop-color="${p[2]}" stop-opacity="0"/></radialGradient>
      <linearGradient id="${id}f" x1="0" y1="0" x2="0" y2="1"><stop offset=".55" stop-color="#000" stop-opacity="0"/><stop offset="1" stop-color="#000" stop-opacity=".75"/></linearGradient></defs>
      <rect width="${w}" height="${h}" fill="url(#${id}s)"/>${stars}
      <circle cx="${sunX}" cy="${sunY}" r="${sunR * 3.2}" fill="url(#${id}g)"/><circle cx="${sunX}" cy="${sunY}" r="${sunR}" fill="${p[2]}"/>
      ${ridge(horizon - h * .12, h * .32, 3)}${ridge(horizon - h * .04, h * .22, 4)}${ridge(horizon + h * .04, h * .14, 5)}
      ${water ? `<rect y="${horizon + h * .04}" width="${w}" height="${h}" fill="url(#${id}w)"/><rect x="${sunX - sunR}" y="${horizon + h * .08}" width="${sunR * 2}" height="${h * .006}" fill="${p[2]}" opacity=".7"/><rect x="${sunX - sunR * .7}" y="${horizon + h * .12}" width="${sunR * 1.4}" height="${h * .005}" fill="${p[2]}" opacity=".5"/><rect x="${sunX - sunR * .4}" y="${horizon + h * .16}" width="${sunR * .8}" height="${h * .004}" fill="${p[2]}" opacity=".35"/>` : ''}
      ${title ? `<rect width="${w}" height="${h}" fill="url(#${id}f)"/><text x="${w * .07}" y="${h * .86}" fill="#fff" font-family="Noto Sans, sans-serif" font-weight="700" font-size="${w * .075}">${esc(title)}</text>${sub ? `<text x="${w * .07}" y="${h * .93}" fill="#fff" opacity=".75" font-family="Noto Sans, sans-serif" font-size="${w * .032}" letter-spacing="${w * .004}">${esc(sub)}</text>` : ''}` : ''}
    </svg>`;
  }
  // A photo's size, a poster's title: what a picture shows depends on where it sits.
  const isPoster = (n, folder) => n.name === 'poster.jpg' && /\(\d{4}\)/.test(FS.split(folder).at(-1) || '');
  function picture(n, folder) {
    if (n.url) return `<img src="${esc(n.url)}" alt="">`;
    if (isPoster(n, folder)) return poster(FS.split(folder).at(-1), 1000, 1500);
    const [w, h] = dims(n, folder);
    return scene(n.name + folder, { w: 1600, h: Math.round(1600 * h / w) });
  }
  const filmTitle = (film) => {
    const base = film.replace(/\.(mkv|mp4|webm|mov|jpg)$/i, '').replace(/\./g, ' ');
    const m = base.match(/^(.*?)\s*\(?((?:19|20)\d{2})\)?/);
    return m ? { name: m[1], year: m[2] } : { name: base, year: '' };
  };
  function poster(film, w = 1600, h = 900, titled = true) {
    const { name, year } = filmTitle(film);
    return scene(name, { w, h, title: titled ? name : '', sub: year });
  }
  function dims(n, folder = '') {
    if (isPoster(n, folder)) return [1000, 1500];
    const r = rng(n.name);
    return r() < .2 ? [3024, 4032] : r() < .5 ? [6000, 4000] : [4032, 3024];
  }

  CC.ck = { hash, rng, real, fmtDur, plural, winKeys, viewKind, editable, target, textFor, md, highlight, scene, picture, poster, filmTitle, dims, SUBS };
})();
