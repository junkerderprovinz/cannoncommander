// The list's columns. Which ones show and in what order is a pref; values the data lacks are made up from the name.
const hashOf = (s) => { let h = 2166136261; for (const c of s) h = Math.imul(h ^ c.charCodeAt(0), 16777619); return h >>> 0; };
const pick = (k, list) => list[hashOf(k.name) % list.length];
const TYPE_NAMES = { img: 'Bild', video: 'Video', audio: 'Audio', arch: 'Archiv', disk: 'Abbild', pdf: 'PDF-Dokument', office: 'Office-Dokument', text: 'Textdatei', code: 'Quelltext', file: 'Datei' };
const RIGHTS = (mode = '0664') => [...mode.slice(-3)].map(d => ['---', '--x', '-w-', '-wx', 'r--', 'r-x', 'rw-', 'rwx'][+d]).join('');
const h = (k, salt) => hashOf(k.name + salt);
const hex = (k, salt, n) => (h(k, salt).toString(16) + h(k, salt + salt).toString(16) + h(k, salt + '_').toString(16)).padEnd(n, '0').slice(0, n);
const stem = (k) => k.name.replace(/\.[^.]+$/, '');
const when = (test, fn) => (k, path) => test(k) ? fn(k, path) : '';
const photo = (k) => k.kind === 'img' && /^(IMG|DSC)/.test(k.name);
const doc = (k) => ['pdf', 'office'].includes(k.kind);
const prog = (k) => ['exe', 'msi', 'apk', 'deb', 'rpm', 'appimage', 'dmg'].includes(FS.ext(k.name));
const unpacked = (k) => Math.round((k.size || 0) * (1.2 + (h(k, 'u') % 30) / 10));
const KIND_NAMES = { img: 'Bild', video: 'Video', audio: 'Musik', arch: 'Archiv', disk: 'Abbild', pdf: 'Dokument', office: 'Dokument', text: 'Text', code: 'Quelltext' };
const CAMERA_MAKER = { 'Pixel 9': 'Google', 'iPhone 16': 'Apple', 'Sony A7 IV': 'Sony', 'Fujifilm X-T5': 'Fujifilm' };
const MIME = {
  jpg: 'image/jpeg', jpeg: 'image/jpeg', png: 'image/png', webp: 'image/webp', heic: 'image/heic', gif: 'image/gif',
  mkv: 'video/x-matroska', mp4: 'video/mp4', webm: 'video/webm', mov: 'video/quicktime',
  mp3: 'audio/mpeg', flac: 'audio/flac', ogg: 'audio/ogg', wav: 'audio/wav',
  zip: 'application/zip', '7z': 'application/x-7z-compressed', rar: 'application/vnd.rar', tar: 'application/x-tar', gz: 'application/gzip', zst: 'application/zstd',
  iso: 'application/x-iso9660-image', pdf: 'application/pdf', odt: 'application/vnd.oasis.opendocument.text',
  docx: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', xlsx: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  md: 'text/markdown', txt: 'text/plain', log: 'text/plain', json: 'application/json', xml: 'application/xml', yml: 'application/yaml', yaml: 'application/yaml',
  sh: 'application/x-sh', js: 'text/javascript', ts: 'text/typescript', go: 'text/x-go',
};
const mmss = (s) => `${Math.floor(s / 3600) ? Math.floor(s / 3600) + ':' : ''}${String(Math.floor(s / 60) % 60).padStart(Math.floor(s / 3600) ? 2 : 1, '0')}:${String(s % 60).padStart(2, '0')}`;

