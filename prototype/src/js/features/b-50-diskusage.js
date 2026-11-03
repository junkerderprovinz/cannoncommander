// Speicherbelegung (Alt+D) works like TreeSize: one folder as a tree sorted by size, as a treemap, by file type and as
// its largest files, and whatever it lists can be selected and deleted right there.
(() => {
  const { closeBtn, seg, segVal } = CC.ui;
  const bk = CC.bk;

  // Bruls, Huizing and van Wijk: fill rows along the short side while the worst aspect ratio keeps improving.
  function squarify(items, x, y, w, h) {
    const out = [];
    const total = items.reduce((a, it) => a + it.v, 0);
    if (!total || w <= 0 || h <= 0) return out;
    const scale = (w * h) / total;
    const rest = items.map(it => ({ it, a: it.v * scale }));
    const worst = (row, side) => {
      const s = row.reduce((a, r) => a + r.a, 0), mx = Math.max(...row.map(r => r.a)), mn = Math.min(...row.map(r => r.a));
      return Math.max((side * side * mx) / (s * s), (s * s) / (side * side * mn));
    };
    while (rest.length) {
      const side = Math.min(w, h);
      let row = [rest.shift()];
      while (rest.length && worst([...row, rest[0]], side) <= worst(row, side)) row.push(rest.shift());
      const s = row.reduce((a, r) => a + r.a, 0);
      if (w >= h) {
        const rw = s / h; let yy = y;
        for (const r of row) { const rh = r.a / rw; out.push({ it: r.it, x, y: yy, w: rw, h: rh }); yy += rh; }
        x += rw; w -= rw;
      } else {
        const rh = s / w; let xx = x;
        for (const r of row) { const rw = r.a / rh; out.push({ it: r.it, x: xx, y, w: rw, h: rh }); xx += rw; }
        y += rh; h -= rh;
      }
    }
    return out;
  }

  const fit = (text, px) => { const max = Math.floor(px / 7.2); return text.length <= max ? text : max > 3 ? text.slice(0, max - 1) + '…' : ''; };
  const shown = (k) => CC.prefs.showHidden || !bk.hidden(k);
  const typeOf = (n) => KIND_NAMES[FS.kindOf(n)] || 'Sonstiges';
  const pct = (part, whole) => whole ? part / whole * 100 : 0;
  const share = (part, whole) => `<span class="du-share"><span class="bar"><i style="width:${pct(part, whole).toFixed(1)}%"></i></span><span class="glim-num">${Math.round(pct(part, whole))} %</span></span>`;

  // One walk per drawing gives every folder its size, its file and folder count and its newest change.
  function measure(root) {
    const memo = new Map();
    const visit = (n) => {
      if (memo.has(n)) return memo.get(n);
      const s = n.dir ? { size: 0, files: 0, dirs: 0, time: n.time || 0 } : { size: n.link ? 0 : n.size || 0, files: 1, dirs: 0, time: n.time || 0 };
      if (n.dir && !n.link) for (const k of FS.kids(n).filter(shown)) {
        const c = visit(k);
        s.size += c.size; s.files += c.files; s.dirs += c.dirs + (k.dir ? 1 : 0);
        if (c.time > s.time) s.time = c.time;
      }
      memo.set(n, s);
      return s;
    };
    visit(root);
    return visit;
  }
  const filesUnder = (n, path, out = []) => {
    for (const k of FS.kids(n).filter(shown)) {
      if (k.dir) { if (!k.link) filesUnder(k, FS.join(path, k.name), out); } else out.push({ node: k, at: path, path: FS.join(path, k.name) });
    }
    return out;
  };

  const VIEWS = [['tree', 'Baum', 'IconRowMedium'], ['map', 'Kacheln', 'IconModules'], ['types', 'Dateitypen', 'IconFiles'], ['largest', 'Größte Dateien', 'IconSort']];
  const COLS = [['name', 'Name'], ['size', 'Größe'], ['share', 'Anteil'], ['files', 'Dateien'], ['dirs', 'Ordner'], ['time', 'Geändert']];

  CC.action('diskUsage', { label: 'Speicherbelegung', g: 'IconDashboard', key: 'Alt+D', run: (start) => {
    let cur = start || (CC.view === 'files' && !FS.inArchive(bk.here()) ? bk.here() : '/mnt/user');
    let view = VIEWS.some(v => v[0] === CC.prefs.duView) ? CC.prefs.duView : 'tree';
    let sort = { by: 'size', dir: -1 }, kind = null, stat, rows = [], focus = null, anchor = null, ro;
    const open = new Set(), sel = new Map();

    const w = win({
      title: 'Speicherbelegung', full: true, cls: 'bk bk-du',
      body: `<div class="du-top"><div class="bk-crumbs" id="du-crumbs"></div>${seg(VIEWS, view, 'id="du-view"')}</div>
        <div class="well du-well"><div class="du-body" id="du-body" tabindex="0"></div></div>`,
      foot: [{ start: ' ' }, closeBtn,
        { label: 'Löschen', g: 'IconTrash', run: () => { remove(); return false; } },
        { label: 'Hier öffnen', g: 'IconFolderOpen', primary: true, run: () => Files.go(cur) }],
      onClose: () => ro?.disconnect(),
    });
    const body = $('#du-body', w), delBtn = $('.win-foot [data-f="2"]', w);

    const go = (p) => { cur = p; open.clear(); sel.clear(); focus = null; kind = null; draw(); body.focus(); };
    const setView = (v) => { view = v; CC.setPref('duView', v); $$('#du-view button', w).forEach(b => b.setAttribute('aria-pressed', String(b.dataset.v === v))); draw(); body.focus(); };

    const order = (a, b) => {
      if (sort.by === 'name') return a.name.localeCompare(b.name, 'de', { numeric: true }) * -sort.dir;
      const key = sort.by === 'share' ? 'size' : sort.by;
      return (stat(a)[key] - stat(b)[key]) * sort.dir || a.name.localeCompare(b.name, 'de', { numeric: true });
    };

    function treeRows() {
      const out = [];
      const add = (n, path, depth) => {
        const whole = stat(n).size;
        for (const k of FS.kids(n).filter(shown).sort(order)) {
          const p = FS.join(path, k.name), canOpen = k.dir && !k.link && FS.kids(k).some(shown);
          out.push({ node: k, at: path, path: p, depth, whole, canOpen });
          if (canOpen && open.has(p)) add(k, p, depth + 1);
        }
      };
      add(FS.nodeAt(cur), cur, 0);
      return out;
    }

    const head = () => `<div class="du-row du-head" role="row">${COLS.map(([id, label]) => `<button class="du-sort ${sort.by === id ? 'on' : ''}" data-du-sort="${id}">${label}${sort.by === id ? g(sort.dir > 0 ? 'IconMoveUp' : 'IconMoveDown') : ''}</button>`).join('')}</div>`;
    const rowClass = (r) => `du-row ${sel.has(r.path) ? 'sel' : ''} ${focus === r.path ? 'cur' : ''} ${r.node.dir ? 'dir' : ''}`;

    function drawTree() {
      rows = treeRows();
      body.className = 'du-body du-tree';
      body.setAttribute('role', 'tree');
      body.innerHTML = head() + (rows.length ? rows.map((r, i) => {
        const s = stat(r.node);
        return `<div class="${rowClass(r)}" data-dui="${i}" role="treeitem" aria-level="${r.depth + 1}" aria-selected="${sel.has(r.path)}" ${r.canOpen ? `aria-expanded="${open.has(r.path)}"` : ''}>
          <span class="du-name" style="--d:${r.depth}">${r.canOpen ? `<button class="tree-tog ${open.has(r.path) ? 'open' : ''}" data-du-tog tabindex="-1" aria-label="${open.has(r.path) ? 'Zuklappen' : 'Aufklappen'}">${g('IconExpand')}</button>` : '<span class="tree-tog"></span>'}${g(FS.glyphOf(r.node))}<span class="n">${esc(r.node.name)}</span></span>
          <span class="glim-num r">${fmtSize(s.size) || '0 B'}</span>${share(s.size, r.whole)}
          <span class="glim-num r">${r.node.dir ? s.files.toLocaleString('de-AT') : ''}</span><span class="glim-num r">${r.node.dir ? s.dirs.toLocaleString('de-AT') : ''}</span>
          <span class="glim-num r muted">${s.time ? fmtDate(s.time) : ''}</span></div>`;
      }).join('') : `<div class="empty">${g('IconFolder')}<div>Dieser Ordner ist leer</div></div>`);
    }

    function drawLargest() {
      const all = filesUnder(FS.nodeAt(cur), cur).filter(f => !kind || typeOf(f.node) === kind.name);
      rows = all.sort((a, b) => (b.node.size || 0) - (a.node.size || 0)).slice(0, 200);
      const whole = stat(FS.nodeAt(cur)).size;
      body.className = 'du-body du-largest';
      body.setAttribute('role', 'listbox');
      body.innerHTML = (kind ? `<div class="du-kind">${g(kind.glyph)}<span>Nur ${esc(kind.name)}</span><button class="btn icon" data-du-allkinds data-tip="Wieder alle Dateien zeigen" aria-label="Wieder alle Dateien zeigen">${g('IconClose')}</button></div>` : '')
        + (rows.length ? rows.map((r, i) => `<div class="${rowClass(r)}" data-dui="${i}" role="option" aria-selected="${sel.has(r.path)}">
          <span class="du-name">${g(FS.glyphOf(r.node))}<span class="n">${esc(r.node.name)}</span></span>
          <span class="du-path mono muted"><bdi>${esc(r.at === cur ? '' : r.at.slice(cur === '/' ? 1 : cur.length + 1))}</bdi></span>
          <span class="glim-num r">${fmtSize(r.node.size) || '0 B'}</span>${share(r.node.size || 0, whole)}
          <span class="glim-num r muted">${r.node.time ? fmtDate(r.node.time) : ''}</span></div>`).join('')
        : `<div class="empty">${g('IconFile')}<div>Hier liegen keine Dateien</div></div>`);
    }

    function drawTypes() {
      const groups = new Map();
      for (const f of filesUnder(FS.nodeAt(cur), cur)) {
        const k = typeOf(f.node), e = FS.ext(f.node.name);
        const grp = groups.get(k) || { name: k, glyph: FS.GLYPH[FS.kindOf(f.node)], size: 0, files: 0, exts: new Map() };
        grp.size += f.node.size || 0; grp.files += 1;
        if (e) grp.exts.set(e, (grp.exts.get(e) || 0) + (f.node.size || 0));
        groups.set(k, grp);
      }
      const list = [...groups.values()].sort((a, b) => b.size - a.size), whole = list.reduce((a, x) => a + x.size, 0);
      rows = [];
      body.className = 'du-body du-types';
      body.setAttribute('role', 'list');
      body.innerHTML = list.length ? list.map(x => `<button class="du-row du-type" data-du-kind="${esc(x.name)}" data-du-glyph="${x.glyph}" data-tip="Die größten Dateien dieser Art zeigen">
          <span class="du-name">${g(x.glyph)}<span class="n">${esc(x.name)}</span></span>
          <span class="du-exts muted">${[...x.exts].sort((a, b) => b[1] - a[1]).slice(0, 5).map(([e]) => '.' + esc(e)).join(' ')}</span>
          <span class="glim-num r muted">${bk.plural(x.files, 'Datei', 'Dateien')}</span>
          <span class="glim-num r">${fmtSize(x.size) || '0 B'}</span>${share(x.size, whole)}</button>`).join('')
        : `<div class="empty">${g('IconFile')}<div>Hier liegen keine Dateien</div></div>`;
    }

    function drawMap() {
      if (view !== 'map') return;
      const node = FS.nodeAt(cur);
      rows = FS.kids(node).filter(shown).map(k => ({ node: k, at: cur, path: FS.join(cur, k.name), v: stat(k).size })).filter(r => r.v > 0).sort((a, b) => b.v - a.v);
      body.className = 'du-body du-map';
      body.setAttribute('role', 'img');
      const { width, height } = body.getBoundingClientRect();
      if (!rows.length) { body.innerHTML = `<div class="empty">${g('IconFolder')}<div>Hier liegt nichts, das Platz braucht</div></div>`; return; }
      const max = rows[0].v;
      body.innerHTML = `<svg width="${width}" height="${height}" viewBox="0 0 ${width} ${height}" aria-label="Belegung von ${esc(cur)}">${squarify(rows.map((r, i) => ({ ...r, i })), 0, 0, width, height).map(b => {
        const r = b.it, pad = 2, x = b.x + pad, y = b.y + pad, bw = Math.max(0, b.w - pad * 2), bh = Math.max(0, b.h - pad * 2);
        const mix = r.node.dir ? Math.round(18 + r.v / max * 37) : 0;
        const fill = r.node.dir ? `color-mix(in srgb, var(--accent) ${mix}%, var(--carbon-surface3))` : 'var(--carbon-surface3)';
        const ink = r.node.dir && mix > 45 ? 'var(--accent-contrast)' : 'var(--carbon-text)';
        const name = fit(r.node.name, bw - 14), size = fit(fmtSize(r.v), bw - 14);
        return `<g class="bk-blk ${r.node.dir ? 'dir' : ''} ${sel.has(r.path) ? 'sel' : ''}" data-dui="${r.i}" data-tip="${esc(r.node.name)}, ${fmtSize(r.v)}" style="--blk:${fill};--ink:${ink}">
          <rect x="${x}" y="${y}" width="${bw}" height="${bh}" rx="6"></rect>
          ${bw > 54 && bh > 24 && name ? `<text x="${x + 8}" y="${y + 17}" class="n">${esc(name)}</text>` : ''}
          ${bw > 54 && bh > 42 && size ? `<text x="${x + 8}" y="${y + 33}" class="s">${esc(size)}</text>` : ''}</g>`;
      }).join('')}</svg>`;
    }

    function summary() {
      if (sel.size) {
        const size = [...sel.values()].reduce((a, r) => a + stat(r.node).size, 0);
        return `${sel.size.toLocaleString('de-AT')} ausgewählt, ${fmtSize(size) || '0 B'}`;
      }
      const s = stat(FS.nodeAt(cur));
      return `${fmtSize(s.size) || '0 B'} in ${bk.plural(s.files, 'Datei', 'Dateien')} und ${bk.plural(s.dirs, 'Ordner', 'Ordnern')}`;
    }

    function draw() {
      stat = measure(FS.nodeAt(cur));
      for (const p of [...sel.keys()]) if (!FS.nodeAt(p)) sel.delete(p);
      $('#du-crumbs', w).innerHTML = bk.crumbs(cur, 'data-du-cd');
      ({ tree: drawTree, largest: drawLargest, types: drawTypes, map: drawMap })[view]();
      bk.setStart(w, summary());
      delBtn.disabled = !sel.size;
      $('.du-row.cur', body)?.scrollIntoView({ block: 'nearest' });
    }

    const at = (el) => rows[+el.closest('[data-dui]').dataset.dui];
    function pick(r, e = {}) {
      if (!r) return;
      if (e.shiftKey && anchor) {
        const a = rows.findIndex(x => x.path === anchor), b = rows.indexOf(r);
        if (!(e.ctrlKey || e.metaKey)) sel.clear();
        for (let i = Math.min(a, b); i <= Math.max(a, b); i++) if (i >= 0) sel.set(rows[i].path, rows[i]);
      } else if (e.ctrlKey || e.metaKey) {
        sel.has(r.path) ? sel.delete(r.path) : sel.set(r.path, r);
        anchor = r.path;
      } else {
        sel.clear(); sel.set(r.path, r); anchor = r.path;
      }
      focus = r.path;
      draw();
    }
    const toggle = (r, state = !open.has(r.path)) => { if (!r?.canOpen) return; state ? open.add(r.path) : open.delete(r.path); focus = r.path; draw(); };

    // A folder and something inside it may both be picked; deleting the folder already takes the inner one.
    function remove() {
      const picked = [...sel.values()];
      if (!picked.length) return CC.toast('Wähle zuerst aus, was weg soll', 'warn');
      if (FS.inArchive(cur)) return CC.toast('Im Archiv geht nur Ansehen und Entpacken', 'warn');
      const outer = picked.filter(r => !picked.some(o => o !== r && r.path.startsWith(o.path + '/')));
      CC.deleteItems(outer.map(r => ({ ...r.node, at: r.at })), cur, () => { sel.clear(); focus = null; draw(); });
    }
    function reveal(r) {
      CC.closeWin();
      if (CC.view !== 'files') CC.go('files');
      Files.go(r.at);
      const i = Files.entries().findIndex(k => k.name === r.node.name);
      if (i >= 0) Files.T().cursor = i;
      CC.render();
    }

    w.addEventListener('change', e => { if (e.target.id === 'du-view') setView(segVal(e.target)); });
    w.addEventListener('click', e => {
      const c = e.target.closest('[data-du-cd]');
      if (c) return go(c.dataset.duCd);
      const s = e.target.closest('[data-du-sort]');
      if (s) { const by = s.dataset.duSort; sort = { by, dir: sort.by === by ? -sort.dir : by === 'name' ? 1 : -1 }; return draw(); }
      const k = e.target.closest('[data-du-kind]');
      if (k) { kind = { name: k.dataset.duKind, glyph: k.dataset.duGlyph }; sel.clear(); return setView('largest'); }
      if (e.target.closest('[data-du-allkinds]')) { kind = null; return draw(); }
      const row = e.target.closest('[data-dui]');
      if (!row) return;
      const r = at(row);
      if (e.target.closest('[data-du-tog]')) return toggle(r);
      if (view === 'map' && r.node.dir && !r.node.link && !(e.ctrlKey || e.metaKey || e.shiftKey)) return go(r.path);
      pick(r, e);
    });
    w.addEventListener('dblclick', e => {
      const row = e.target.closest('[data-dui]');
      if (!row || view === 'map') return;
      const r = at(row);
      if (r.node.dir && !r.node.link) go(r.path); else reveal(r);
    });
    w.addEventListener('contextmenu', e => {
      const row = e.target.closest('[data-dui]');
      if (!row) return;
      e.preventDefault();
      e.stopPropagation();
      const r = at(row);
      if (!sel.has(r.path)) pick(r);
      const many = sel.size > 1;
      menu(e.clientX, e.clientY, [
        ...(r.canOpen && view === 'tree' ? [{ label: open.has(r.path) ? 'Zuklappen' : 'Aufklappen', g: open.has(r.path) ? 'IconCollapse' : 'IconExpand', run: () => toggle(r) }] : []),
        ...(r.node.dir && !r.node.link ? [{ label: 'Von hier aus ansehen', g: 'IconDashboard', run: () => go(r.path) }] : []),
        { label: 'Im Fenster zeigen', g: 'IconFolderOpen', run: () => reveal(r) },
        { label: 'Pfad kopieren', g: 'IconCopy', run: () => bk.copyText(r.path) },
        '-',
        { label: many ? `${sel.size} Objekte löschen` : 'Löschen', g: 'IconTrash', run: remove },
      ]);
    });
    body.addEventListener('keydown', e => {
      if (e.key === 'Backspace' && cur !== '/') { e.preventDefault(); return go(FS.parent(cur)); }
      if (e.key === 'Delete') { e.preventDefault(); return remove(); }
      if (!rows.length || view === 'map' || view === 'types') return;
      const i = rows.findIndex(r => r.path === focus), r = rows[i];
      const step = (to) => { e.preventDefault(); const n = rows[Math.max(0, Math.min(rows.length - 1, to))]; if (e.shiftKey) pick(n, { shiftKey: true }); else pick(n); };
      if (e.key === 'ArrowDown') return step(i + 1);
      if (e.key === 'ArrowUp') return step(i < 0 ? 0 : i - 1);
      if (e.key === 'Home') return step(0);
      if (e.key === 'End') return step(rows.length - 1);
      if (!r) return;
      if (e.key === 'ArrowRight' && view === 'tree') { e.preventDefault(); if (r.canOpen && !open.has(r.path)) toggle(r, true); else if (rows[i + 1]?.depth > r.depth) pick(rows[i + 1]); }
      if (e.key === 'ArrowLeft' && view === 'tree') { e.preventDefault(); if (open.has(r.path)) toggle(r, false); else { const up = rows.find(x => x.path === r.at); if (up) pick(up); } }
      if (e.key === 'Enter') { e.preventDefault(); if (view === 'tree' && r.canOpen) toggle(r); else if (r.node.dir) go(r.path); else reveal(r); }
      if (e.key === 'a' && (e.ctrlKey || e.metaKey)) { e.preventDefault(); rows.forEach(x => sel.set(x.path, x)); draw(); }
    });
    w.addEventListener('mouseover', e => {
      const b = e.target.closest('.bk-blk');
      $$('.bk-blk.hl', w).forEach(x => x.classList.remove('hl'));
      b?.classList.add('hl');
    });
    ro = new ResizeObserver(() => drawMap());
    ro.observe(body);
    draw();
    setTimeout(() => body.focus(), 0);
  } });

  CC.css(`
.bk-du .win-body { gap: .8rem; }
.du-top { display: flex; align-items: center; justify-content: space-between; gap: 1rem; flex-wrap: wrap; }
.du-top .bk-crumbs { flex: 1; min-width: 0; }
.du-well { flex: 1; min-height: 0; padding: .35rem; display: flex; }
.du-body { flex: 1; min-height: 0; overflow: auto; outline: none; position: relative; border-radius: var(--radius-control); }
.du-body:focus-visible { box-shadow: 0 0 0 2px var(--accent-soft, color-mix(in srgb, var(--accent) 35%, transparent)); }
.du-row { display: grid; align-items: center; gap: .75rem; height: 2.15rem; padding: 0 .6rem; border-radius: var(--radius-control); width: 100%; text-align: left; cursor: default; }
.du-tree .du-row { grid-template-columns: minmax(14rem, 1fr) 6rem 9rem 5.5rem 5rem 6.5rem; }
.du-largest .du-row { grid-template-columns: minmax(12rem, 1fr) minmax(8rem, 1fr) 6rem 9rem 6.5rem; }
.du-types .du-row { grid-template-columns: minmax(10rem, 14rem) minmax(0, 1fr) 7rem 6rem 9rem; cursor: pointer; }
.du-row:not(.du-head):hover { background: var(--carbon-surface3); }
.du-row.sel { background: color-mix(in srgb, var(--accent) 18%, var(--carbon-surface2)); }
.du-row.sel:hover { background: color-mix(in srgb, var(--accent) 24%, var(--carbon-surface2)); }
.du-row.cur { box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--accent) 60%, transparent); }
.du-row .r { text-align: right; font-size: 13px; }
.du-head { position: sticky; top: 0; z-index: 1; background: var(--carbon-surface2); height: 2rem; }
.du-sort { all: unset; cursor: pointer; display: inline-flex; align-items: center; gap: .3rem; font-size: 12px; color: var(--carbon-text-muted); }
.du-sort:not([data-du-sort="name"]) { justify-content: flex-end; }
.du-sort.on { color: var(--carbon-text); }
.du-sort svg { width: .7rem; height: .7rem; }
.du-name { display: flex; align-items: center; gap: .45rem; min-width: 0; padding-left: calc(var(--d, 0) * 1.1rem); }
.du-name > svg { width: 1rem; height: 1rem; flex: none; color: var(--carbon-text-sub); }
.du-row.dir .du-name > svg { color: var(--accent-glyph); }
.du-name .n { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.du-path, .du-exts { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 12px; }
.du-share { display: grid; grid-template-columns: 1fr 2.6rem; gap: .45rem; align-items: center; font-size: 12px; color: var(--carbon-text-muted); text-align: right; }
.du-share .bar { height: .4rem; contain: size layout; }
.du-kind { display: flex; align-items: center; gap: .5rem; margin: .1rem .2rem .45rem; padding: .2rem .25rem .2rem .7rem; width: fit-content; border-radius: var(--radius-pill); background: color-mix(in srgb, var(--accent) 14%, var(--carbon-surface2)); font-size: 13px; }
.du-kind > svg { width: .95rem; height: .95rem; color: var(--accent-glyph); }
.du-kind .btn { width: 1.7rem; height: 1.7rem; background: transparent; }
.du-map svg { position: absolute; inset: 0; }
.bk-blk rect { fill: var(--blk); transition: fill .12s; }
.bk-blk text { fill: var(--ink); font-size: 12px; pointer-events: none; }
.bk-blk text.n { font-weight: 600; }
.bk-blk text.s { opacity: .8; font-variant-numeric: tabular-nums; }
.bk-blk.dir { cursor: pointer; }
.bk-blk.dir:hover rect, .bk-blk.dir.hl rect { fill: var(--accent); }
.bk-blk.dir:hover text, .bk-blk.dir.hl text { fill: var(--accent-contrast); }
.bk-blk:not(.dir):hover rect, .bk-blk:not(.dir).hl rect { fill: var(--carbon-hover-raised); }
.bk-blk.sel rect { stroke: var(--accent); stroke-width: 2; }
@media (max-width: 900px) {
  .du-tree .du-row { grid-template-columns: minmax(10rem, 1fr) 5.5rem 7rem; }
  .du-tree .du-row > :nth-child(n+4) { display: none; }
  .du-largest .du-row { grid-template-columns: minmax(8rem, 1fr) 5.5rem 7rem; }
  .du-largest .du-path, .du-largest .du-row > :last-child { display: none; }
  .du-types .du-row { grid-template-columns: minmax(8rem, 1fr) 5.5rem 7rem; }
  .du-types .du-exts, .du-types .du-row > :nth-child(3) { display: none; }
}
`);
})();
