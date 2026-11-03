// Doppelte Dateien: groups of files with the same name and size, and the copies you pick go to the trash.
(() => {
  const { closeBtn, seg, segVal, sw, swVal, frow } = CC.ui;
  const bk = CC.bk;

  function scan(root, by) {
    const groups = new Map();
    let files = 0;
    FS.walk((k, p) => {
      if (k.dir || k.link || (!CC.prefs.showHidden && bk.hidden(k)) || !k.size) return;
      files++;
      const key = by === 'name' ? `${k.name}|${k.size}` : `${k.size}`;
      if (!groups.has(key)) groups.set(key, []);
      groups.get(key).push({ node: k, dir: FS.parent(p) });
    }, FS.nodeAt(root), root === '/' ? '' : root);
    const list = [...groups.values()].filter(g2 => g2.length > 1).map(copies => {
      // On equal times the copy with the shorter path counts as the original and is kept first.
      copies.sort((a, b) => b.node.time - a.node.time || a.dir.length - b.dir.length);
      return { name: copies[0].node.name, size: copies[0].node.size, copies, wasted: copies[0].node.size * (copies.length - 1) };
    });
    list.sort((a, b) => b.wasted - a.wasted);
    return { list, files };
  }

  CC.action('duplicates', { label: 'Doppelte Dateien', g: 'IconCopy', run: () => {
    let root = CC.view === 'files' && !FS.inArchive(bk.here()) ? bk.here() : '/mnt/user', by = 'name', res = null;
    const drop = new Set();
    const w = win({
      title: 'Doppelte Dateien', wide: true, cls: 'bk bk-dups',
      body: `<div class="well bk-form"><div class="bk-grid2">
          ${frow('Suchen in', bk.pathField('u-in', root))}
          ${frow('Gleich heißt', seg([['name', 'Name und Größe'], ['size', 'Nur Größe']], by, 'id="u-by"'), 'Nur Größe findet auch umbenannte Kopien, aber auch Dateien, die zufällig gleich groß sind.')}
        </div></div>
        <div class="well bk-fill bk-dupwell"><div class="bk-tools"><span class="eyebrow" id="u-sum">Gruppen</span>
          <button class="btn" data-bk-keep="new">${g('IconLatest')}Neueste behalten</button><button class="btn" data-bk-keep="old">${g('IconFirst')}Älteste behalten</button><button class="btn" data-bk-keep="none">${g('IconClearSelection')}Keine löschen</button></div>
          <div class="bk-list" id="u-list"></div></div>`,
      foot: [{ start: 'Gelöschte Kopien landen im Papierkorb' }, closeBtn, { label: 'Kopien löschen', g: 'IconTrash', primary: true, run: () => remove() }],
    });
    bk.wirePicks(w);
    const all = () => res ? res.list.flatMap(gr => gr.copies) : [];
    const draw = () => {
      const list = $('#u-list', w);
      $$('[data-bk-keep]', w).forEach(b => { b.disabled = !res?.list.length; });
      if (!res) return;
      const freed = all().filter(c => drop.has(c.node)).reduce((a, c) => a + c.node.size, 0);
      const n = all().filter(c => drop.has(c.node)).length;
      $('#u-sum', w).textContent = res.list.length ? `${bk.plural(res.list.length, 'Gruppe', 'Gruppen')}, ${fmtSize(res.list.reduce((a, gr) => a + gr.wasted, 0))} doppelt` : 'Gruppen';
      bk.setPrimary(w, n ? `${bk.plural(n, 'Kopie', 'Kopien')} löschen, ${fmtSize(freed)}` : 'Kopien löschen', 'IconTrash', !n);
      list.innerHTML = res.list.length ? res.list.map((gr, gi) => `<section class="bk-group">
          <div class="bk-ghead">${bk.tile(gr.copies[0].node)}<span class="bk-name">${esc(by === 'name' ? gr.name : `${gr.copies.length} Dateien gleicher Größe`)}</span><span class="pill glim-num">${bk.plural(gr.copies.length, 'Kopie', 'Kopien')} je ${fmtSize(gr.size)}</span></div>
          ${gr.copies.map((c, ci) => `<div class="bk-copy ${drop.has(c.node) ? 'gone' : ''}">
            <span class="bk-path mono" title="${esc(FS.join(c.dir, c.node.name))}"><bdi>${esc(by === 'name' ? c.dir : FS.join(c.dir, c.node.name))}</bdi></span>
            ${gr.copies[0].node.time === gr.copies.at(-1).node.time ? '<span></span>' : ci === 0 ? '<span class="pill">neueste</span>' : ci === gr.copies.length - 1 ? '<span class="pill">älteste</span>' : '<span></span>'}
            <span class="pill glim-num">${fmtTime(c.node.time)}</span>
            <span class="bk-swl">${sw(drop.has(c.node), `data-bk-dup="${gi}:${ci}" aria-label="${esc(FS.join(c.dir, c.node.name))} löschen" data-tip="In den Papierkorb"`)}</span></div>`).join('')}
        </section>`).join('')
        : `<div class="empty">${g('IconCheck')}<div>Keine doppelten Dateien gefunden</div><div class="muted" style="font-size:12px">${res.files.toLocaleString('de-AT')} Dateien durchsucht</div></div>`;
      CC.ui.wireControls(list);
    };
    let scanTimer;
    const start = () => {
      res = null; drop.clear();
      clearInterval(scanTimer);
      const target = $('#u-in', w).value.trim() || '/';
      if (!bk.isDir(target)) { $('#u-list', w).innerHTML = `<div class="empty">${g('IconFolder')}<div>Diesen Ordner gibt es nicht</div></div>`; bk.setPrimary(w, 'Kopien löschen', 'IconTrash', true); return; }
      root = target;
      $('#u-sum', w).textContent = 'Gruppen';
      $('#u-list', w).innerHTML = `<div class="bk-scan"><div>Durchsuche ${esc(root)}</div><div class="bar"><i id="u-bar" style="width:0%"></i></div><div class="glim-num" id="u-n">0 Dateien</div></div>`;
      bk.setPrimary(w, 'Kopien löschen', 'IconTrash', true);
      $$('[data-bk-keep]', w).forEach(b => { b.disabled = true; });
      const found = scan(root, by);
      let pct = 0;
      scanTimer = setInterval(() => {
        pct = Math.min(100, pct + 9 + Math.random() * 14);
        const bar = $('#u-bar', w);
        if (!bar) return clearInterval(scanTimer);
        bar.style.width = pct + '%';
        $('#u-n', w).textContent = `${Math.round(found.files * pct / 100).toLocaleString('de-AT')} Dateien`;
        if (pct >= 100) { clearInterval(scanTimer); res = found; draw(); }
      }, 90);
    };
    const keep = (mode) => {
      drop.clear();
      if (mode !== 'none') for (const gr of res.list) gr.copies.forEach((c, i) => { if (mode === 'new' ? i > 0 : i < gr.copies.length - 1) drop.add(c.node); });
      draw();
    };
    function remove() {
      const victims = all().filter(c => drop.has(c.node));
      if (!victims.length) return false;
      const freed = victims.reduce((a, c) => a + c.node.size, 0);
      for (const c of victims) {
        const parent = FS.nodeAt(c.dir);
        parent.children = parent.children.filter(k => k !== c.node);
        DATA.trash.unshift({ name: c.node.name, from: c.dir, size: c.node.size, deleted: Date.now(), by: CC.user.name, node: c.node });
      }
      bk.log('gelöscht', `${bk.plural(victims.length, 'doppelte Kopie', 'doppelte Kopien')} in ${root}`);
      if (CC.view === 'files') CC.render();
      CC.toast(`${bk.plural(victims.length, 'Kopie', 'Kopien')} im Papierkorb, ${fmtSize(freed)} frei`, 'ok', { label: 'Rückgängig', run: () => {
        for (const c of victims) { FS.nodeAt(c.dir).children.push(c.node); DATA.trash = DATA.trash.filter(t => t.node !== c.node); }
        if (CC.view === 'files') CC.render();
      } });
    }
    w.addEventListener('click', e => { const k = e.target.closest('[data-bk-keep]'); if (k && res) keep(k.dataset.bkKeep); });
    w.addEventListener('change', e => {
      const s = e.target.closest('[data-bk-dup]');
      if (s) {
        const [gi, ci] = s.dataset.bkDup.split(':').map(Number), gr = res.list[gi], c = gr.copies[ci];
        if (swVal(s)) {
          if (gr.copies.every(x => x === c || drop.has(x.node))) { s.setAttribute('aria-checked', 'false'); CC.toast('Eine Kopie bleibt immer', 'warn'); return; }
          drop.add(c.node);
        } else drop.delete(c.node);
        draw();
        return;
      }
      if (e.target.closest('#u-by')) { by = segVal($('#u-by', w)); start(); }
    });
    let typing;
    w.addEventListener('input', e => { if (e.target.id === 'u-in') { clearTimeout(typing); typing = setTimeout(start, 400); } });
    start();
  } });

  CC.css(`
.bk-dups { height: min(46rem, calc(100vh - 3rem)); }
.bk-dupwell { gap: .5rem; }
.bk-dupwell .bk-list { gap: 1.1rem; padding-top: .3rem; }
.bk-group { display: flex; flex-direction: column; gap: 1px; }
.bk-ghead { display: grid; grid-template-columns: 1.75rem minmax(0, 1fr) auto; gap: .6rem; align-items: center; height: 2.1rem; padding: 0 .45rem; }
.bk-ghead .bk-name { font-weight: 600; }
.bk-copy { display: grid; grid-template-columns: minmax(0, 1fr) auto auto auto; gap: .6rem; align-items: center; height: 2.1rem; padding: 0 .45rem 0 2.8rem; border-radius: var(--radius-control); }
.bk-copy:hover { background: var(--carbon-surface3); }
.bk-copy .bk-path { font-size: 12.5px; }
.bk-copy.gone .bk-path { color: var(--carbon-text-muted); text-decoration: line-through; text-decoration-color: color-mix(in srgb, var(--carbon-text) 35%, transparent); }
.bk-swl { display: flex; }
@media (max-width: 760px) { .bk-copy { padding-left: .45rem; grid-template-columns: minmax(0, 1fr) auto; } .bk-copy > .pill { display: none; } }
`);
})();