const COLUMNS = [
  { id: 'name', label: 'Name', group: 'Allgemein', width: 'minmax(0, 1fr)', min: 14, sort: 'name', fixed: true },
  { id: 'size', label: 'Größe', group: 'Allgemein', width: '6.5rem', r: true, sort: 'size', value: (k) => k.dir ? (CC.prefs.folderSizes ? fmtSize(FS.totalSize(k)) || '0 B' : `${FS.kids(k).length} Obj.`) : fmtSize(k.size) },
  { id: 'modified', label: 'Geändert', group: 'Allgemein', width: '7.5rem', r: true, sort: 'time', value: (k) => fmtListDate(k.time) },
  { id: 'created', label: 'Erstellt', group: 'Allgemein', width: '7.5rem', r: true, num: (k) => k.time - (hashOf(k.name) % 400) * DAY, value: (k) => fmtListDate(k.time - (hashOf(k.name) % 400) * DAY) },
  { id: 'accessed', label: 'Letzter Zugriff', group: 'Allgemein', width: '7.5rem', r: true, num: (k) => Math.min(NOW, k.time + (hashOf(k.name) % 60) * DAY), value: (k) => fmtListDate(Math.min(NOW, k.time + (hashOf(k.name) % 60) * DAY)) },
  { id: 'type', label: 'Typ', group: 'Allgemein', width: '8.5rem', sort: 'type', value: (k) => k.dir ? 'Ordner' : k.link ? 'Verknüpfung' : (FS.ext(k.name).toUpperCase() ? `${TYPE_NAMES[k.kind] || 'Datei'}` : 'Datei') },
  { id: 'ext', label: 'Endung', group: 'Allgemein', width: '5rem', value: (k) => k.dir ? '' : FS.ext(k.name) },
  { id: 'items', label: 'Elemente', group: 'Allgemein', width: '5.5rem', r: true, num: (k) => k.dir ? FS.kids(k).length : -1, value: (k) => k.dir ? String(FS.kids(k).length) : '' },
  { id: 'path', label: 'Ordner', group: 'Allgemein', width: '11rem', value: (k, path) => path },
  { id: 'link', label: 'Verknüpfungsziel', group: 'Allgemein', width: '11rem', value: (k) => k.link || '' },
  { id: 'owner', label: 'Besitzer', group: 'Rechte', width: '6rem', value: (k) => k.owner || 'nobody' },
  { id: 'group', label: 'Gruppe', group: 'Rechte', width: '6rem', value: (k) => k.group || 'users' },
  { id: 'rights', label: 'Rechte', group: 'Rechte', width: '7rem', mono: true, value: (k) => (k.dir ? 'd' : '-') + RIGHTS(k.mode) },
  { id: 'octal', label: 'Oktal', group: 'Rechte', width: '4.5rem', mono: true, value: (k) => (k.mode || '0664').slice(-3) },
  { id: 'dimensions', label: 'Abmessungen', group: 'Medien', width: '7.5rem', r: true, value: (k) => k.kind === 'img' ? pick(k, ['4032 × 3024', '3024 × 4032', '6000 × 4000', '1920 × 1080']) : k.kind === 'video' ? pick(k, ['1920 × 1080', '3840 × 2160', '1280 × 720']) : '' },
  { id: 'duration', label: 'Dauer', group: 'Medien', width: '5.5rem', r: true, num: (k) => ['video', 'audio'].includes(k.kind) ? hashOf(k.name) % (k.kind === 'video' ? 7200 : 420) + 60 : -1, value: (k) => ['video', 'audio'].includes(k.kind) ? mmss(hashOf(k.name) % (k.kind === 'video' ? 7200 : 420) + 60) : '' },
  { id: 'artist', label: 'Interpret', group: 'Musik', width: '8rem', value: (k) => k.kind === 'audio' ? pick(k, ['Kevin MacLeod', 'Josh Woodward', 'Chris Zabriskie']) : '' },
  { id: 'album', label: 'Album', group: 'Musik', width: '8rem', value: (k) => k.kind === 'audio' ? pick(k, ['Calming', 'Comedy', 'Electronica']) : '' },
  { id: 'camera', label: 'Kameramodell', group: 'Fotos', width: '8rem', value: (k) => k.kind === 'img' && /^(IMG|DSC)/.test(k.name) ? pick(k, ['Pixel 9', 'iPhone 16', 'Sony A7 IV', 'Fujifilm X-T5']) : '' },
  { id: 'taken', label: 'Aufgenommen', group: 'Fotos', width: '7.5rem', r: true, num: (k) => k.kind === 'img' ? k.time - DAY : -1, value: (k) => k.kind === 'img' && /^(IMG|DSC)/.test(k.name) ? fmtListDate(k.time - DAY) : '' },
  { id: 'pages', label: 'Seiten', group: 'Dokumente', width: '4.5rem', r: true, num: (k) => ['pdf', 'office'].includes(k.kind) ? hashOf(k.name) % 40 + 1 : -1, value: (k) => ['pdf', 'office'].includes(k.kind) ? String(hashOf(k.name) % 40 + 1) : '' },
  { id: 'sha256', label: 'SHA-256', group: 'Prüfsumme', width: '9rem', mono: true, value: (k) => k.dir ? '' : hex(k, 'x', 12) },
  { id: 'disksize', label: 'Größe auf Datenträger', group: 'Allgemein', width: '7rem', r: true, num: (k) => k.dir ? -1 : Math.ceil((k.size || 0) / 4096) * 4096, value: (k) => k.dir ? '' : fmtSize(Math.ceil((k.size || 0) / 4096) * 4096) || '0 B' },
  { id: 'kind', label: 'Art', group: 'Allgemein', width: '6.5rem', value: (k) => k.dir ? 'Ordner' : KIND_NAMES[k.kind] || 'Sonstiges' },
  { id: 'mime', label: 'MIME-Typ', group: 'Allgemein', width: '10rem', mono: true, value: (k) => k.dir ? 'inode/directory' : MIME[FS.ext(k.name)] || 'application/octet-stream' },
  { id: 'attributes', label: 'Attribute', group: 'Allgemein', width: '5rem', mono: true, value: (k) => [k.hidden || k.name.startsWith('.') ? 'H' : '', k.link ? 'L' : '', +(k.mode || '0664').slice(-3)[0] < 6 ? 'R' : '', !k.dir && h(k, 'a') % 5 === 0 ? 'A' : ''].join('') },
  { id: 'statchange', label: 'Statusänderung', group: 'Allgemein', width: '7.5rem', r: true, num: (k) => Math.min(NOW, k.time + (h(k, 'c') % 3) * DAY), value: (k) => fmtListDate(Math.min(NOW, k.time + (h(k, 'c') % 3) * DAY)) },
  { id: 'disk', label: 'Laufwerk', group: 'Speicher', width: '6rem', value: (k, path) => path.startsWith('/mnt/user') ? pick(k, ['disk1', 'disk2', 'disk3', 'cache']) : FS.LABEL[FS.volumeOf(path)] || '' },
  { id: 'share', label: 'Freigabe', group: 'Speicher', width: '7rem', value: (k, path) => path.startsWith('/mnt/user/') ? FS.split(path)[2] : path === '/mnt/user' && k.dir ? k.name : '' },
  { id: 'hardlinks', label: 'Harte Links', group: 'Speicher', width: '5.5rem', r: true, num: (k) => k.dir ? 2 : 1 + (h(k, 'l') % 9 === 0), value: (k) => String(k.dir ? 2 : 1 + (h(k, 'l') % 9 === 0)) },
  { id: 'inode', label: 'Inode', group: 'Speicher', width: '6.5rem', r: true, mono: true, num: (k) => h(k, 'i') % 9000000, value: (k) => String(h(k, 'i') % 9000000) },
  { id: 'blocks', label: 'Blöcke', group: 'Speicher', width: '5.5rem', r: true, num: (k) => k.dir ? -1 : Math.ceil((k.size || 0) / 512), value: (k) => k.dir ? '' : String(Math.ceil((k.size || 0) / 512)) },
  { id: 'uid', label: 'UID', group: 'Rechte', width: '4rem', r: true, mono: true, value: (k) => k.owner === 'root' ? '0' : '99' },
  { id: 'gid', label: 'GID', group: 'Rechte', width: '4rem', r: true, mono: true, value: (k) => k.group === 'root' ? '0' : '100' },
  { id: 'special', label: 'Sonderrechte', group: 'Rechte', width: '6.5rem', value: (k) => k.dir && h(k, 's') % 7 === 0 ? 'Sticky' : '' },
  { id: 'vcodec', label: 'Videocodec', group: 'Video', width: '6rem', value: when(k => k.kind === 'video', k => pick(k, ['H.264', 'HEVC', 'AV1', 'VP9'])) },
  { id: 'acodec', label: 'Audiocodec', group: 'Video', width: '6rem', value: when(k => ['video', 'audio'].includes(k.kind), k => k.kind === 'audio' ? FS.ext(k.name).toUpperCase() : ['AAC', 'AC-3', 'E-AC-3', 'DTS', 'Opus'][h(k, 'ac') % 5]) },
  { id: 'framerate', label: 'Bildrate', group: 'Video', width: '5.5rem', r: true, value: when(k => k.kind === 'video', k => pick(k, ['23,976 fps', '25 fps', '29,97 fps', '50 fps', '60 fps'])) },
  { id: 'vbitrate', label: 'Datenrate', group: 'Video', width: '6.5rem', r: true, num: (k) => k.kind === 'video' ? h(k, 'v') % 18000 + 2000 : -1, value: when(k => k.kind === 'video', k => `${(h(k, 'v') % 18000 + 2000).toLocaleString('de')} kBit/s`) },
  { id: 'aspect', label: 'Seitenverhältnis', group: 'Video', width: '6rem', r: true, value: when(k => k.kind === 'video', k => pick(k, ['16:9', '21:9', '4:3'])) },
  { id: 'hdr', label: 'HDR', group: 'Video', width: '5.5rem', value: when(k => k.kind === 'video', k => pick(k, ['SDR', 'HDR10', 'Dolby Vision', 'SDR'])) },
  { id: 'audiotracks', label: 'Tonspuren', group: 'Video', width: '8rem', value: when(k => k.kind === 'video', k => pick(k, ['Deutsch', 'Deutsch, Englisch', 'Englisch', 'Deutsch, Englisch, Französisch'])) },
  { id: 'subtitles', label: 'Untertitel', group: 'Video', width: '7rem', value: when(k => k.kind === 'video', k => pick(k, ['', 'Deutsch', 'Deutsch, Englisch', 'Englisch'])) },
  { id: 'title', label: 'Titel', group: 'Musik', width: '9rem', value: when(k => k.kind === 'audio', k => stem(k).replace(/^\d+[\s._-]*/, '')) },
  { id: 'albumartist', label: 'Albuminterpret', group: 'Musik', width: '8rem', value: when(k => k.kind === 'audio', k => pick(k, ['Kevin MacLeod', 'Josh Woodward', 'Verschiedene'])) },
  { id: 'track', label: 'Nr.', group: 'Musik', width: '3.5rem', r: true, num: (k) => k.kind === 'audio' ? h(k, 't') % 14 + 1 : -1, value: when(k => k.kind === 'audio', k => String(h(k, 't') % 14 + 1)) },
  { id: 'year', label: 'Jahr', group: 'Musik', width: '4rem', r: true, value: when(k => ['audio', 'video'].includes(k.kind), k => String(1975 + h(k, 'y') % 51)) },
  { id: 'genre', label: 'Genre', group: 'Musik', width: '6.5rem', value: when(k => k.kind === 'audio', k => pick(k, ['Ambient', 'Jazz', 'Rock', 'Klassik', 'Elektronisch'])) },
  { id: 'composer', label: 'Komponist', group: 'Musik', width: '8rem', value: when(k => k.kind === 'audio', k => pick(k, ['Kevin MacLeod', 'Josh Woodward', ''])) },
  { id: 'abitrate', label: 'Bitrate', group: 'Musik', width: '6rem', r: true, value: when(k => k.kind === 'audio', k => FS.ext(k.name) === 'flac' ? `${900 + h(k, 'b') % 500} kBit/s` : pick(k, ['192 kBit/s', '256 kBit/s', '320 kBit/s'])) },
  { id: 'samplerate', label: 'Abtastrate', group: 'Musik', width: '6rem', r: true, value: when(k => ['audio', 'video'].includes(k.kind), k => pick(k, ['44,1 kHz', '48 kHz', '96 kHz'])) },
  { id: 'channels', label: 'Kanäle', group: 'Musik', width: '5rem', value: when(k => ['audio', 'video'].includes(k.kind), k => k.kind === 'video' ? pick(k, ['Stereo', '5.1', '7.1']) : pick(k, ['Stereo', 'Stereo', 'Mono'])) },
  { id: 'make', label: 'Kamerahersteller', group: 'Fotos', width: '7rem', value: when(photo, k => CAMERA_MAKER[COL.camera.value(k)]) },
  { id: 'lens', label: 'Objektiv', group: 'Fotos', width: '9rem', value: when(photo, k => pick(k, ['24-70 mm F2.8', '35 mm F1.4', 'Weitwinkel 6,7 mm', 'Hauptkamera 6,9 mm'])) },
  { id: 'focal', label: 'Brennweite', group: 'Fotos', width: '5.5rem', r: true, value: when(photo, k => `${[24, 28, 35, 50, 85][h(k, 'f') % 5]} mm`) },
  { id: 'aperture', label: 'Blende', group: 'Fotos', width: '4.5rem', r: true, value: when(photo, k => ['f/1,8', 'f/2,8', 'f/4', 'f/8'][h(k, 'a') % 4]) },
  { id: 'exposure', label: 'Belichtungszeit', group: 'Fotos', width: '6rem', r: true, value: when(photo, k => ['1/60 s', '1/125 s', '1/250 s', '1/1000 s'][h(k, 'e') % 4]) },
  { id: 'iso', label: 'ISO', group: 'Fotos', width: '4.5rem', r: true, num: (k) => photo(k) ? [100, 200, 400, 800, 1600][h(k, 'iso') % 5] : -1, value: when(photo, k => String([100, 200, 400, 800, 1600][h(k, 'iso') % 5])) },
  { id: 'flash', label: 'Blitz', group: 'Fotos', width: '6rem', value: when(photo, k => ['Kein Blitz', 'Kein Blitz', 'Ausgelöst'][h(k, 'b') % 3]) },
  { id: 'place', label: 'Ort', group: 'Fotos', width: '8rem', value: when(photo, k => ['Wien', 'Salzburg', 'Graz', '', 'Innsbruck'][h(k, 'o') % 5]) },
  { id: 'orientation', label: 'Ausrichtung', group: 'Fotos', width: '6rem', value: (k) => k.kind === 'img' ? (COL.dimensions.value(k).startsWith('3024') ? 'Hochformat' : 'Querformat') : '' },
  { id: 'depth', label: 'Farbtiefe', group: 'Fotos', width: '5rem', r: true, value: when(k => k.kind === 'img', k => FS.ext(k.name) === 'png' ? '32 Bit' : '24 Bit') },
  { id: 'dpi', label: 'Auflösung', group: 'Fotos', width: '5.5rem', r: true, value: when(k => k.kind === 'img', k => ['72 dpi', '96 dpi', '300 dpi'][h(k, 'd') % 3]) },
  { id: 'author', label: 'Autor', group: 'Dokumente', width: '7rem', value: when(doc, k => pick(k, ['jdp', 'Buchhaltung', 'Verwaltung'])) },
  { id: 'lastauthor', label: 'Zuletzt gespeichert von', group: 'Dokumente', width: '8rem', value: when(doc, k => ['jdp', 'Buchhaltung'][h(k, 'l') % 2]) },
  { id: 'doctitle', label: 'Dokumenttitel', group: 'Dokumente', width: '9rem', value: when(doc, k => stem(k).replace(/[-_]/g, ' ')) },
  { id: 'subject', label: 'Betreff', group: 'Dokumente', width: '8rem', value: when(doc, k => ['', 'Rechnung', 'Vertrag', 'Angebot'][h(k, 's') % 4]) },
  { id: 'words', label: 'Wörter', group: 'Dokumente', width: '5rem', r: true, num: (k) => doc(k) ? h(k, 'w') % 12000 + 80 : -1, value: when(doc, k => (h(k, 'w') % 12000 + 80).toLocaleString('de')) },
  { id: 'lines', label: 'Zeilen', group: 'Dokumente', width: '5rem', r: true, num: (k) => ['text', 'code'].includes(k.kind) ? h(k, 'n') % 3000 + 1 : -1, value: when(k => ['text', 'code'].includes(k.kind), k => (h(k, 'n') % 3000 + 1).toLocaleString('de')) },
  { id: 'encoding', label: 'Kodierung', group: 'Dokumente', width: '6.5rem', value: when(k => ['text', 'code'].includes(k.kind), k => ['UTF-8', 'UTF-8', 'ASCII', 'UTF-8 mit BOM'][h(k, 'k') % 4]) },
  { id: 'app', label: 'Programm', group: 'Dokumente', width: '8rem', value: when(doc, k => k.kind === 'pdf' ? ['LibreOffice', 'Microsoft Word', 'Scanner'][h(k, 'p') % 3] : ['LibreOffice', 'Microsoft Office'][h(k, 'p') % 2]) },
  { id: 'encrypted', label: 'Verschlüsselt', group: 'Dokumente', width: '5.5rem', value: when(doc, k => h(k, 'x') % 6 === 0 ? 'Ja' : 'Nein') },
  { id: 'unpacked', label: 'Entpackte Größe', group: 'Archive', width: '7rem', r: true, num: (k) => k.kind === 'arch' ? unpacked(k) : -1, value: when(k => k.kind === 'arch', k => fmtSize(unpacked(k))) },
  { id: 'ratio', label: 'Kompressionsrate', group: 'Archive', width: '6rem', r: true, value: when(k => k.kind === 'arch', k => `${Math.round(100 - 100 * (k.size || 0) / unpacked(k))} %`) },
  { id: 'entries', label: 'Dateien im Archiv', group: 'Archive', width: '6rem', r: true, num: (k) => k.kind === 'arch' ? (k.zip ? k.zip.length : h(k, 'z') % 400 + 2) : -1, value: when(k => k.kind === 'arch', k => String(k.zip ? k.zip.length : h(k, 'z') % 400 + 2)) },
  { id: 'method', label: 'Packverfahren', group: 'Archive', width: '6.5rem', value: when(k => k.kind === 'arch', k => ({ zip: 'Deflate', '7z': 'LZMA2', rar: 'RAR5', zst: 'Zstandard', gz: 'Gzip', tar: 'Ohne' })[FS.ext(k.name)]) },
  { id: 'archcrypt', label: 'Passwortschutz', group: 'Archive', width: '6rem', value: when(k => k.kind === 'arch', k => h(k, 'p') % 5 === 0 ? 'Ja' : 'Nein') },
  { id: 'fs', label: 'Dateisystem', group: 'Abbilder', width: '6rem', value: when(k => k.kind === 'disk', k => FS.ext(k.name) === 'iso' ? 'ISO 9660' : ['FAT32', 'ext4', 'NTFS'][h(k, 'f') % 3]) },
  { id: 'volume', label: 'Bezeichnung', group: 'Abbilder', width: '9rem', mono: true, value: when(k => k.kind === 'disk', k => stem(k).toUpperCase().replace(/[^A-Z0-9]+/g, '_').slice(0, 16)) },
  { id: 'bootable', label: 'Startfähig', group: 'Abbilder', width: '5.5rem', value: when(k => k.kind === 'disk', k => FS.ext(k.name) === 'iso' ? 'Ja' : 'Nein') },
  { id: 'version', label: 'Version', group: 'Programme', width: '5.5rem', r: true, value: when(prog, k => `${h(k, 'v') % 9 + 1}.${h(k, 'w') % 20}.${h(k, 'x') % 10}`) },
  { id: 'vendor', label: 'Hersteller', group: 'Programme', width: '8rem', value: when(prog, k => pick(k, ['Debian', 'Mozilla', 'VideoLAN', 'Lime Technology'])) },
  { id: 'arch', label: 'Architektur', group: 'Programme', width: '6rem', value: when(prog, k => ['x86-64', 'x86-64', 'ARM64'][h(k, 'r') % 3]) },
  { id: 'signed', label: 'Signiert', group: 'Programme', width: '5rem', value: when(prog, k => h(k, 'g') % 3 ? 'Ja' : 'Nein') },
  { id: 'crc32', label: 'CRC32', group: 'Prüfsumme', width: '6rem', mono: true, value: (k) => k.dir ? '' : hex(k, 'c', 8) },
  { id: 'md5', label: 'MD5', group: 'Prüfsumme', width: '9rem', mono: true, value: (k) => k.dir ? '' : hex(k, 'm', 12) },
  { id: 'sha1', label: 'SHA-1', group: 'Prüfsumme', width: '9rem', mono: true, value: (k) => k.dir ? '' : hex(k, 's', 12) },
];
const COL = Object.fromEntries(COLUMNS.map(c => [c.id, c]));
const DEFAULT_COLUMNS = ['name', 'size', 'modified'];
const COLUMN_GROUPS = ['Allgemein', 'Speicher', 'Rechte', 'Medien', 'Video', 'Musik', 'Fotos', 'Dokumente', 'Archive', 'Abbilder', 'Programme', 'Prüfsumme'];
const GROUP_GLYPH = { Allgemein: 'IconFile', Speicher: 'IconDatabase', Rechte: 'IconKey', Medien: 'IconFileImage', Video: 'IconFileVideo', Musik: 'IconFileAudio', Fotos: 'IconFileRaw', Dokumente: 'IconFileDoc', Archive: 'IconArchive', Abbilder: 'IconDisc', Programme: 'IconFileProgram', 'Prüfsumme': 'IconHash' };

