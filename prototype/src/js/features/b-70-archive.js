// Packen and Entpacken. Every archive made here can be opened like a folder, as the zip files in the data can.
(() => {
  const { sw, seg, frow, segVal, swVal, cancel } = CC.ui;
  const bk = CC.bk;

  const FORMATS = [['zip', 'zip'], ['tar.zst', 'tar.zst'], ['7z', '7z'], ['rar', 'rar']];
  const LEVELS = [['0', 'Ohne'], ['3', 'Schnell'], ['6', 'Normal'], ['9', 'Stark']];
  const squeezed = (n) => ['img', 'video', 'audio', 'arch', 'disk'].includes(FS.kindOf(n)) ? .97 : FS.kindOf(n) === 'pdf' || FS.kindOf(n) === 'office' ? .82 : .3;
  function packedSize(nodes, fmt, level) {
    const raw = (n) => n.dir ? FS.kids(n).reduce((a, k) => a + raw(k), 0) : (n.size || 0) * (level === '0' ? 1 : squeezed(n) * (1.06 - +level * .012) * (fmt === '7z' ? .94 : fmt === 'tar.zst' ? .96 : 1));
    return Math.max(120, nodes.reduce((a, n) => a + raw(n), 0));
  }
  const stripArchive = (name) => name.replace(/\.(tar\.(zst|gz|xz|bz2)|zip|7z|rar|tgz|tar|zst|gz)$/i, '');
  const isArchive = (n) => !n.dir && (!!n.zip || FS.kindOf(n) === 'arch');
  const where = (dir, name) => `<span class="bk-path mono bk-final" title="${esc(FS.join(dir, name))}"><bdi>${esc(FS.join(dir, name))}</bdi></span>`;

  CC.action('pack', { label: 'Verpacken', g: 'IconArchive', key: 'Alt+P', needs: 'sel', run: () => {
    const src = bk.here(), nodes = bk.selNodes();
    const total = nodes.reduce((a, n) => a + FS.totalSize(n), 0);
    const base = nodes.length === 1 ? (nodes[0].dir ? nodes[0].name : stripArchive(nodes[0].name.replace(/\.[^.]+$/, '')) || nodes[0].name) : (FS.split(src).pop() || 'Archiv').replace('::', '');
    const dest0 = FS.inArchive(bk.otherDir()) ? FS.parent(src) : bk.otherDir();
    const w = win({
      title: 'Verpacken', cls: 'bk',
      body: `<div class="well bk-form"><div class="eyebrow">${bk.plural(nodes.length, 'Objekt', 'Objekte')}, ${fmtSize(total)}</div>
          ${frow('Name', `<span class="bk-namef"><input class="field grow" id="k-name" value="${esc(base)}" spellcheck="false" autocomplete="off"><span class="pill mono" id="k-ext">.zip</span></span>`)}
          ${frow('Format', seg(FORMATS, 'zip', 'id="k-fmt"'), 'zip öffnet jedes System ohne Zusatzprogramm. tar.zst ist am schnellsten und behält Rechte und Besitzer. 7z packt am kleinsten. rar braucht das rar-Programm auf dem Server.')}
          ${frow('Kompression', seg(LEVELS, '6', 'id="k-level"'), 'Fotos, Videos und Musik sind schon gepackt und werden kaum kleiner. Bei ihnen spart „Ohne“ Zeit.')}
          ${frow('Passwort', CC.admin.pwField('id="k-pw" placeholder="Ohne Passwort"', 'm'), 'zip, 7z und rar verschlüsseln mit AES-256. tar.zst kennt kein Passwort.')}
        </div>
        <div class="well bk-form"><div class="eyebrow">Ziel</div>
          ${frow('Ordner', bk.pathField('k-dest', dest0))}
          ${frow('Wird gespeichert als', `<span class="bk-finalw" id="k-final"></span>`)}
        </div>`,
      foot: [{ start: ' ' }, cancel, { label: 'Verpacken', g: 'IconArchive', primary: true, run: () => go() }],
    });
    bk.wirePicks(w);
    const st = {};
    const draw = () => {
      st.fmt = segVal($('#k-fmt', w)); st.level = segVal($('#k-level', w));
      st.dest = $('#k-dest', w).value.trim();
      const pw = $('#k-pw', w);
      pw.disabled = st.fmt === 'tar.zst';
      if (pw.disabled) pw.value = '';
      pw.nextElementSibling.disabled = pw.disabled;
      $('#k-ext', w).textContent = '.' + st.fmt;
      const name = $('#k-name', w).value.trim();
      const ok = bk.isDir(st.dest);
      st.file = ok && name ? (bk.nodeIn(st.dest, `${name}.${st.fmt}`) ? Files.freeName(FS.nodeAt(st.dest), `${name}.${st.fmt}`) : `${name}.${st.fmt}`) : '';
      $('#k-final', w).innerHTML = !ok ? '<span class="pill fault">Diesen Ordner gibt es nicht</span>' : !name ? '<span class="pill fault">Der Name fehlt</span>'
        : where(st.dest, st.file) + (st.file !== `${name}.${st.fmt}` ? '<span class="pill warn" data-tip="Eine Datei mit diesem Namen liegt schon dort. Sie bleibt, das neue Archiv bekommt eine Nummer.">Name vergeben</span>' : '');
      st.size = packedSize(nodes, st.fmt, st.level);
      bk.setStart(w, `Etwa ${fmtSize(st.size)}, läuft als Aufgabe im Hintergrund`);
      bk.primary(w).disabled = !st.file;
    };
    function go() {
      if (!st.file) return false;
      const { dest, file, fmt, size } = st, locked = !!$('#k-pw', w).value;
      const label = `Verpacken: ${file}`;
      Jobs.start({ kind: 'pack', label, total: nodes.reduce((a, n) => a + FS.totalSize(n), 0), done: () => {
        const name = bk.nodeIn(dest, file) ? Files.freeName(FS.nodeAt(dest), file) : file;
        FS.nodeAt(dest).children.push(bk.fileNode(name, size, { zip: nodes.map(bk.clone), ...(locked ? { locked: true } : {}), packed: fmt }));
        bk.log('gepackt', `${bk.plural(nodes.length, 'Objekt', 'Objekte')} nach ${FS.join(dest, name)}`);
        bk.reveal(dest, name);
        bk.doneToast(`Gepackt: ${FS.join(dest, name)}`, FS.join(dest, name));
      } });
      CC.toast(`${label} läuft`, 'ok', { label: 'Aufgaben', run: () => CC.run('jobs') });
    }
    w.addEventListener('input', draw);
    w.addEventListener('change', draw);
    w.addEventListener('keydown', e => { if (e.key === 'Enter' && e.target.closest('input')) { e.preventDefault(); bk.primary(w).click(); } });
    draw();
    setTimeout(() => { const i = $('#k-name', w); i.focus(); i.select(); }, 0);
  } });

  // Archives without a list of their own get one that fits their name, so unpacking always yields something.
  function contentsOf(n) {
    if (n.zip) return n.zip.map(bk.clone);
    const r = bk.rng(n.name), name = stripArchive(n.name), size = n.size || MB;
    if (/appdata/i.test(n.name)) {
      const apps = ['bombvault', 'knightloader', 'plex', 'nginx', 'homarr'];
      return [bk.dirNode('appdata', apps.map((a, i) => bk.dirNode(a, [bk.fileNode(i % 2 ? 'config.xml' : 'settings.json', 8 * KB), bk.fileNode(`${a}.db`, size * 1.4 * (.1 + r() * .3))], { time: n.time })), { time: n.time })];
    }
    if (/flash/i.test(n.name)) return [bk.dirNode('config', [bk.fileNode('ident.cfg', KB), bk.fileNode('go', KB), bk.fileNode('super.dat', 4 * KB)], { time: n.time }), bk.fileNode('bzimage', 9 * MB, { time: n.time }), bk.fileNode('bzroot', 140 * MB, { time: n.time })];
    const count = 3 + Math.floor(r() * 4);
    return [bk.dirNode(name, Array.from({ length: count }, (_, i) => bk.fileNode(`${name}-${String(i + 1).padStart(2, '0')}.${['jpg', 'pdf', 'txt', 'mkv'][Math.floor(r() * 4)]}`, size * 1.05 / count, { time: n.time })), { time: n.time })];
  }

  CC.action('unpack', { label: 'Entpacken', g: 'IconArchive', key: 'Alt+U', needs: 'sel', run: () => {
    const src = bk.here(), inside = FS.inArchive(src);
    const archives = inside ? [] : bk.selNodes().filter(isArchive);
    const loose = inside ? bk.selNodes() : [];
    if (!inside && !archives.length) return CC.toast('Erst ein Archiv wählen', 'warn');
    const archName = inside ? FS.split(src).find(p => p.endsWith('::')).replace('::', '') : '';
    const dest0 = inside ? (FS.inArchive(bk.otherDir()) ? '/mnt/user/downloads' : bk.otherDir()) : (CC.prefs.dual && !FS.inArchive(bk.otherDir()) ? bk.otherDir() : src);
    const jobs = inside ? [{ name: archName, items: loose }] : archives.map(a => ({ name: a.name, items: contentsOf(a), locked: a.locked }));
    const unpacked = jobs.reduce((a, j) => a + j.items.reduce((b, k) => b + FS.totalSize(k), 0), 0);
    const w = win({
      title: 'Entpacken', cls: 'bk',
      body: `<div class="well"><div class="eyebrow">${inside ? `${bk.plural(loose.length, 'Objekt', 'Objekte')} aus ${esc(archName)}` : bk.plural(archives.length, 'Archiv', 'Archive')}</div>
          <table class="tbl">${(inside ? loose : archives).slice(0, 5).map(a => `<tr><td>${esc(a.name)}</td><td class="glim-num r">${fmtSize(FS.totalSize(a))}</td></tr>`).join('')}${(inside ? loose : archives).length > 5 ? `<tr><td colspan="2" class="muted">und ${(inside ? loose : archives).length - 5} weitere</td></tr>` : ''}</table></div>
        <div class="well bk-form"><div class="eyebrow">Ziel</div>
          ${frow('Ordner', bk.pathField('x-dest', dest0))}
          ${frow('In eigenen Ordner', sw(!inside, 'id="x-own"'), 'Legt für jedes Archiv einen Ordner mit seinem Namen an, damit sich nichts mit vorhandenen Dateien mischt.')}
          ${frow('Wenn es das schon gibt', seg([['skip', 'Überspringen'], ['both', 'Beide behalten'], ['replace', 'Ersetzen']], 'both', 'id="x-clash"'), 'Beide behalten hängt an den neuen Namen eine Nummer an. Ersetzen überschreibt die vorhandene Datei.')}
          ${frow('Landet in', `<span class="bk-finalw" id="x-final"></span>`)}
        </div>`,
      foot: [{ start: `Ergibt ${fmtSize(unpacked)}, läuft als Aufgabe im Hintergrund` }, cancel, { label: 'Entpacken', g: 'IconArchive', primary: true, run: () => go() }],
    });
    bk.wirePicks(w);
    const st = {};
    const draw = () => {
      st.dest = $('#x-dest', w).value.trim(); st.own = swVal($('#x-own', w)); st.clash = segVal($('#x-clash', w));
      const ok = bk.isDir(st.dest);
      $('#x-final', w).innerHTML = !ok ? '<span class="pill fault">Diesen Ordner gibt es nicht</span>'
        : st.own ? where(st.dest, stripArchive(jobs[0].name)) + (jobs.length > 1 ? `<span class="pill">und ${jobs.length - 1} weitere</span>` : '') : where(st.dest, '');
      bk.primary(w).disabled = !ok;
    };
    function go() {
      if (!bk.isDir(st.dest)) return false;
      const { dest, own, clash } = st;
      const label = `Entpacken: ${jobs.length === 1 ? jobs[0].name : bk.plural(jobs.length, 'Archiv', 'Archive')}`;
      Jobs.start({ kind: 'unpack', label, total: unpacked, done: () => {
        let first = null, skipped = 0;
        for (const j of jobs) {
          let target = FS.nodeAt(dest), targetPath = dest;
          if (own) {
            const folder = stripArchive(j.name);
            let d = FS.kids(target).find(k => k.name === folder && k.dir);
            if (!d) { d = bk.dirNode(folder); target.children.push(d); }
            target = d; targetPath = FS.join(dest, folder);
            first ??= folder;
          }
          for (const item of j.items.map(bk.clone)) {
            const there = FS.kids(target).find(k => k.name === item.name);
            if (there && clash === 'skip') { skipped++; continue; }
            if (there && clash === 'replace') target.children = target.children.filter(k => k !== there);
            if (there && clash === 'both') item.name = Files.freeName(target, item.name);
            item.time = item.time || NOW;
            target.children.push(item);
            if (!own) first ??= item.name;
          }
          bk.log('entpackt', `${j.name} nach ${targetPath}`);
        }
        if (first) bk.reveal(dest, first); else if (CC.view === 'files') CC.render();
        const path = own && first ? FS.join(dest, first) : dest;
        bk.doneToast(`Entpackt nach ${path}${skipped ? `, ${skipped} übersprungen` : ''}`, path);
      } });
      CC.toast(`${label} läuft`, 'ok', { label: 'Aufgaben', run: () => CC.run('jobs') });
    }
    w.addEventListener('input', draw);
    w.addEventListener('change', draw);
    draw();
  } });

  CC.css(`
.bk-namef { display: flex; gap: .4rem; align-items: center; flex: 0 1 22rem; margin-left: auto; }
.bk-namef .field { flex: 1; min-width: 0; }
.bk-finalw { display: flex; gap: .4rem; align-items: center; min-width: 0; flex: 1; max-width: 30rem; justify-content: flex-end; }
.bk-final { font-size: 12.5px; color: var(--carbon-text); }
`);
})();
