// Ordner vergleichen and Synchronisieren: the two panels side by side, file by file.
(() => {
  const { sw, seg, frow, segVal, swVal, cancel, closeBtn, info } = CC.ui;
  const bk = CC.bk;

  // Two copies count as the same when size and time agree; by content only the size can tell them apart here.
  function state(l, r, content) {
    if (!r) return 'onlyL';
    if (!l) return 'onlyR';
    if (l.dir || r.dir) return l.dir && r.dir && FS.totalSize(l) === FS.totalSize(r) ? 'same' : 'differ';
    const sameSize = l.size === r.size;
    if (content ? sameSize : sameSize && Math.abs(l.time - r.time) < 2000) return 'same';
    if (l.time > r.time + 2000) return 'newerL';
    if (r.time > l.time + 2000) return 'newerR';
    return 'differ';
  }

  function compare(lPath, rPath, { content = false, recursive = false, top = false } = {}) {
    const out = [];
    const walk = (ln, rn, rel) => {
      const show = (k) => CC.prefs.showHidden || !bk.hidden(k);
      const names = [...new Set([...FS.kids(ln).filter(show), ...FS.kids(rn).filter(show)].map(k => k.name))].sort((a, b) => a.localeCompare(b, 'de', { numeric: true }));
      for (const name of names) {
        const l = FS.kids(ln).find(k => k.name === name), r = FS.kids(rn).find(k => k.name === name);
        const p = rel ? rel + '/' + name : name;
        if (!top && l?.dir && r?.dir) { if (recursive) walk(l, r, p); continue; }
        out.push({ rel: p, name, l, r, state: state(l, r, content) });
      }
    };
    walk(FS.nodeAt(lPath), FS.nodeAt(rPath), '');
    return out;
  }

  const MARK = {
    same: [{ label: 'gleich', tone: 'ok' }, { label: 'gleich', tone: 'ok' }],
    newerL: [{ label: 'neuer', tone: 'warn' }, { label: 'älter', tone: 'warn' }],
    newerR: [{ label: 'älter', tone: 'warn' }, { label: 'neuer', tone: 'warn' }],
    differ: [{ label: 'anders', tone: 'warn' }, { label: 'anders', tone: 'warn' }],
    onlyL: [{ label: 'nur hier', tone: '' }, null],
    onlyR: [null, { label: 'nur hier', tone: '' }],
  };

  const label = (p) => FS.LABEL[p] || FS.split(p).pop() || 'Server';

  CC.action('compareDirs', { label: 'Ordner vergleichen', g: 'IconCompare', key: 'Strg+Umschalt+C', needs: 'dual', run: () => {
    const L = Files.T(0).path, R = Files.T(1).path;
    if (L === R) return CC.toast('Beide Fenster zeigen denselben Ordner', 'warn');
    const items = compare(L, R, { top: true });
    Files.marks = [{}, {}];
    for (const it of items) MARK[it.state].forEach((m, i) => { if (m) Files.marks[i][it.name] = m; });
    CC.render();
    const count = (s) => items.filter(i => i.state === s).length;
    const rows = [['Gleich', 'same', 'ok'], ['Links neuer', 'newerL', 'warn'], ['Rechts neuer', 'newerR', 'warn'], ['Unterschiedlich', 'differ', 'warn'], ['Nur links', 'onlyL', ''], ['Nur rechts', 'onlyR', '']];
    const diffs = items.filter(i => i.state !== 'same');
    win({
      title: 'Ordner vergleichen', cls: 'bk',
      body: `<div class="well"><div class="eyebrow">${esc(label(L))} und ${esc(label(R))}</div>
        <table class="tbl">${rows.filter(([, s]) => s === 'same' || count(s)).map(([l, s, tone]) => `<tr><td>${l}</td><td class="r"><span class="pill glim-num ${count(s) && tone ? tone : ''}">${count(s)}</span></td></tr>`).join('')}</table></div>`,
      foot: [{ start: diffs.length ? bk.plural(diffs.length, 'Unterschied', 'Unterschiede') : 'Beide Ordner sind gleich' }, closeBtn,
        { label: 'Unterschiede markieren', g: 'IconSelectAll', run: () => {
          [0, 1].forEach(i => { Files.T(i).marked = new Set(diffs.filter(d => i === 0 ? d.l : d.r).map(d => d.name)); });
          CC.render();
          CC.toast(`${bk.plural(diffs.length, 'Unterschied', 'Unterschiede')} markiert`);
        } },
        { label: 'Synchronisieren', g: 'IconSync', primary: true, run: () => { setTimeout(() => CC.run('sync'), 0); } }],
    });
  } });

  // Turns the comparison into steps. A step that would overwrite something newer starts switched off.
  function plan(items, dir, mirror) {
    const steps = [];
    const add = (it, act, to, off = false, why = '') => steps.push({ key: `${it.rel}|${act}|${to}`, it, act, to, off, why, size: FS.totalSize(act === 'delete' ? (to === 'R' ? it.r : it.l) : (to === 'R' ? it.l : it.r)) });
    for (const it of items) {
      const s = it.state;
      if (s === 'same') continue;
      if (dir === 'right') {
        if (s === 'onlyL') add(it, 'copy', 'R');
        else if (s === 'onlyR') { if (mirror) add(it, 'delete', 'R'); }
        else add(it, 'replace', 'R', s !== 'newerL', s === 'newerR' ? 'Rechts ist neuer' : s === 'differ' ? 'Gleich alt, andere Größe' : '');
      } else if (dir === 'left') {
        if (s === 'onlyR') add(it, 'copy', 'L');
        else if (s === 'onlyL') { if (mirror) add(it, 'delete', 'L'); }
        else add(it, 'replace', 'L', s !== 'newerR', s === 'newerL' ? 'Links ist neuer' : s === 'differ' ? 'Gleich alt, andere Größe' : '');
      } else {
        if (s === 'onlyL') add(it, 'copy', 'R');
        else if (s === 'onlyR') add(it, 'copy', 'L');
        else if (s === 'newerL') add(it, 'replace', 'R');
        else if (s === 'newerR') add(it, 'replace', 'L');
        else add(it, 'replace', 'R', true, 'Gleich alt, andere Größe');
      }
    }
    return steps;
  }

  const ACT = { copy: 'Kopieren', replace: 'Ersetzen', delete: 'Löschen' };

  CC.action('sync', { label: 'Synchronisieren', g: 'IconSync', needs: 'dual', run: () => {
    const L = Files.T(0).path, R = Files.T(1).path;
    if (L === R) return CC.toast('Beide Fenster zeigen denselben Ordner', 'warn');
    if (FS.inArchive(L) || FS.inArchive(R)) return CC.toast('Im Archiv geht nur Ansehen und Entpacken', 'warn');
    const o = { dir: 'right', by: 'time', mirror: false, recursive: true };
    const touched = new Map();
    let steps = [];
    const w = win({
      title: 'Synchronisieren', wide: true, cls: 'bk bk-sync',
      body: `<div class="well bk-form"><div class="eyebrow">Abgleich</div>
          <div class="bk-sides"><span class="bk-side mono" title="${esc(L)}"><bdi>${esc(L)}</bdi></span><span class="bk-arrow" id="y-arrow"></span><span class="bk-side mono" title="${esc(R)}"><bdi>${esc(R)}</bdi></span></div>
          <div class="bk-rows">
            ${frow('Richtung', seg([['left', 'Nach links', 'IconBack'], ['both', 'Beide', 'IconSync'], ['right', 'Nach rechts', 'IconForward']], o.dir, 'id="y-dir"'))}
            ${frow('Vergleichen nach', seg([['time', 'Größe und Datum'], ['content', 'Inhalt']], o.by, 'id="y-by"'), 'Größe und Datum geht schnell. Inhalt liest jede Datei ganz und findet auch Änderungen, die Größe und Datum nicht zeigen.')}
            ${frow('Unterordner einbeziehen', sw(o.recursive, 'id="y-rec"'))}
            ${frow('Spiegeln', sw(o.mirror, 'id="y-mirror"'), 'Löscht im Ziel, was es in der Quelle nicht gibt, damit beide Ordner danach gleich sind. Gelöschtes landet im Papierkorb. In beide Richtungen geht das nicht.')}
          </div></div>
        <div class="well bk-fill"><div class="bk-tools"><span class="eyebrow" id="y-count">Vorschau</span>${info('Probelauf: noch ist nichts geschrieben. Jeder Schritt lässt sich einzeln abschalten.')}</div>
          <div class="bk-list" id="y-list"></div></div>`,
      foot: [{ start: 'Läuft als Aufgabe im Hintergrund' }, cancel, { label: 'Synchronisieren', g: 'IconSync', primary: true, run: () => apply() }],
    });
    const isOn = (s) => touched.has(s.key) ? touched.get(s.key) : !s.off;
    const draw = () => {
      o.dir = segVal($('#y-dir', w)); o.by = segVal($('#y-by', w)); o.recursive = swVal($('#y-rec', w));
      const mirrorSw = $('#y-mirror', w);
      mirrorSw.disabled = o.dir === 'both';
      if (o.dir === 'both') mirrorSw.setAttribute('aria-checked', 'false');
      o.mirror = swVal(mirrorSw);
      $('#y-arrow', w).innerHTML = g(o.dir === 'left' ? 'IconBack' : o.dir === 'right' ? 'IconForward' : 'IconSync');
      steps = plan(compare(L, R, { content: o.by === 'content', recursive: o.recursive }), o.dir, o.mirror);
      const on = steps.filter(isOn);
      $('#y-count', w).textContent = steps.length ? `${bk.plural(on.length, 'Schritt', 'Schritte')} von ${steps.length}, ${fmtSize(on.filter(s => s.act !== 'delete').reduce((a, s) => a + s.size, 0)) || '0 B'}` : 'Vorschau';
      $('#y-list', w).innerHTML = steps.length ? steps.map((s, i) => {
        const node = s.act === 'delete' ? (s.to === 'R' ? s.it.r : s.it.l) : (s.to === 'R' ? s.it.l : s.it.r);
        const arrow = s.act === 'delete' ? 'IconTrash' : s.to === 'R' ? 'IconForward' : 'IconBack';
        return `<div class="bk-step ${isOn(s) ? '' : 'skip'}">${sw(isOn(s), `data-bk-step="${i}" aria-label="${esc(s.it.rel)}"`)}${bk.tile(node)}
          <span class="bk-name">${esc(s.it.rel)}${node.dir ? '/' : ''}</span>
          ${s.why ? `<span class="pill warn">${s.why}</span>` : '<span></span>'}
          <span class="bk-arrow sm" data-tip="${s.to === 'R' ? 'rechts' : 'links'}">${g(arrow)}</span>
          <span class="pill">${ACT[s.act]}${s.act === 'delete' ? (s.to === 'R' ? ' rechts' : ' links') : ''}</span>
          <span class="pill glim-num">${fmtSize(s.size) || '0 B'}</span></div>`;
      }).join('') : `<div class="empty">${g('IconCheck')}<div>Nichts zu tun, beide Seiten sind gleich</div></div>`;
      CC.ui.wireControls(w);
      bk.primary(w).disabled = !on.length;
    };
    w.addEventListener('change', e => {
      const s = e.target.closest('[data-bk-step]');
      if (s) { const st = steps[+s.dataset.bkStep]; touched.set(st.key, swVal(s)); }
      draw();
    });
    function apply() {
      const todo = steps.filter(isOn);
      if (!todo.length) return false;
      const total = todo.reduce((a, s) => a + (s.act === 'delete' ? 0 : s.size), 0);
      const labelTxt = `Synchronisieren: ${label(L)} und ${label(R)}`;
      Jobs.start({ kind: 'sync', label: labelTxt, total, speed: total > 10 * GB ? .5 : 1, done: () => {
        for (const s of todo) {
          const base = s.to === 'R' ? R : L, src = s.to === 'R' ? s.it.l : s.it.r;
          const relDir = s.it.rel.split('/').slice(0, -1).join('/');
          const parent = relDir ? FS.join(base, relDir) : base;
          const dirNode = bk.mkdirs(parent);
          if (s.act === 'delete') {
            const victim = s.to === 'R' ? s.it.r : s.it.l;
            dirNode.children = dirNode.children.filter(c => c !== victim);
            DATA.trash.unshift({ name: victim.name, from: parent, size: FS.totalSize(victim), deleted: Date.now(), by: CC.user.name, dir: !!victim.dir, node: victim });
          } else {
            dirNode.children = dirNode.children.filter(c => c.name !== src.name);
            dirNode.children.push(bk.clone(src));
          }
        }
        Files.marks = [{}, {}];
        bk.log('synchronisiert', `${bk.plural(todo.length, 'Schritt', 'Schritte')} zwischen ${L} und ${R}`);
        CC.toast(`Synchronisiert: ${bk.plural(todo.length, 'Schritt', 'Schritte')}`);
        if (CC.view === 'files') CC.render();
      } });
      CC.toast(`${labelTxt} läuft`, 'ok', { label: 'Aufgaben', run: () => CC.run('jobs') });
    }
    draw();
  } });

  CC.css(`
.bk-sync { height: min(48rem, calc(100vh - 3rem)); }
.bk-rows { display: contents; }
.bk-sides { display: grid; grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr); gap: .75rem; align-items: center; margin-bottom: .2rem; }
.bk-side { height: var(--btn-h); display: flex; align-items: center; padding: 0 .7rem; border-radius: var(--radius-control); background: var(--carbon-surface3); overflow: hidden; white-space: nowrap; text-overflow: ellipsis; direction: rtl; }
.bk-arrow { width: 1.75rem; height: 1.75rem; display: grid; place-items: center; color: var(--accent-glyph); }
.bk-arrow svg { width: 1rem; height: 1rem; }
.bk-arrow.sm { color: var(--carbon-text-sub); }
.bk-step { display: grid; grid-template-columns: auto 1.75rem minmax(0, 1fr) auto 1.75rem 7.5rem 5.5rem; gap: .6rem; align-items: center; min-height: 2.25rem; padding: 0 .45rem; border-radius: var(--radius-control); flex: none; }
.bk-step:hover { background: var(--carbon-surface3); }
.bk-step .pill { justify-self: end; }
.bk-step.skip .bk-name, .bk-step.skip .pill:not(.warn), .bk-step.skip .bk-arrow { color: var(--carbon-text-muted); }
.bk-step.skip .bk-name { text-decoration: line-through; text-decoration-color: color-mix(in srgb, var(--carbon-text) 35%, transparent); }
@media (max-width: 760px) { .bk-step { grid-template-columns: auto minmax(0, 1fr) 1.75rem auto; } .bk-step .bk-tile, .bk-step > span:nth-of-type(3), .bk-step .pill.glim-num { display: none; } .bk-sides { grid-template-columns: 1fr; } }
`);
})();
