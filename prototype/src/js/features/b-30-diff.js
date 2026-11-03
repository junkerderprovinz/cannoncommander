// Dateien vergleichen: two texts side by side, changed lines tinted, a jump from one difference to the next.
(() => {
  const { closeBtn } = CC.ui;
  const bk = CC.bk;

  // Plain LCS over lines; the files here are short enough that the square table costs nothing.
  function diffLines(a, b) {
    const n = a.length, m = b.length, t = Array.from({ length: n + 1 }, () => new Uint16Array(m + 1));
    for (let i = n - 1; i >= 0; i--) for (let j = m - 1; j >= 0; j--) t[i][j] = a[i] === b[j] ? t[i + 1][j + 1] + 1 : Math.max(t[i + 1][j], t[i][j + 1]);
    const ops = [];
    let i = 0, j = 0;
    while (i < n || j < m) {
      if (i < n && j < m && a[i] === b[j]) { ops.push(['eq', i++, j++]); }
      else if (j < m && (i === n || t[i][j + 1] >= t[i + 1][j])) ops.push(['add', null, j++]);
      else ops.push(['del', i++, null]);
    }
    // A run of removals next to a run of additions reads as changed lines, paired up row by row.
    const rows = [];
    for (let k = 0; k < ops.length;) {
      if (ops[k][0] === 'eq') { rows.push({ t: 'eq', l: ops[k][1], r: ops[k][2] }); k++; continue; }
      const dels = [], adds = [];
      while (k < ops.length && ops[k][0] !== 'eq') { ops[k][0] === 'del' ? dels.push(ops[k][1]) : adds.push(ops[k][2]); k++; }
      for (let x = 0; x < Math.max(dels.length, adds.length); x++) {
        const l = dels[x] ?? null, r = adds[x] ?? null;
        rows.push({ t: l != null && r != null ? 'chg' : l != null ? 'del' : 'add', l, r });
      }
    }
    return rows;
  }

  // Marks the middle part that differs between two changed lines.
  function inline(a, b) {
    let s = 0;
    while (s < a.length && s < b.length && a[s] === b[s]) s++;
    let e = 0;
    while (e < a.length - s && e < b.length - s && a[a.length - 1 - e] === b[b.length - 1 - e]) e++;
    const mark = (x) => esc(x.slice(0, s)) + (x.length - e > s ? `<mark>${esc(x.slice(s, x.length - e))}</mark>` : '') + esc(x.slice(x.length - e));
    return [mark(a), mark(b)];
  }

  function pickPair() {
    const sel = Files.sel().filter(k => !k.dir);
    if (sel.length === 2) return [[bk.here(), bk.nodeIn(bk.here(), sel[0].name)], [bk.here(), bk.nodeIn(bk.here(), sel[1].name)]];
    if (CC.prefs.dual) {
      const a = Files.cursorEntry(0), b = Files.cursorEntry(1);
      if (a && b && !a.dir && !b.dir) return [[Files.T(0).path, bk.nodeIn(Files.T(0).path, a.name)], [Files.T(1).path, bk.nodeIn(Files.T(1).path, b.name)]];
    }
    return null;
  }

  CC.action('diff', { label: 'Dateien vergleichen', g: 'IconCompare', run: () => {
    const pair = pickPair();
    if (!pair) return CC.toast(CC.prefs.dual ? 'Zwei Dateien markieren oder in jedem Fenster eine Datei wählen' : 'Erst zwei Dateien markieren', 'warn');
    const [[da, a], [db, b]] = pair;
    const binary = bk.isBinary(a) || bk.isBinary(b);
    const ta = bk.textOf(da, a).replace(/\n$/, '').split('\n');
    const identical = a.size === b.size && Math.abs(a.time - b.time) < 2000 && a.text == null && b.text == null;
    // Without real contents the second file is the first with a few edits, so the window has something honest to show.
    const tb = b.text != null || b.content != null || (a.text != null && FS.ext(a.name) !== FS.ext(b.name)) ? bk.textOf(db, b).replace(/\n$/, '').split('\n')
      : identical ? ta.slice() : bk.variant(ta, FS.join(db, b.name), binary);
    const rows = diffLines(ta, tb);
    const hunks = rows.map((r, i) => r.t !== 'eq' && (i === 0 || rows[i - 1].t === 'eq') ? i : -1).filter(i => i >= 0);
    let at = -1;
    const cell = (n, txt, cls) => `<span class="bk-ln glim-num">${n == null ? '' : n + 1}</span><span class="bk-code ${cls}">${txt}</span>`;
    const body = rows.map((r, i) => {
      let lt = r.l != null ? esc(ta[r.l]) : '', rt = r.r != null ? esc(tb[r.r]) : '';
      if (r.t === 'chg') [lt, rt] = inline(ta[r.l], tb[r.r]);
      const lc = r.t === 'chg' ? 'chg' : r.t === 'del' ? 'del' : r.t === 'add' ? 'gap' : '';
      const rc = r.t === 'chg' ? 'chg' : r.t === 'add' ? 'add' : r.t === 'del' ? 'gap' : '';
      return `<div class="bk-dl" data-bk-row="${i}">${cell(r.l, lt || ' ', lc)}${cell(r.r, rt || ' ', rc)}</div>`;
    }).join('');
    const count = (t) => rows.filter(r => r.t === t).length;
    const head = (dir, n) => `<div class="bk-dhead"><span class="bk-tile">${g(FS.glyphOf(n))}</span><span class="bk-name">${esc(n.name)}</span><span class="bk-path muted mono"><bdi>${esc(dir)}</bdi></span><span class="pill glim-num">${fmtSize(n.size)}</span><span class="pill glim-num">${fmtTime(n.time)}</span></div>`;
    const w = win({
      title: 'Dateien vergleichen', full: true, cls: 'bk bk-diff',
      body: `<div class="bk-dheads">${head(da, a)}${head(db, b)}</div>
        <div class="well bk-fill bk-dwell">${hunks.length ? '' : `<div class="bk-same"><span class="bk-dot"></span>Die Dateien sind gleich</div>`}<div class="bk-dbody mono ${binary ? 'hex' : ''}" id="d-body" tabindex="0">${body}</div></div>`,
      foot: [{ start: hunks.length ? `${bk.plural(hunks.length, 'Stelle', 'Stellen')}: ${count('chg')} geändert, ${count('del')} entfernt, ${count('add')} dazu` : ' ' }, closeBtn,
        { label: 'Vorherige Stelle', g: 'IconMoveUp', run: () => { jump(-1); return false; } },
        { label: 'Nächste Stelle', g: 'IconMoveDown', primary: true, run: () => { jump(1); return false; } }],
    });
    const jump = (d) => {
      if (!hunks.length) return;
      at = (at + d + hunks.length) % hunks.length;
      $$('.bk-dl.cur', w).forEach(x => x.classList.remove('cur'));
      const el = $(`[data-bk-row="${hunks[at]}"]`, w);
      el.classList.add('cur');
      el.scrollIntoView({ block: 'center' });
      CC.bk.setStart(w, `Stelle ${at + 1} von ${hunks.length}`);
    };
    $$('.win-foot .btn', w).slice(1).forEach(x => { x.disabled = !hunks.length; });
    w.addEventListener('keydown', e => {
      if (e.key === 'F7' || (e.key === 'ArrowDown' && e.altKey)) { e.preventDefault(); jump(e.shiftKey ? -1 : 1); }
      if (e.key === 'ArrowUp' && e.altKey) { e.preventDefault(); jump(-1); }
    });
    setTimeout(() => $('#d-body', w).focus(), 0);
  } });

  CC.css(`
.bk-diff .win-body { gap: .7rem; }
.bk-dheads { display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; }
.bk-dhead { display: grid; grid-template-columns: 1.75rem auto minmax(0, 1fr) auto auto; gap: .55rem; align-items: center; min-width: 0; }
.bk-dhead .bk-name { font-weight: 600; }
.bk-dhead .bk-path { font-size: 12px; }
.bk-dwell { padding: .4rem 0; position: relative; }
.bk-dbody { overflow: auto; height: 100%; font-size: 12.5px; line-height: 1.55; outline: none; }
.bk-dl { display: grid; grid-template-columns: 3rem minmax(0, 1fr) 3rem minmax(0, 1fr); min-width: 0; }
.bk-ln { color: var(--carbon-text-muted); text-align: right; padding: 0 .7rem 0 0; user-select: none; }
.bk-code { white-space: pre; overflow: hidden; text-overflow: ellipsis; padding: 0 .6rem; tab-size: 4; }
.bk-code.del { background: color-mix(in srgb, #ff8389 18%, transparent); }
.bk-code.add { background: color-mix(in srgb, #6fdc8c 18%, transparent); }
.bk-code.chg { background: color-mix(in srgb, #FF832B 14%, transparent); }
.bk-code.gap { background: repeating-linear-gradient(135deg, transparent 0 6px, color-mix(in srgb, var(--carbon-text) 5%, transparent) 6px 7px); }
.bk-code mark { background: color-mix(in srgb, #FF832B 38%, transparent); color: inherit; border-radius: 3px; }
.bk-dl.cur { outline: 2px solid var(--accent-ink); outline-offset: -2px; border-radius: 4px; }
.bk-dbody.hex .bk-code { letter-spacing: .02em; }
.bk-same { display: flex; align-items: center; gap: .5rem; padding: .2rem .9rem .6rem; font-size: 13px; color: var(--carbon-text-sub); }
@media (max-width: 760px) { .bk-dheads { grid-template-columns: 1fr; } .bk-dhead .bk-path, .bk-dhead .pill:last-child { display: none; } .bk-dl { grid-template-columns: 2rem minmax(0, 1fr) 2rem minmax(0, 1fr); } }
`);
})();
