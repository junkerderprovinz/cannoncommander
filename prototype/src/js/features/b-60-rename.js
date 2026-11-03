// Mehrfach umbenennen and Nach Muster markieren.
(() => {
  const { sw, seg, frow, segVal, swVal, cancel } = CC.ui;
  const bk = CC.bk;
  const { dropdown, ddVal } = CC.admin;

  const TOKENS = [['[N]', 'Name', 'Der alte Name ohne Endung'], ['[E]', 'Endung', 'Die alte Endung ohne Punkt'], ['[C]', 'Zähler', 'Zählt hoch. [C3] füllt mit Nullen auf drei Stellen auf: 001, 002'], ['[D]', 'Datum', 'Änderungsdatum als 2026-11-02'], ['[P]', 'Ordner', 'Name des Ordners, in dem die Datei liegt']];
  const isoDate = (t) => new Date(t).toISOString().slice(0, 10);
  // Metadata tokens read the same values the list's columns show.
  const META = {
    A: ['Aufnahmedatum', 'Aufnahmedatum aus den EXIF-Daten als 2026-11-02', (k) => COL.taken.value(k) && isoDate(COL.taken.num(k))],
    K: ['Kamera', 'Kameramodell aus den EXIF-Daten', (k) => COL.camera.value(k)],
    I: ['Interpret', 'Interpret aus den Tags der Musikdatei', (k) => COL.artist.value(k)],
    T: ['Titel', 'Titel aus den Tags der Musikdatei', (k) => COL.title.value(k)],
    R: ['Auflösung', 'Breite x Höhe eines Bilds oder Films, etwa 1920x1080', (k) => COL.dimensions.value(k).replace(' × ', 'x')],
    S: ['Staffel', 'Staffel aus dem Dateinamen, etwa aus S01E02. [S2] füllt auf zwei Stellen auf: 01', (k) => COL.season.value(k)],
    F: ['Folge', 'Folge aus dem Dateinamen, etwa aus S01E02. [F2] füllt auf zwei Stellen auf: 02', (k) => COL.episode.value(k)],
  };
  const META_TOKENS = Object.entries(META).map(([t, [l, tip]]) => [/[SF]/.test(t) ? `[${t}2]` : `[${t}]`, l, tip]);
  const splitName = (n) => { const dot = n.dir ? -1 : n.name.lastIndexOf('.'); return dot > 0 ? [n.name.slice(0, dot), n.name.slice(dot + 1)] : [n.name, '']; };

  // Fills a mask for one node; a metadata token the file has no value for lands in missing.
  function fill(mask, n, i, o, parent, missing = new Set()) {
    const [base, ext] = splitName(n);
    const k = { ...n, kind: FS.kindOf(n) };
    return mask.replace(/\[([NEDPAKIRT])\]|\[([CSF])(\d?)\]/g, (m, t, counted, pad) => {
      if (t === 'N') return base;
      if (t === 'E') return ext;
      if (t === 'D') return isoDate(n.time);
      if (t === 'P') return parent;
      if (counted === 'C') return String(o.start + i * o.step).padStart(+pad || 1, '0');
      const id = t || counted, v = META[id][2](k) || '';
      if (!v) missing.add(META[id][0]);
      return counted ? v.padStart(+pad || 1, '0') : v;
    });
  }
  const CASES = { keep: (s) => s, lower: (s) => s.toLowerCase(), upper: (s) => s.toUpperCase(), title: (s) => s.toLowerCase().replace(/(^|[\s_\-.(])(\p{L})/gu, (m, a, b) => a + b.toUpperCase()) };

  function preview(nodes, o, dir) {
    const parent = FS.split(dir).pop()?.replace('::', '') || '';
    let re = null, bad = false;
    if (o.find) {
      try { re = o.regex ? new RegExp(o.find, 'g') : new RegExp(o.find.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'gi'); } catch { bad = true; }
    }
    const rows = nodes.map((n, i) => {
      const missing = new Set();
      const nm = fill(o.name, n, i, o, parent, missing).trim(), ex = n.dir ? '' : fill(o.ext, n, i, o, parent, missing);
      let out = ex ? `${nm}.${ex}` : nm;
      if (re) out = out.replace(re, o.repl);
      out = CASES[o.case](out).trim();
      return { node: n, old: n.name, out, missing: [...missing] };
    });
    const moving = new Set(nodes);
    const taken = new Set(bk.kidsOf(dir).filter(k => !moving.has(k)).map(k => k.name.toLowerCase()));
    const seen = new Map();
    rows.forEach(r => seen.set(r.out.toLowerCase(), (seen.get(r.out.toLowerCase()) || 0) + 1));
    rows.forEach(r => {
      r.err = !r.out ? 'leer' : /[/\0]/.test(r.out) ? 'unzulässig' : taken.has(r.out.toLowerCase()) ? 'gibt es schon' : seen.get(r.out.toLowerCase()) > 1 ? 'doppelt' : '';
    });
    return { rows, bad };
  }

  const CASE_OPTS = [['keep', 'Wie bisher'], ['lower', 'alles klein'], ['upper', 'ALLES GROSS'], ['title', 'Jedes Wort Groß']];
  let lastOpts = null;

  // Vorlagen: a few that come along, and the ones you save, kept in the browser.
  const PRESET_KEY = 'cannoncommander-rename-presets';
  const pattern = (name, ext = '[E]', more = {}) => ({ name, ext, find: '', repl: '', regex: false, case: 'keep', ...more });
  const BUILT_IN = [
    { name: 'Fotos nach Datum', o: pattern('[A]_[C3]') },
    { name: 'Interpret - Titel', o: pattern('[I] - [T]') },
    { name: 'Serie S01E02', o: pattern('S[S2]E[F2]') },
    { name: 'Mit Auflösung', o: pattern('[N] [R]') },
  ];
  const own = (() => { try { const v = JSON.parse(localStorage.getItem(PRESET_KEY)); return Array.isArray(v) ? v : []; } catch { return []; } })();
  const storeOwn = () => { try { localStorage.setItem(PRESET_KEY, JSON.stringify(own)); } catch {} };
  const presets = () => [...BUILT_IN, ...own];
  const presetOpts = () => [['', 'Keine Vorlage'], ...presets().map((p, i) => [String(i), p.name, ...(own.includes(p) ? ['IconStar'] : [])])];
  const presetAt = (v) => v === '' || v == null ? null : presets()[+v];
  CC.action('multiRename', { label: 'Mehrfach umbenennen', g: 'IconPencil', key: 'Strg+M', needs: ['sel', 'writable'], run: () => {
    const dir = bk.here(), nodes = bk.selNodes();
    const o = { name: '[N]', ext: '[E]', start: 1, step: 1, find: '', repl: '', regex: false, case: 'keep', ...(lastOpts || {}) };
    let focusField = 'm-name', res;
    const w = win({
      title: 'Mehrfach umbenennen', wide: true, cls: 'bk bk-mren',
      body: `<div class="bk-grid2 bk-top">
          <div class="well bk-form"><div class="eyebrow">Muster</div>
            ${frow('Vorlage', `<span class="bk-range" id="m-presets">${dropdown(presetOpts(), '', 'id="m-preset"', 'grow')}<button class="btn icon" id="m-psave" data-tip="Muster als Vorlage speichern" aria-label="Muster als Vorlage speichern">${g('IconSave')}</button><button class="btn icon" id="m-pdel" data-tip="Eigene Vorlage löschen" aria-label="Eigene Vorlage löschen" disabled>${g('IconTrash')}</button></span>`, 'Eine Vorlage füllt Name, Endung, Suchen und Ersetzen und die Schreibung. Eigene Vorlagen tragen einen Stern und bleiben in diesem Browser gespeichert.')}
            ${frow('Name', `<input class="field grow mono" id="m-name" value="${esc(o.name)}" spellcheck="false" autocomplete="off">`)}
            ${frow('Endung', `<input class="field grow mono" id="m-ext" value="${esc(o.ext)}" spellcheck="false" autocomplete="off">`)}
            <div class="bk-tokens">${TOKENS.map(([t, l, tip]) => `<button class="bk-chip" data-bk-tok="${t}" data-tip="${esc(tip)}"><b>${t}</b>${l}</button>`).join('')}</div>
            <div class="bk-tokens bk-meta"><span class="muted">Aus den Metadaten</span>${META_TOKENS.map(([t, l, tip]) => `<button class="bk-chip" data-bk-tok="${t}" data-tip="${esc(tip)}"><b>${t}</b>${l}</button>`).join('')}</div>
            ${frow('Zähler', `<span class="bk-range"><span class="muted">ab</span><input class="field s glim-num" id="m-start" value="${o.start}" inputmode="numeric"><span class="muted">Schritt</span><input class="field s glim-num" id="m-step" value="${o.step}" inputmode="numeric"></span>`)}
          </div>
          <div class="well bk-form"><div class="eyebrow">Suchen und ersetzen</div>
            ${frow('Suchen', `<input class="field grow mono" id="m-find" value="${esc(o.find)}" spellcheck="false" autocomplete="off">`)}
            ${frow('Ersetzen durch', `<input class="field grow mono" id="m-repl" value="${esc(o.repl)}" spellcheck="false" autocomplete="off">`)}
            ${frow('Regulärer Ausdruck', sw(o.regex, 'id="m-regex"'), 'Suchen wird als JavaScript-Ausdruck gelesen. Im Ersetzen stehen $1, $2 für die Gruppen.')}
            ${frow('Schreibung', dropdown(CASE_OPTS, o.case, 'id="m-case"', 'grow'))}
          </div>
        </div>
        <div class="well bk-fill"><div class="bk-tools"><span class="eyebrow" id="m-count">Vorschau</span></div>
          <div class="bk-mhead"><span>Bisher</span><span></span><span>Neu</span><span></span></div>
          <div class="bk-list" id="m-list"></div></div>`,
      foot: [{ start: ' ' }, cancel, { label: 'Umbenennen', g: 'IconPencil', primary: true, run: () => apply() }],
    });
    const read = () => Object.assign(o, {
      name: $('#m-name', w).value, ext: $('#m-ext', w).value, start: parseInt($('#m-start', w).value, 10) || 0, step: parseInt($('#m-step', w).value, 10) || 1,
      find: $('#m-find', w).value, repl: $('#m-repl', w).value, regex: swVal($('#m-regex', w)), case: ddVal($('#m-case', w)),
    });
    const draw = () => {
      read();
      res = preview(nodes, o, dir);
      const changed = res.rows.filter(r => r.out !== r.old).length, errs = res.rows.filter(r => r.err).length;
      $('#m-count', w).textContent = `${bk.plural(nodes.length, 'Objekt', 'Objekte')}, ${changed} ${changed === 1 ? 'Name ändert' : 'Namen ändern'} sich`;
      bk.setStart(w, res.bad ? 'Der Ausdruck ist ungültig' : errs ? `${bk.plural(errs, 'Name passt', 'Namen passen')} nicht` : '');
      $('#m-list', w).innerHTML = res.rows.map(r => `<div class="bk-mrow ${r.out === r.old ? 'same' : ''}">
          <span class="bk-name mono">${esc(r.old)}</span><span class="bk-arrow sm">${g('IconForward')}</span>
          <span class="bk-name mono ${r.err ? '' : 'new'}">${esc(r.out) || '&nbsp;'}</span>
          ${r.err ? `<span class="pill fault">${r.err}</span>` : r.missing.length ? `<span class="pill" data-tip="${esc(`Die Datei hat keinen Wert für ${r.missing.join(', ')}. Die Stelle bleibt leer.`)}">ohne ${esc(r.missing[0])}</span>` : '<span></span>'}</div>`).join('');
      bk.primary(w).disabled = res.bad || !!errs || !changed;
    };
    const setFields = (p) => {
      Object.assign(o, p);
      for (const [id, k] of [['m-name', 'name'], ['m-ext', 'ext'], ['m-find', 'find'], ['m-repl', 'repl']]) $('#' + id, w).value = p[k];
      $('#m-regex', w).setAttribute('aria-checked', String(!!p.regex));
      const cs = $('#m-case', w), opt = CASE_OPTS.find(c => c[0] === p.case) || CASE_OPTS[0];
      cs.dataset.val = opt[0];
      cs.querySelector('span').textContent = opt[1];
    };
    const presetsChanged = (pick) => {
      $('#m-presets', w).firstElementChild.outerHTML = dropdown(presetOpts(), pick, 'id="m-preset"', 'grow');
      $('#m-pdel', w).disabled = !own.includes(presetAt(pick));
    };
    w.addEventListener('change', e => {
      if (e.target.id !== 'm-preset') return;
      const p = presetAt(ddVal(e.target));
      $('#m-pdel', w).disabled = !own.includes(p);
      if (p) setFields(p.o);
    });
    w.addEventListener('click', e => {
      if (e.target.closest('#m-psave')) {
        read();
        const saveAs = { name: o.name, ext: o.ext, find: o.find, repl: o.repl, regex: o.regex, case: o.case };
        win({ title: 'Als Vorlage speichern', body: `<div class="well">${frow('Name', `<input class="field w" id="m-pname" value="${esc(o.name)}" autocomplete="off">`, 'Unter diesem Namen findest du das Muster in der Liste der Vorlagen wieder.')}</div>`,
          foot: [cancel, { label: 'Speichern', g: 'IconSave', primary: true, run: (pw) => {
            const name = $('#m-pname', pw).value.trim();
            if (!name) return false;
            const same = own.find(p => p.name === name);
            if (same) same.o = saveAs; else own.push({ name, o: saveAs });
            storeOwn();
            presetsChanged(String(presets().findIndex(p => p.name === name && own.includes(p))));
            CC.toast(`Vorlage „${name}“ gespeichert`);
          } }],
          onOpen: (pw) => { const i = $('#m-pname', pw); setTimeout(() => i.select(), 0); i.onkeydown = (ev) => { if (ev.key === 'Enter') $('.win-foot .btn.primary', pw).click(); }; } });
      }
      if (e.target.closest('#m-pdel')) {
        const p = presetAt(ddVal($('#m-preset', w)));
        if (!own.includes(p)) return;
        own.splice(own.indexOf(p), 1);
        storeOwn();
        presetsChanged('');
        CC.toast(`Vorlage „${p.name}“ gelöscht`);
      }
    });
    function apply() {
      if (res.bad || res.rows.some(r => r.err)) return false;
      const todo = res.rows.filter(r => r.out !== r.old);
      if (!todo.length) return false;
      lastOpts = { ...o };
      const t = Files.T();
      todo.forEach(r => { r.node.name = r.out; if (t.marked.delete(r.old)) t.marked.add(r.out); });
      bk.log('umbenannt', `${bk.plural(todo.length, 'Objekt', 'Objekte')} in ${dir}`);
      CC.render();
      CC.toast(`${bk.plural(todo.length, 'Name', 'Namen')} geändert`, 'ok', { label: 'Rückgängig', run: () => {
        todo.forEach(r => { r.node.name = r.old; if (t.marked.delete(r.out)) t.marked.add(r.old); });
        CC.render();
      } });
    }
    w.addEventListener('focusin', e => { if (e.target.id === 'm-name' || e.target.id === 'm-ext') focusField = e.target.id; });
    w.addEventListener('click', e => {
      const b = e.target.closest('[data-bk-tok]');
      if (!b) return;
      const f = $('#' + focusField, w), a = f.selectionStart ?? f.value.length, z = f.selectionEnd ?? a;
      f.value = f.value.slice(0, a) + b.dataset.bkTok + f.value.slice(z);
      f.focus();
      f.setSelectionRange(a + b.dataset.bkTok.length, a + b.dataset.bkTok.length);
      draw();
    });
    w.addEventListener('input', draw);
    w.addEventListener('change', draw);
    w.addEventListener('keydown', e => { if (e.key === 'Enter' && e.target.closest('input')) { e.preventDefault(); bk.primary(w).click(); } });
    draw();
  } });

  let lastPattern = '*.jpg *.png', lastFolders = false, lastMode = 'add';
  const PRESETS = ['*.jpg *.png', '*.mkv *.mp4', '*.iso *.img', '*.zip *.7z *.zst', '*.pdf *.docx'];
  CC.action('selectPattern', { label: 'Nach Muster markieren', g: 'IconSelectAll', key: '+', run: () => {
    if (CC.view !== 'files') CC.go('files');
    const list = () => Files.entries().filter(k => !k.up);
    const matches = () => {
      const re = bk.globRe($('#p-pat', w).value);
      return re ? list().filter(k => (swVal($('#p-dirs', w)) || !k.dir) && re.test(k.name)) : [];
    };
    const w = win({
      title: 'Nach Muster markieren', cls: 'bk',
      body: `<div class="well bk-form">
          ${frow('Muster', `<input class="field grow mono" id="p-pat" value="${esc(lastPattern)}" spellcheck="false" autocomplete="off">`, 'Mehrere Muster mit Leerzeichen trennen. * steht für beliebig viele Zeichen, ? für genau eines.')}
          <div class="bk-tokens">${PRESETS.map(p => `<button class="bk-chip mono" data-bk-pre="${p}">${p}</button>`).join('')}</div>
          ${frow('Ordner einbeziehen', sw(lastFolders, 'id="p-dirs"'))}
          ${frow('Markierung', seg([['add', 'Dazunehmen'], ['replace', 'Ersetzen']], lastMode, 'id="p-mode"'), 'Dazunehmen behält, was schon markiert ist. Ersetzen hebt die alte Markierung auf.')}
        </div>`,
      foot: [{ start: ' ' }, cancel, { label: 'Markieren', g: 'IconSelectAll', primary: true, run: () => {
        const hits = matches();
        if (!hits.length) return false;
        lastPattern = $('#p-pat', w).value; lastFolders = swVal($('#p-dirs', w)); lastMode = segVal($('#p-mode', w));
        const t = Files.T();
        if (lastMode === 'replace') t.marked = new Set();
        hits.forEach(k => t.marked.add(k.name));
        CC.render();
      } }],
    });
    const count = () => { const n = matches().length; bk.setStart(w, `Passt auf ${n} von ${list().length}`); bk.primary(w).disabled = !n; };
    w.addEventListener('click', e => { const p = e.target.closest('[data-bk-pre]'); if (p) { $('#p-pat', w).value = p.dataset.bkPre; count(); } });
    w.addEventListener('input', count);
    w.addEventListener('change', count);
    w.addEventListener('keydown', e => { if (e.key === 'Enter' && e.target.id === 'p-pat') { e.preventDefault(); bk.primary(w).click(); } });
    count();
    setTimeout(() => $('#p-pat', w).select(), 0);
  } });

  CC.css(`
.bk-mren { height: min(48rem, calc(100vh - 3rem)); }
.bk-top { gap: 1rem; }
.bk-tokens { display: flex; flex-wrap: wrap; gap: .35rem; }
.bk-meta > .muted { width: 100%; font-size: 12px; }
#m-presets .a-dd { flex: 1; min-width: 0; }
#m-presets .a-dd > span { min-width: 0; }
.bk-mhead, .bk-mrow { display: grid; grid-template-columns: minmax(0, 1fr) 1.75rem minmax(0, 1fr) 7rem; gap: .6rem; align-items: center; }
.bk-mhead { font-size: 12px; color: var(--carbon-text-muted); padding: 0 .45rem; height: 1.4rem; }
.bk-mrow { min-height: 2rem; padding: 0 .45rem; border-radius: var(--radius-control); font-size: 13px; flex: none; }
.bk-mrow:hover { background: var(--carbon-surface3); }
.bk-mrow .pill { justify-self: end; }
.bk-mrow .new { color: var(--carbon-text); font-weight: 600; }
.bk-mrow > .bk-name:first-child { color: var(--carbon-text-sub); }
.bk-mrow.same .bk-name { color: var(--carbon-text-muted); font-weight: 400; }
@media (max-width: 760px) { .bk-mhead { display: none; } .bk-mrow { grid-template-columns: minmax(0, 1fr) 1.2rem minmax(0, 1fr); } .bk-mrow .pill { grid-column: 1 / -1; justify-self: start; } }
`);
})();
