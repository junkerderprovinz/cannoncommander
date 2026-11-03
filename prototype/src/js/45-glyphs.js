// Every glyph GlimStone has for folders, places and files, with what CastleCommander uses it for, and the kinds still without one.
const GLYPH_SETS = [
  ['Ordner und Orte', [
    ['IconFolder', 'Ordner', 'Jeder Ordner in Liste, Baum und Fenstern'],
    ['IconFolderOpen', 'Geöffneter Ordner', 'Aufgeklappter Ordner im Baum, die Marke'],
    ['IconFolderUp', 'Eine Ebene hoch', 'Die Zeile „..“'],
    ['IconFolderAdd', 'Neuer Ordner', 'F7 und das Datei-Menü'],
    ['IconFiles', 'Dateien und Ordner', 'Dateiliste in den Einstellungen, untere Leiste'],
    ['IconZFS', 'Platte, Share, Pool', 'Shares, Cache und Disks im Baum'],
    ['IconDatabase', 'Datenbank, Array', 'Der Server im Baum, Datenbanken, Abbilder'],
    ['IconFlash', 'USB-Stick', 'Der Boot-Stick'],
    ['IconCloud', 'Fernziele', 'Der Ordner der rclone-Ziele'],
    ['IconNetwork', 'Netzwerkziel', 'Ein SMB- oder SFTP-Ziel'],
    ['IconLocal', 'Lokaler Speicher', 'Noch nicht benutzt'],
    ['IconTrash', 'Papierkorb', 'Papierkorb und Löschen'],
  ]],
  ['Dateien', [
    ['IconFile', 'Datei', 'Alles ohne eigenen Glyphen'],
    ['IconFileText', 'Textdatei', 'Text und Markdown, im Moment auch PDF und Office'],
    ['IconFileImage', 'Bild', 'JPG, PNG, HEIC und andere Bilder'],
    ['IconFileVideo', 'Video', 'MKV, MP4 und andere Videos'],
    ['IconFileAudio', 'Audio', 'MP3, FLAC und andere Musik'],
    ['IconFileSubtitles', 'Untertitel', 'SRT, ASS, VTT'],
    ['IconCode', 'Quelltext', 'Go, Shell, YAML, JavaScript'],
    ['IconConfig', 'Konfiguration', 'JSON, XML, CFG, CONF, INI'],
    ['IconLog', 'Logdatei', '.log und rotierte Logs'],
    ['IconArchive', 'Archiv', 'ZIP, TAR, 7z, RAR'],
    ['IconKey', 'Schlüssel', 'Noch nicht benutzt, gut für .pem und .key'],
    ['IconLink', 'Verknüpfung', 'Noch nicht benutzt, gut für Symlinks'],
  ]],
];
const GLYPH_MISSING = [
  ['PDF', 'IconFileText'], ['Textdokument (DOCX, ODT)', 'IconFileText'], ['Tabelle (XLSX, ODS, CSV)', 'IconFileText'],
  ['Präsentation (PPTX, ODP)', 'IconFileText'], ['Abbild (ISO, IMG)', 'IconDatabase'], ['Prüfsumme (SHA256, MD5)', 'IconFileText'],
  ['Schrift (TTF, OTF)', 'IconFile'], ['E-Book (EPUB)', 'IconFile'], ['Programm (EXE, AppImage)', 'IconFile'],
];

CC.registerView('glyphs', {
  page: true,
  render(host) {
    const tile = ([name, label, use]) => `<div class="gl-tile"><span class="gl-g">${g(name)}</span><div><b>${esc(label)}</b><div class="gl-use">${esc(use)}</div><div class="gl-name mono">${name}</div></div></div>`;
    host.innerHTML = `<div class="page-inner gl">
      ${GLYPH_SETS.map(([title, list]) => CC.card(title, `<div class="gl-grid">${list.map(tile).join('')}</div>`)).join('')}
      ${CC.card('Fehlt noch in GlimStone', `<div class="gl-grid">${GLYPH_MISSING.map(([what, now]) => `<div class="gl-tile"><span class="gl-g muted-g">${g(now)}</span><div><b>${esc(what)}</b><div class="gl-use">Nutzt im Moment ${esc(now)}</div></div></div>`).join('')}</div>`)}
    </div>`;
  },
});
CC.action('glyphs', { label: 'Glyphen für Dateien', g: 'IconFiles', run: () => CC.go('glyphs') });
CC.css(`
.gl-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(17rem, 1fr)); gap: .5rem 1.25rem; }
.gl-tile { display: flex; gap: .85rem; align-items: flex-start; padding: .55rem .25rem; }
.gl-g svg { width: 2rem; height: 2rem; color: var(--accent-glyph); }
.gl-g.muted-g svg { color: var(--carbon-text-muted); }
.gl-use { font-size: 13px; color: var(--carbon-text-sub); }
.gl-name { font-size: 12px; color: var(--carbon-text-muted); margin-top: .15rem; }
`);