const Columns = {
  shown: () => (CC.prefs.columns || DEFAULT_COLUMNS).filter(id => COL[id]),
  // A phone has room for the name and the size; the chosen columns stay as they are for wider screens.
  narrow: () => matchMedia('(max-width: 760px)').matches,
  visible: () => Columns.narrow() ? Columns.shown().filter(id => id === 'name' || id === 'size') : Columns.shown(),
  width: (id) => CC.prefs.colWidths?.[id] ? `${CC.prefs.colWidths[id]}rem` : COL[id].width,
  template: () => Columns.visible().map(Columns.width).join(' '),
  // Every row is its own grid, so they line up only when all share one width; below it the list scrolls sideways.
  minWidth: () => {
    if (Columns.narrow()) return '0';
    const ids = Columns.visible();
    const rem = ids.reduce((a, id) => a + (COL[id].min || parseFloat(Columns.width(id))), 0) + (ids.length - 1) + 1;
    return `${rem}rem`;
  },
  set(ids) { CC.setPref('columns', ids.includes('name') ? ids : ['name', ...ids]); },
  reset() { CC.prefs.colWidths = {}; Columns.set(DEFAULT_COLUMNS); },
  toggle(id) {
    const cur = Columns.shown();
    if (COL[id].fixed) return;
    Columns.set(cur.includes(id) ? cur.filter(x => x !== id) : [...cur, id]);
  },
  // Sorting by any column: the four classic keys keep their own comparers, the rest compare numbers or text.
  comparer(sort) {
    const c = COLUMNS.find(x => x.id === sort || x.sort === sort);
    if (!c || ['name', 'size', 'time', 'type'].includes(sort)) return null;
    if (c.num) return (a, b) => c.num(a) - c.num(b);
    return (a, b) => String(c.value(a, '')).localeCompare(String(c.value(b, '')), 'de', { numeric: true });
  },
  sortKey: (c) => c.sort || c.id,
  menu(x, y) {
    const cur = Columns.shown();
    menu(x, y, [
      ...COLUMN_GROUPS.map(gr => ({
        label: gr, g: GROUP_GLYPH[gr],
        sub: COLUMNS.filter(c => c.group === gr && !c.fixed).map(c => ({ label: c.label, checked: () => cur.includes(c.id), run: () => Columns.toggle(c.id) })),
      })),
      '-', { label: 'Spalten wählen', g: 'IconSliders', run: () => CC.run('columns') }, { label: 'Standard wiederherstellen', g: 'IconRefresh', run: () => Columns.reset() },
    ]);
  },
};

