// Speicherbelegung (Alt+D): a squarified treemap of one folder beside the same items as a sorted list.
(() => {
  const { closeBtn } = CC.ui;
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

  const fileCount = (n) => n.dir ? FS.kids(n).reduce((a, k) => a + fileCount(k), 0) : 1;
  const fit = (text, px) => { const max = Math.floor(px / 7.2); return text.length <= max ? text : max > 3 ? text.slice(0, max - 1) + '…' : ''; };

  CC.action('diskUsage', { label: 'Speicherbelegung', g: 'IconDashboard', key: 'Alt+D', run: (start) => {
    let cur = start || (CC.view === 'files' && !FS.inArchive(bk.here()) ? bk.here() : '/mnt/user');
    let items = [], ro;
    const w = win({
      title: 'Speicherbelegung', full: true, cls: 'bk bk-du',
      body: `<div class="bk-crumbs" id="du-crumbs"></div>
        <div class="bk-dugrid">
          <div class="well bk-tmwell"><div class="bk-tm" id="du-map"></div></div>
          <div class="well bk-fill"><div class="bk-tools"><span class="eyebrow" id="du-sum">Inhalt</span></div><div class="bk-list" id="du-list"></div></div>
        </div>`,
      foot: [{ start: ' ' }, closeBtn, { label: 'Hier öffnen', g: 'IconFolderOpen', primary: true, run: () => Files.go(cur) }],
      onClose: () => ro?.disconnect(),
    });
    const go = (p) => { cur = p; draw(); };
    const drawMap = () => {
      const host = $('#du-map', w);
      const { width, height } = host.getBoundingClientRect();
      const visible = items.filter(it => it.v > 0);
      if (!visible.length) { host.innerHTML = `<div class="empty">${g('IconFolder')}<div>Hier liegt nichts, das Platz braucht</div></div>`; return; }
      const max = Math.max(...visible.map(it => it.v));
      host.innerHTML = `<svg width="${width}" height="${height}" viewBox="0 0 ${width} ${height}" role="img" aria-label="Belegung von ${esc(cur)}">${squarify(visible, 0, 0, width, height).map(b => {
        const it = b.it, pad = 2, x = b.x + pad, y = b.y + pad, bw = Math.max(0, b.w - pad * 2), bh = Math.max(0, b.h - pad * 2);
        const share = it.v / max, mix = it.node.dir ? Math.round(18 + share * 37) : 0;
        const fill = it.node.dir ? `color-mix(in srgb, var(--accent) ${mix}%, var(--carbon-surface3))` : 'var(--carbon-surface3)';
        const ink = it.node.dir && mix > 45 ? 'var(--accent-contrast)' : 'var(--carbon-text)';
        const name = fit(it.node.name, bw - 14), size = fit(fmtSize(it.v), bw - 14);
        return `<g class="bk-blk ${it.node.dir ? 'dir' : ''}" data-bk-du="${it.i}" data-tip="${esc(it.node.name)}, ${fmtSize(it.v)}" style="--blk:${fill};--ink:${ink}">
          <rect x="${x}" y="${y}" width="${bw}" height="${bh}" rx="6"></rect>
          ${bw > 54 && bh > 24 && name ? `<text x="${x + 8}" y="${y + 17}" class="n">${esc(name)}</text>` : ''}
          ${bw > 54 && bh > 42 && size ? `<text x="${x + 8}" y="${y + 33}" class="s">${esc(size)}</text>` : ''}</g>`;
      }).join('')}</svg>`;
    };
    const draw = () => {
      const node = FS.nodeAt(cur);
      const kids = FS.kids(node).filter(k => CC.prefs.showHidden || !bk.hidden(k));
      items = kids.map((k, i) => ({ node: k, v: k.link ? 0 : FS.totalSize(k), i })).sort((a, b) => b.v - a.v);
      items.forEach((it, i) => { it.i = i; });
      const total = items.reduce((a, it) => a + it.v, 0);
      $('#du-crumbs', w).innerHTML = bk.crumbs(cur, 'data-bk-ducd');
      $('#du-sum', w).textContent = `${bk.plural(items.length, 'Objekt', 'Objekte')}, ${fmtSize(total) || '0 B'}`;
      bk.setStart(w, `${fmtSize(total) || '0 B'} in ${bk.plural(fileCount(node), 'Datei', 'Dateien')}`);
      $('#du-list', w).innerHTML = items.length ? items.map(it => `<button class="bk-row bk-durow ${it.node.dir ? 'dir' : ''}" data-bk-du="${it.i}" ${it.node.dir ? '' : 'tabindex="-1"'}>
          ${bk.tile(it.node)}<span class="bk-name">${esc(it.node.name)}</span>
          <span class="bk-share"><span class="bar"><i style="width:${total ? (it.v / total * 100).toFixed(1) : 0}%"></i></span><span class="glim-num">${total ? Math.round(it.v / total * 100) : 0} %</span></span>
          <span class="pill glim-num">${fmtSize(it.v) || '0 B'}</span></button>`).join('')
        : `<div class="empty">${g('IconFolder')}<div>Dieser Ordner ist leer</div></div>`;
      drawMap();
    };
    w.addEventListener('click', e => {
      const c = e.target.closest('[data-bk-ducd]');
      if (c) return go(c.dataset.bkDucd);
      const b = e.target.closest('[data-bk-du]');
      if (!b) return;
      const it = items[+b.dataset.bkDu];
      if (it?.node.dir && !it.node.link) go(FS.join(cur, it.node.name));
    });
    const hl = (i, on) => $$(`[data-bk-du="${i}"]`, w).forEach(el => el.classList.toggle('hl', on));
    w.addEventListener('mouseover', e => { const b = e.target.closest('[data-bk-du]'); $$('.hl', w).forEach(x => x.classList.remove('hl')); if (b) hl(b.dataset.bkDu, true); });
    w.addEventListener('keydown', e => { if (e.key === 'Backspace' && cur !== '/' && !e.target.closest('input')) { e.preventDefault(); go(FS.parent(cur)); } });
    ro = new ResizeObserver(() => drawMap());
    ro.observe($('#du-map', w));
    draw();
  } });

  CC.css(`
.bk-du .win-body { gap: .8rem; }
.bk-dugrid { flex: 1; min-height: 0; display: grid; grid-template-columns: minmax(0, 1fr) 30rem; gap: 1rem; }
.bk-tmwell { padding: .5rem; min-height: 0; }
.bk-tm { width: 100%; height: 100%; min-height: 0; position: relative; }
.bk-tm svg { position: absolute; inset: 0; }
.bk-blk rect { fill: var(--blk); transition: fill .12s; }
.bk-blk text { fill: var(--ink); font-size: 12px; pointer-events: none; }
.bk-blk text.n { font-weight: 600; }
.bk-blk text.s { opacity: .8; font-variant-numeric: tabular-nums; }
.bk-blk.dir { cursor: pointer; }
.bk-blk.dir:hover rect, .bk-blk.dir.hl rect { fill: var(--accent); }
.bk-blk.dir:hover text, .bk-blk.dir.hl text { fill: var(--accent-contrast); }
.bk-blk:not(.dir):hover rect, .bk-blk:not(.dir).hl rect { fill: var(--carbon-hover-raised); }
.bk-durow { grid-template-columns: 1.75rem minmax(0, 1fr) 6rem 5.2rem; }
.bk-durow .pill { justify-self: end; }
.bk-durow:not(.dir) { cursor: default; }
.bk-share { display: grid; grid-template-columns: 1fr 2.4rem; gap: .45rem; align-items: center; font-size: 12px; color: var(--carbon-text-muted); text-align: right; }
.bk-share .bar { height: .4rem; contain: size layout; }
.bk-durow.hl { background: var(--carbon-surface3); }
.bk-durow.hl .bk-tile { background: var(--accent); }
.bk-durow.hl .bk-tile svg { color: var(--accent-contrast); }
@media (max-width: 900px) { .bk-dugrid { grid-template-columns: 1fr; grid-template-rows: minmax(14rem, 1fr) minmax(0, 1fr); } }
`);
})();
