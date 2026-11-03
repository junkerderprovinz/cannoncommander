// Aufräumen finds what a server collects over time, and Archiv testen reads archives through without unpacking them.
(() => {
  const { closeBtn, frow } = CC.ui;
  const bk = CC.bk;
  const AGO = (d) => NOW - d * DAY;

  // A few leftovers of the usual kinds, so the tool has something to find in the prototype.
  const unfertig = FS.nodeAt('/mnt/user/downloads/unfertig');
  unfertig.children.push(
    bk.dirNode('alt', [bk.dirNode('leer')], { time: AGO(90) }),
    bk.fileNode('.DS_Store', 6 * KB, { time: AGO(40) }),
    bk.fileNode('Thumbs.db', 18 * KB, { time: AGO(120) }),
    bk.fileNode('Elephants Dream (2006).mkv.!qB', 312 * MB, { time: AGO(3) }),
    bk.fileNode('~$Bewerbung.docx', 162, { time: AGO(1) }),
    bk.fileNode('notizen-leer.txt', 0, { time: AGO(14) }),
    bk.fileNode('Urlaub-007.jpg', 3.1 * MB, { time: AGO(30), sig: 'png' }),
    bk.fileNode('Rechnung März.pdf ', 210 * KB, { time: AGO(60) }),
    bk.fileNode('Notiz: wichtig?.txt', 2 * KB, { time: AGO(8) }),
    bk.fileNode('altes-iso', 0, { link: '/mnt/user/isos/Win10_22H2_German_x64.iso', time: AGO(200) }),
  );
  FS.nodeAt('/mnt/user/backups/appdata-2026-10-24.tar.zst').flaw = 'CRC-Fehler bei appdata/bombvault/bombvault.db';

  const SIG = { png: ['PNG-Bild', 'png'], jpg: ['JPEG-Bild', 'jpg'], pdf: ['PDF-Dokument', 'pdf'], zip: ['ZIP-Archiv', 'zip'] };
  const TEMP = /(\.part|\.!qb|\.tmp|\.crdownload|\.partial)$/i;
  const JUNK = ['.ds_store', 'thumbs.db', 'desktop.ini'];
  const BAD_CHARS = /[\\:*?"<>|]/;
  const emptyDir = (n) => n.dir && !n.link && FS.kids(n).every(emptyDir);
  const badName = (name) => /^\s|\s$/.test(name) ? 'Leerzeichen am Rand' : /\.$/.test(name) ? 'Punkt am Ende' : BAD_CHARS.test(name) ? 'Zeichen, die Windows nicht kennt' : name.length > 255 ? 'länger als 255 Zeichen' : '';
  const cleanName = (name) => name.trim().replace(/\.+$/, '').replace(new RegExp(BAD_CHARS.source, 'g'), '_');
  const fixExt = (n) => { const dot = n.name.lastIndexOf('.'); return (dot > 0 ? n.name.slice(0, dot) : n.name) + '.' + SIG[n.sig][1]; };

  // Each group says what it finds, and whether a ticked entry goes to the trash or gets a better name.
  const GROUPS = [
    { id: 'emptyDir', label: 'Leere Ordner', g: 'IconFolder', act: 'delete', tip: 'Ordner ohne Dateien. Ein Ordner, in dem nur leere Ordner liegen, zählt mit.' },
    { id: 'emptyFile', label: 'Leere Dateien', g: 'IconFile', act: 'delete', tip: 'Dateien mit 0 Byte.' },
    { id: 'temp', label: 'Temp-Reste', g: 'IconHourglass', act: 'delete', tip: 'Abgebrochene Downloads wie .part und .!qB, Office-Sperrdateien mit ~$ und was macOS und Windows in Ordnern ablegen.' },
    { id: 'link', label: 'Kaputte Verknüpfungen', g: 'IconUnlink', act: 'delete', tip: 'Verknüpfungen, deren Ziel es nicht mehr gibt.' },
    { id: 'ext', label: 'Falsche Endung', g: 'IconWarning', act: 'fix', tip: 'Der Inhalt passt nicht zur Endung, etwa ein PNG, das .jpg heißt. Aufräumen gibt die richtige Endung.' },
    { id: 'name', label: 'Problematische Namen', g: 'IconPencil', act: 'fix', tip: 'Namen, die über SMB auf Windows nicht gehen oder Ärger machen. Aufräumen ersetzt die Zeichen durch _ und nimmt Leerzeichen am Rand weg.' },
  ];

  function scan(root) {
    const found = Object.fromEntries(GROUPS.map(gr => [gr.id, []]));
    let scanned = 0;
    const walk = (n, path) => {
      for (const k of FS.kids(n)) {
        scanned++;
        const item = { node: k, dir: path, path: FS.join(path, k.name) };
        if (k.dir && !k.link && emptyDir(k)) { found.emptyDir.push({ ...item, note: FS.kids(k).length ? 'nur leere Ordner' : 'leer' }); continue; }
        if (k.link && !FS.nodeAt(k.link)) found.link.push({ ...item, note: `zeigt auf ${k.link}` });
        else if (!k.dir && (TEMP.test(k.name) || /^~\$/.test(k.name) || JUNK.includes(k.name.toLowerCase()))) found.temp.push({ ...item, note: fmtSize(k.size) || '0 B' });
        else if (!k.dir && !k.link && !k.size) found.emptyFile.push({ ...item, note: '0 B' });
        if (k.sig && SIG[k.sig] && FS.ext(k.name) !== SIG[k.sig][1]) found.ext.push({ ...item, note: `ist ein ${SIG[k.sig][0]}`, to: fixExt(k) });
        const why = badName(k.name);
        if (why) found.name.push({ ...item, note: why, to: cleanName(k.name) });
        if (k.dir && !k.link) walk(k, FS.join(path, k.name));
      }
    };
    walk(FS.nodeAt(root), root === '/' ? '' : root);
    return { found, scanned };
  }

  CC.action('cleanup', { label: 'Aufräumen', g: 'IconPrune', run: () => {
    let root = CC.view === 'files' && !FS.inArchive(bk.here()) ? bk.here() : '/mnt/user', res = null;
    const picked = new Set();
    const w = win({
      title: 'Aufräumen', wide: true, cls: 'bk bk-clean',
      body: `<div class="well bk-form">${frow('Suchen in', bk.pathField('cl-in', root), 'Aufräumen geht auch durch alle Unterordner.')}</div>
        <div class="well bk-fill bk-cleanwell"><div class="bk-tools"><span class="eyebrow" id="cl-sum">Funde</span></div><div class="bk-list" id="cl-list"></div></div>`,
      foot: [{ start: 'Gelöschtes landet im Papierkorb' }, closeBtn, { label: 'Aufräumen', g: 'IconPrune', primary: true, run: () => { apply(); return false; } }],
    });
    bk.wirePicks(w);
    const items = () => res ? GROUPS.flatMap(gr => res.found[gr.id].map(it => ({ ...it, gr }))) : [];
    const key = (it) => `${it.gr.id}|${it.path}`;
    const box = (on, attr) => `<button class="col-check cl-check" type="button" role="checkbox" aria-checked="${on}" ${attr}><span class="cbox ${on ? 'on' : ''}">${g('IconCheck')}</span></button>`;
    function draw() {
      const list = $('#cl-list', w);
      if (!res) return;
      const all = items(), chosen = all.filter(it => picked.has(key(it)));
      const del = chosen.filter(it => it.gr.act === 'delete').length, fix = chosen.length - del;
      $('#cl-sum', w).textContent = all.length ? `${bk.plural(all.length, 'Fund', 'Funde')} in ${bk.plural(res.scanned, 'Objekt', 'Objekten')}` : 'Funde';
      bk.setPrimary(w, chosen.length ? [del && `${del} löschen`, fix && `${fix} umbenennen`].filter(Boolean).join(', ') : 'Aufräumen', 'IconPrune', !chosen.length);
      const groups = GROUPS.filter(gr => res.found[gr.id].length);
      list.innerHTML = groups.length ? groups.map(gr => {
        const rows = res.found[gr.id], on = rows.filter(it => picked.has(`${gr.id}|${it.path}`)).length;
        return `<section class="bk-group">
          <div class="cl-head">${box(on === rows.length, `data-cl-group="${gr.id}" aria-label="Alle ${esc(gr.label)} wählen"`)}<span class="cl-g">${g(gr.g)}</span><span class="bk-name">${esc(gr.label)}</span>${CC.ui.info(gr.tip)}<span class="pill glim-num">${rows.length}</span><span class="pill">${gr.act === 'delete' ? 'Papierkorb' : 'Umbenennen'}</span></div>
          ${rows.map(it => `<div class="cl-row ${picked.has(`${gr.id}|${it.path}`) ? 'on' : ''}">${box(picked.has(`${gr.id}|${it.path}`), `data-cl="${gr.id}|${esc(it.path)}" aria-label="${esc(it.path)}"`)}
            <span class="bk-path mono" title="${esc(it.path)}"><bdi>${esc(it.path.slice(root === '/' ? 0 : root.length) || it.path)}</bdi></span>
            ${it.to ? `<span class="cl-to mono">${g('IconForward')}${esc(it.to)}</span>` : ''}<span class="pill">${esc(it.note)}</span></div>`).join('')}
        </section>`;
      }).join('') : `<div class="empty">${g('IconCheck')}<div>Hier ist alles aufgeräumt</div><div class="muted" style="font-size:12px">${bk.plural(res.scanned, 'Objekt', 'Objekte')} durchsucht</div></div>`;
    }
    const start = () => {
      const target = $('#cl-in', w).value.trim() || '/';
      picked.clear();
      if (!bk.isDir(target)) { res = null; $('#cl-list', w).innerHTML = `<div class="empty">${g('IconFolder')}<div>Diesen Ordner gibt es nicht</div></div>`; bk.setPrimary(w, 'Aufräumen', 'IconPrune', true); return; }
      root = target;
      res = scan(root);
      draw();
    };
    function apply() {
      const chosen = items().filter(it => picked.has(key(it)));
      const fixes = chosen.filter(it => it.gr.act === 'fix'), dels = chosen.filter(it => it.gr.act === 'delete');
      // A folder and something inside it may both be picked; the folder already takes the inner one along.
      const outer = dels.filter(it => !dels.some(o => o !== it && it.path.startsWith(o.path + '/')));
      if (fixes.length) {
        const done = fixes.filter(it => !FS.kids(FS.nodeAt(it.dir)).some(k => k !== it.node && k.name === it.to)).map(it => ({ node: it.node, from: it.node.name, to: it.to }));
        done.forEach(r => { r.node.name = r.to; });
        bk.log('umbenannt', `${bk.plural(done.length, 'Name', 'Namen')} beim Aufräumen`);
        CC.toast(`${bk.plural(done.length, 'Name', 'Namen')} bereinigt`, 'ok', { label: 'Rückgängig', run: () => { done.forEach(r => { r.node.name = r.from; }); if (CC.view === 'files') CC.render(); start(); } });
        if (done.length < fixes.length) CC.toast(`${fixes.length - done.length} ${fixes.length - done.length === 1 ? 'Name ist' : 'Namen sind'} schon vergeben und bleiben, wie sie sind`, 'warn');
      }
      if (outer.length) CC.deleteItems(outer.map(it => ({ ...it.node, at: it.dir })), root, start);
      else start();
      if (CC.view === 'files') CC.render();
    }
    w.addEventListener('click', e => {
      const one = e.target.closest('[data-cl]'), all = e.target.closest('[data-cl-group]');
      if (one) { const k = one.dataset.cl; picked.has(k) ? picked.delete(k) : picked.add(k); return draw(); }
      if (all) {
        const rows = res.found[all.dataset.clGroup].map(it => `${all.dataset.clGroup}|${it.path}`), every = rows.every(k => picked.has(k));
        rows.forEach(k => every ? picked.delete(k) : picked.add(k));
        draw();
      }
    });
    let typing;
    w.addEventListener('input', e => { if (e.target.id === 'cl-in') { clearTimeout(typing); typing = setTimeout(start, 400); } });
    start();
  } });

  // Archiv testen reads every member and its checksum; a damaged archive keeps a red mark in the list until it passes again.
  const isArchive = (k) => !k.dir && (FS.kindOf(k) === 'arch' || !!k.zip || /\.(7z|rar|zip)\.0*1$/i.test(k.name));
  CC.action('testArchive', { label: 'Archiv testen', g: 'IconCheck', needs: 'sel', run: () => {
    const dir = bk.here(), sel = bk.selNodes().filter(isArchive);
    if (!sel.length) return CC.toast('Wähl zuerst ein oder mehrere Archive, etwa ZIP, 7z, RAR oder TAR', 'warn');
    const rows = sel.map(n => ({ node: n, members: 6 + hashOf(n.name) % 140, done: 0, state: 'wartet' }));
    let timer;
    const line = (r, i) => `<div class="at-row ${r.state === 'defekt' ? 'bad' : r.state === 'in Ordnung' ? 'good' : ''}" data-at="${i}">${bk.tile(r.node)}
        <span class="bk-name">${esc(r.node.name)}</span><span class="bar"><i style="width:${(r.done / r.members * 100).toFixed(1)}%"></i></span>
        <span class="pill glim-num">${r.done} von ${r.members}</span><span class="pill at-state" ${r.why ? `data-tip="${esc(r.why)}"` : ''}>${r.state === 'defekt' ? g('IconWarning') : r.state === 'in Ordnung' ? g('IconCheck') : ''}${r.state}</span></div>`;
    const w = win({
      title: sel.length === 1 ? 'Archiv testen' : `${sel.length} Archive testen`, wide: true, cls: 'bk bk-atest',
      body: `<div class="well"><div class="eyebrow" id="at-sum">Wird geprüft</div><div class="at-list" id="at-list">${rows.map(line).join('')}</div></div>`,
      foot: [{ start: 'Geprüft wird jede Datei im Archiv, entpackt wird nichts' }, closeBtn],
      onClose: () => clearInterval(timer),
    });
    const draw = () => {
      $('#at-list', w).innerHTML = rows.map(line).join('');
      const bad = rows.filter(r => r.state === 'defekt').length, open = rows.filter(r => !['defekt', 'in Ordnung'].includes(r.state)).length;
      $('#at-sum', w).textContent = open ? 'Wird geprüft' : bad ? `${bk.plural(bad, 'Archiv ist', 'Archive sind')} defekt` : rows.length === 1 ? 'Das Archiv ist in Ordnung' : 'Alle Archive sind in Ordnung';
      if (!open) bk.setStart(w, bad ? 'Defekte Archive bleiben in der Liste rot markiert' : 'Nichts zu beanstanden');
    };
    timer = setInterval(() => {
      const r = rows.find(x => !['defekt', 'in Ordnung'].includes(x.state));
      if (!r) { clearInterval(timer); if (CC.view === 'files') CC.render(); return; }
      r.state = 'prüft';
      r.done = Math.min(r.members, r.done + Math.max(3, Math.ceil(r.members / 6)));
      // The damaged member is found partway through, as a real test stops at the first bad checksum.
      if (r.node.flaw && r.done >= r.members * .6) { r.state = 'defekt'; r.why = r.node.flaw; r.node.damaged = r.node.flaw; }
      else if (r.done >= r.members) { r.state = 'in Ordnung'; delete r.node.damaged; }
      draw();
    }, 160);
    bk.log('geprüft', `${bk.plural(sel.length, 'Archiv', 'Archive')} in ${dir}`);
  } });

  const tools = MENUS.find(m => m[0] === 'Werkzeuge')[1];
  tools.splice(tools.indexOf('duplicates') + 1, 0, 'cleanup');
  const file = MENUS.find(m => m[0] === 'Datei')[1];
  file.splice(file.indexOf('unpack') + 1, 0, 'testArchive');

  function decorate() {
    for (let i = 0; i < (CC.prefs.dual ? 2 : 1); i++) {
      const list = Files.entries(i);
      $$(`.panel[data-panel="${i}"] .row[data-idx]`).forEach(row => {
        const k = list[+row.dataset.idx];
        if (!k?.damaged) return;
        row.classList.add('damaged');
        row.querySelector('.name')?.insertAdjacentHTML('beforeend', `<span class="pill fault" data-tip="${esc(k.damaged)}">defekt</span>`);
      });
    }
  }
  const render = CC.views.files.render;
  CC.views.files.render = (host) => { render(host); decorate(); };

  CC.css(`
.bk-clean { height: min(46rem, calc(100vh - 3rem)); }
.bk-cleanwell .bk-list { gap: 1.1rem; padding-top: .3rem; }
.cl-head { display: flex; align-items: center; gap: .55rem; height: 2.2rem; padding: 0 .45rem; }
.cl-head .bk-name { font-weight: 600; flex: none; }
.cl-head .pill:first-of-type { margin-left: auto; }
.cl-g svg { width: 1rem; height: 1rem; color: var(--accent-glyph); display: block; }
.cl-check { min-height: 0; width: 1.6rem; height: 1.6rem; padding: 0; justify-content: center; flex: none; }
.cl-row { display: grid; grid-template-columns: 1.6rem minmax(0, 1fr) auto auto; gap: .6rem; align-items: center; min-height: 2.1rem; padding: 0 .45rem 0 2.6rem; border-radius: var(--radius-control); }
.cl-row:not(:has(.cl-to)) { grid-template-columns: 1.6rem minmax(0, 1fr) auto; }
.cl-row:hover { background: var(--carbon-surface3); }
.cl-row .bk-path { font-size: 12.5px; }
.cl-to { display: inline-flex; align-items: center; gap: .35rem; font-size: 12.5px; color: var(--carbon-text-sub); max-width: 18rem; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.cl-to svg { width: .75rem; height: .75rem; color: var(--accent-glyph); flex: none; }
.at-list { display: flex; flex-direction: column; gap: 2px; }
.at-row { display: grid; grid-template-columns: 1.75rem minmax(0, 1fr) 9rem auto 8rem; gap: .6rem; align-items: center; height: 2.4rem; padding: 0 .45rem; border-radius: var(--radius-control); }
.at-row .bar { height: .4rem; }
.at-row .at-state { justify-self: end; display: inline-flex; align-items: center; gap: .3rem; }
.at-row .at-state svg { width: .75rem; height: .75rem; }
.at-row.good .at-state { color: var(--status-ok-text, #6fdc8c); }
.at-row.bad .at-state { background: color-mix(in srgb, var(--status-fail-solid, #ff6b72) 22%, transparent); color: var(--status-fail-text, #ff8389); }
.at-row.bad .bar i { background: var(--status-fail-solid, #ff6b72); }
.row.damaged .name .t { color: var(--status-fail-text, #ff8389); }
.row.damaged .name .pill.fault { margin-left: .5rem; height: 1.2rem; font-size: 11px; }
@media (max-width: 760px) { .cl-row { padding-left: .45rem; } .cl-to { display: none; } .at-row { grid-template-columns: 1.75rem minmax(0, 1fr) auto; } .at-row .bar, .at-row .pill.glim-num { display: none; } }
`);
})();