CC.action('columns', { label: 'Spalten wählen', g: 'IconSliders', run: () => {
  const cur = Columns.shown();
  const box = (c) => `<button class="col-check" type="button" role="checkbox" aria-checked="${cur.includes(c.id)}" data-col="${c.id}" ${c.fixed ? 'disabled' : ''}><span class="cbox ${cur.includes(c.id) ? 'on' : ''}">${g('IconCheck')}</span>${esc(c.label)}</button>`;
  const w = win({ title: 'Spalten', wide: true,
    body: `<input class="field grow" id="col-q" type="search" placeholder="Spalte suchen, etwa Codec oder ISO" autocomplete="off">
      <div class="well"><div class="col-cols">${COLUMN_GROUPS.map(gr => `<section class="col-group" data-colgroup><div class="eyebrow">${esc(gr)}</div>${COLUMNS.filter(c => c.group === gr).map(box).join('')}</section>`).join('')}</div>
      <div class="empty" id="col-none" hidden>${g('IconSearch')}<div>Keine Spalte passt</div></div></div>`,
    foot: [{ start: 'Im Spaltenkopf verschiebst du Spalten mit der Maus, am rechten Rand ziehst du die Breite' }, { label: 'Standard wiederherstellen', g: 'IconRefresh', run: () => Columns.reset() }, { label: 'Übernehmen', g: 'IconApply', primary: true, run: (w) => {
      const on = $$('[data-col][aria-checked="true"]', w).map(b => b.dataset.col);
      Columns.set([...cur.filter(id => on.includes(id)), ...on.filter(id => !cur.includes(id))]);
    } }],
  });
  w.addEventListener('click', e => {
    const b = e.target.closest('.col-check');
    if (!b || b.disabled) return;
    const on = b.getAttribute('aria-checked') !== 'true';
    b.setAttribute('aria-checked', on);
    b.querySelector('.cbox').classList.toggle('on', on);
  });
  $('#col-q', w).addEventListener('input', e => {
    const q = e.target.value.trim().toLowerCase();
    $$('[data-colgroup]', w).forEach(gr => {
      const groupHit = gr.querySelector('.eyebrow').textContent.toLowerCase().includes(q);
      $$('.col-check', gr).forEach(b => { b.hidden = !!q && !groupHit && !b.textContent.toLowerCase().includes(q); });
      gr.hidden = !!q && !gr.querySelector('.col-check:not([hidden])');
    });
    $('#col-none', w).hidden = $$('[data-colgroup]', w).some(gr => !gr.hidden);
  });
  $('#col-q', w).focus();
} });
CC.css(`#col-q { flex: none; }
.col-cols { columns: 13rem; column-gap: 2rem; }
.col-group { break-inside: avoid; display: flex; flex-direction: column; padding-bottom: 1.1rem; }
.col-group .eyebrow { margin-bottom: .25rem; }
.col-check { display: flex; align-items: center; gap: .6rem; min-height: 2rem; padding: 0 .4rem; border-radius: var(--radius-control); text-align: left; }
.col-check:hover { background: var(--carbon-hover); }
.col-check:disabled { opacity: .55; cursor: default; }
.col-check:disabled:hover { background: none; }
.col-cols [hidden], #col-none[hidden] { display: none; }`);
