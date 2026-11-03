// Suchen (Alt+F7): every criterion Krusader's search window has, with hits that follow each keystroke.
(() => {
  const { sw, seg, frow, segVal, swVal, closeBtn } = CC.ui;
  const bk = CC.bk;
  const { dropdown, ddVal } = CC.admin;
  const TYPES = [['all', 'Alles'], ['dir', 'Ordner'], ['img', 'Bilder'], ['video', 'Videos'], ['doc', 'Dokumente'], ['arch', 'Archive']];
  const WHEN = [['any', 'Beliebig'], ['1', 'Heute'], ['7', 'Letzte 7 Tage'], ['30', 'Letzte 30 Tage'], ['365', 'Letztes Jahr'], ['old', 'Älter als ein Jahr']];
  const UNIT = { B: 1, K: KB, KB, M: MB, MB, G: GB, GB, T: TB, TB };
  const bytes = (s) => { const m = String(s).trim().toUpperCase().match(/^([\d.,]+)\s*([KMGT]?B?)$/); return m ? parseFloat(m[1].replace(',', '.')) * (UNIT[m[2]] || MB) : NaN; };
  const LIMIT = 400;
  let last = null;

  const typeOk = (n, type) => {
    if (type === 'all') return true;
    const k = FS.kindOf(n);
    if (type === 'doc') return ['pdf', 'office', 'text', 'code'].includes(k);
    if (type === 'arch') return k === 'arch' || k === 'disk' || !!n.zip;
    return k === type;
  };

  function find(q) {
    const nameRe = bk.globRe(q.name);
    const text = q.text.trim().toLowerCase();
    const min = bytes(q.min), max = bytes(q.max);
    const now = NOW;
    const hits = [];
    let scanned = 0;
    const walk = (n, path) => {
      for (const k of FS.kids(n)) {
        if (!q.hidden && bk.hidden(k)) continue;
        const p = FS.join(path, k.name);
        scanned++;
        const size = FS.totalSize(k);
        let ok = (!nameRe || nameRe.test(k.name)) && typeOk(k, q.type);
        if (ok && text) ok = !k.dir && !bk.isBinary(k) && bk.textOf(path, k).toLowerCase().includes(text);
        if (ok && !isNaN(min)) ok = size >= min;
        if (ok && !isNaN(max)) ok = size <= max;
        if (ok && q.when !== 'any') ok = q.when === 'old' ? k.time < now - 365 * DAY : k.time >= now - +q.when * DAY;
        if (ok) hits.push({ node: k, dir: path, path: p, size });
        if (k.dir && !k.link) walk(k, p);
      }
    };
    const root = FS.nodeAt(q.in);
    if (root?.dir) walk(root, q.in === '/' ? '' : q.in);
    hits.sort((a, b) => (b.node.dir ? 1 : 0) - (a.node.dir ? 1 : 0) || a.node.name.localeCompare(b.node.name, 'de', { numeric: true }));
    return { hits, scanned, found: !!root?.dir };
  }

  function open(hit, asTab) {
    if (!CC.prefs.showHidden && bk.hidden(hit.node)) CC.setPref('showHidden', true);
    const dir = hit.dir || '/';
    if (asTab) {
      const p = Files.P();
      p.tabs.push(newTab(dir));
      p.cur = p.tabs.length - 1;
      Files.go(dir, Files.active, false);
    } else Files.go(dir);
    bk.reveal(dir, hit.node.name);
    $('.panel.active .row.cursor')?.scrollIntoView({ block: 'center' });
  }

  CC.findFiles = find;
  CC.action('search', { label: 'Suchen', g: 'IconSearch', key: 'Alt+F7', run: () => {
    const start = CC.view === 'files' && !FS.inArchive(bk.here()) ? bk.here() : '/mnt/user';
    const q = { name: '', text: '', hidden: CC.prefs.showHidden, min: '', max: '', when: 'any', type: 'all', ...(last || {}), in: start };
    let res = { hits: [] }, cur = 0, timer;
    const w = win({
      title: 'Suchen', wide: true, cls: 'bk bk-search',
      body: `<div class="well bk-form">
          <div class="bk-grid2">
            ${frow('Name', `<input class="field grow" id="s-name" value="${esc(q.name)}" placeholder="*.iso *.img oder ein Teil des Namens" autocomplete="off" spellcheck="false">`, 'Mehrere Muster mit Leerzeichen trennen. * steht für beliebig viele Zeichen, ? für genau eines. Ohne * passt jeder Name, der den Text enthält.')}
            ${frow('Enthält Text', `<input class="field grow" id="s-text" value="${esc(q.text)}" placeholder="Wort oder Satz" autocomplete="off">`, 'Durchsucht den Inhalt von Text- und Quelltextdateien. Groß- und Kleinschreibung zählen nicht.')}
            ${frow('Suchen in', bk.pathField('s-in', q.in))}
            ${frow('Versteckte Dateien', sw(q.hidden, 'id="s-hidden"'), 'Dateien und Ordner, deren Name mit einem Punkt beginnt.')}
            ${frow('Größe', `<span class="bk-range"><input class="field bk-num glim-num" id="s-min" value="${esc(q.min)}" placeholder="ab" autocomplete="off"><span class="muted">bis</span><input class="field bk-num glim-num" id="s-max" value="${esc(q.max)}" placeholder="beliebig" autocomplete="off"></span>`, 'Eine Zahl allein zählt in MB. Mit Einheit geht auch 300 KB oder 2,5 GB.')}
            ${frow('Geändert', dropdown(WHEN, q.when, 'id="s-when"', 'grow'))}
          </div>
          ${frow('Art', seg(TYPES, q.type, 'id="s-type"'))}
        </div>
        <div class="well bk-fill bk-hits" hidden><div class="bk-tools"><span class="eyebrow" id="s-count">Treffer</span><button class="btn" id="s-save" data-tip="Die Suche kommt ins Menü des Suchfelds und zeigt beim Klick immer die aktuellen Treffer">${g('IconSave')}Suche speichern</button></div>
          <div class="bk-hhead"><span></span><span>Name</span><span>Ordner</span><span class="r">Größe</span><span class="r">Geändert</span></div>
          <div class="bk-list" id="s-list" role="listbox" aria-label="Treffer"></div></div>`,
      foot: [{ start: ' ' }, closeBtn,
        { label: 'Als Tab öffnen', g: 'IconAdd', run: () => { if (!res.hits[cur]) return false; open(res.hits[cur], true); } },
        { label: 'Ordner öffnen', g: 'IconFolderOpen', primary: true, run: () => { if (!res.hits[cur]) return false; open(res.hits[cur], false); } }],
    });
    bk.wirePicks(w);
    const read = () => Object.assign(q, {
      name: $('#s-name', w).value, text: $('#s-text', w).value, in: $('#s-in', w).value.trim() || '/', hidden: swVal($('#s-hidden', w)),
      min: $('#s-min', w).value, max: $('#s-max', w).value, when: ddVal($('#s-when', w)), type: segVal($('#s-type', w)),
    });
    const empty = () => !q.name.trim() && !q.text.trim() && !q.min && !q.max && q.when === 'any' && q.type === 'all';
    // The window stays as short as the form until there is something to list, then grows down to its full height.
    let drawn = false;
    const showHits = (on) => {
      const box = $('.win', w), well = $('.bk-hits', w);
      if (well.hidden === !on) return;
      const from = box.offsetHeight;
      well.hidden = !on;
      box.classList.toggle('bk-open', on);
      const to = box.offsetHeight;
      if (drawn && CC.prefs.motion !== 'off' && !matchMedia('(prefers-reduced-motion: reduce)').matches) box.animate([{ height: from + 'px' }, { height: to + 'px' }], { duration: 240, easing: 'cubic-bezier(.2, .8, .2, 1)' });
    };
    const draw = () => {
      const list = $('#s-list', w);
      const btns = $$('.win-foot .btn', w).slice(1);
      btns.forEach(b => { b.disabled = !res.hits.length; });
      showHits(!empty());
      drawn = true;
      if (empty()) {
        bk.setStart(w, '');
        list.innerHTML = '';
        return;
      }
      if (!res.found) {
        $('#s-count', w).textContent = 'Treffer';
        list.innerHTML = `<div class="empty">${g('IconFolder')}<div>Diesen Ordner gibt es nicht</div></div>`;
        return;
      }
      const shown = res.hits.slice(0, LIMIT);
      $('#s-count', w).textContent = res.hits.length > LIMIT ? `Die ersten ${LIMIT} von ${res.hits.length.toLocaleString('de-AT')} Treffern` : bk.plural(res.hits.length, 'Treffer', 'Treffer');
      bk.setStart(w, `${res.scanned.toLocaleString('de-AT')} Objekte durchsucht`);
      list.innerHTML = shown.length ? shown.map((h, i) => `<div class="bk-row bk-hit ${i === cur ? 'cur' : ''}" data-bk-hit="${i}" role="option" aria-selected="${i === cur}">
          ${bk.tile(h.node)}<span class="bk-name">${esc(h.node.name)}</span><span class="bk-path muted mono" title="${esc(h.dir || '/')}"><bdi>${esc(h.dir || '/')}</bdi></span>
          <span class="pill glim-num">${fmtSize(h.size) || '0 B'}</span><span class="pill glim-num">${fmtDate(h.node.time)}</span></div>`).join('')
        : `<div class="empty">${g('IconFilter')}<div>Nichts gefunden</div><div class="muted" style="font-size:12px">Ein anderer Ordner oder weniger Bedingungen finden vielleicht mehr</div></div>`;
    };
    const run = () => { read(); last = { ...q }; res = empty() ? { hits: [], found: true } : find(q); cur = 0; draw(); };
    const move = (d) => {
      if (!res.hits.length) return;
      cur = Math.max(0, Math.min(Math.min(res.hits.length, LIMIT) - 1, cur + d));
      $$('.bk-hit', w).forEach((r, i) => { r.classList.toggle('cur', i === cur); r.setAttribute('aria-selected', i === cur); });
      $(`.bk-hit[data-bk-hit="${cur}"]`, w)?.scrollIntoView({ block: 'nearest' });
    };
    w.addEventListener('input', () => { clearTimeout(timer); timer = setTimeout(run, 120); });
    w.addEventListener('change', run);
    let lastClick = {};
    w.addEventListener('click', e => {
      if (e.target.closest('#s-save')) { read(); CC.smart.save({ ...q }); return; }
      const r = e.target.closest('[data-bk-hit]');
      if (!r) return;
      const i = +r.dataset.bkHit;
      // A single click on a hit picks it; a second one on the same hit goes there.
      if (lastClick.i === i && e.timeStamp - lastClick.at < 450) { open(res.hits[i], false); CC.closeWin(); return; }
      lastClick = { i, at: e.timeStamp };
      cur = i; move(0);
    });
    w.addEventListener('keydown', e => {
      if (e.key === 'ArrowDown') { e.preventDefault(); move(1); }
      if (e.key === 'ArrowUp') { e.preventDefault(); move(-1); }
      if (e.key === 'Enter' && e.target.closest('input')) { e.preventDefault(); bk.primary(w).click(); }
    });
    run();
    setTimeout(() => { const i = $('#s-name', w); i.focus(); i.select(); }, 0);
  } });

  CC.css(`
.bk-search { align-self: start; margin-top: calc((100vh - 3rem - min(46rem, 100vh - 3rem)) / 2); }
.bk-search.bk-open { height: min(46rem, calc(100vh - 3rem)); }
.bk-search .bk-hits[hidden] { display: none; }
.bk-range { display: flex; gap: .4rem; align-items: center; margin-left: auto; flex: 1; min-width: 0; justify-content: flex-end; }
.bk-num { width: 6.5rem; min-width: 0; flex: 1 1 6.5rem; }
.bk-hits { gap: .35rem; }
.bk-hhead, .bk-hit { display: grid; grid-template-columns: 1.75rem minmax(0, 1.2fr) minmax(0, 1fr) 6rem 6.5rem; gap: .6rem; align-items: center; }
.bk-hhead { font-size: 12px; color: var(--carbon-text-muted); padding: 0 .45rem; height: 1.4rem; }
.bk-hhead .r, .bk-hit .pill { justify-self: end; }
.bk-hit .bk-path { font-size: 12px; }
@media (max-width: 760px) { .bk-search .win-foot .btn:not(.primary):not(:first-of-type) { display: none; } .bk-range { flex-wrap: wrap; margin-left: 0; } .bk-hhead { display: none; } .bk-hit { grid-template-columns: 1.75rem minmax(0, 1fr) auto; } .bk-hit .bk-path, .bk-hit .pill:last-child { display: none; } }
`);
})();
