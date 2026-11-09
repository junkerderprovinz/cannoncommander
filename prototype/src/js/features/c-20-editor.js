// Bearbeiten (F4) and Neue Datei (Shift+F4): a plain text editor with a tab for the file, line numbers, find and replace,
// a markdown preview and a status bar that says how the file is written.
(() => {
  const { real, winKeys, viewKind, editable, target, textFor, md } = CC.ck;
  const opts = { wrap: false, split: true };
  // Tabs or spaces as most indented lines have them, the way the app's editor reads a file. Go and makefiles want tabs.
  const TWO = ['md', 'markdown', 'yml', 'yaml', 'json', 'js', 'ts', 'vue', 'html', 'css', 'xml'];
  const indentOf = (text, node) => {
    const e = FS.ext(node.name), fallback = e === 'go' || node.name === 'Makefile' ? 't4' : TWO.includes(e) ? 's2' : 's4';
    let tabs = 0, spaces = 0, prev = 0;
    const steps = new Map();
    for (const line of text.split('\n')) {
      if (!line.trim()) continue;
      const lead = line.match(/^[ \t]*/)[0], n = lead.startsWith('\t') ? 0 : lead.length, step = Math.abs(n - prev);
      if (lead.startsWith('\t')) tabs++;
      else if (lead.length) spaces++;
      if (step > 1 && step <= 8 && !lead.startsWith('\t')) steps.set(step, (steps.get(step) || 0) + 1);
      prev = n;
    }
    if (!tabs && !spaces) return fallback;
    if (tabs > spaces) return 't' + fallback.slice(1);
    return 's' + ([...steps].sort((a, b) => b[1] - a[1] || a[0] - b[0])[0]?.[0] ?? fallback.slice(1));
  };

  function openEditor(node, folder) {
    const isMd = viewKind(node) === 'md';
    let saved = textFor(node), dirty = false, changed = false, matches = [], hit = -1;
    const w = win({
      title: node.name, full: true, cls: 'ce-win',
      body: `<div class="ce-bar"><div class="ce-tabs" role="tablist" aria-label="Offene Dateien"><span class="ce-tab cur" role="tab" aria-selected="true"><button class="ce-tabname">${esc(node.name)}</button><button class="x" aria-label="${esc(node.name)} schließen">${g('IconClose')}</button></span>
          <button class="btn icon ce-addtab" data-tip="Datei öffnen oder anlegen" aria-label="Datei öffnen oder anlegen">${g('IconAdd')}</button></div><span class="grow"></span>
          ${isMd ? `<span class="cv-swl">Vorschau ${sw(opts.split, 'id="ce-split"')}</span>` : ''}
          <button class="btn" id="ce-findbtn" data-tip="Strg+F">${g('IconSearch')}Suchen und ersetzen</button>
          <button class="btn icon" id="ce-more" data-tip="Weitere Befehle" aria-label="Weitere Befehle">${g('IconMore')}</button></div>
        <div class="ce-main ${isMd && opts.split ? 'split' : ''}">
          <div class="ce-pane">
            <div class="ce-scroll ${opts.wrap ? 'wrap' : ''}"><div class="ce-grid"><div class="ce-back" aria-hidden="true"></div><textarea class="ce-ta" spellcheck="false" autocapitalize="off" wrap="${opts.wrap ? 'soft' : 'off'}" aria-label="Inhalt von ${esc(node.name)}"></textarea></div></div>
            <div class="ce-find glim-card" hidden>
              <div class="ce-frow"><input class="field" id="ce-q" placeholder="Suchen" aria-label="Suchen"><span class="pill glim-num" id="ce-n"></span>
                <button class="btn icon" data-hit="-1" data-tip="Vorheriger Treffer (Umschalt+Enter)" aria-label="Vorheriger Treffer">${g('IconMoveUp')}</button>
                <button class="btn icon" data-hit="1" data-tip="Nächster Treffer (Enter)" aria-label="Nächster Treffer">${g('IconMoveDown')}</button>
                <button class="btn icon" id="ce-fclose" data-tip="Schließen (Esc)" aria-label="Suche schließen">${g('IconClose')}</button></div>
              <div class="ce-frow"><input class="field" id="ce-r" placeholder="Ersetzen durch" aria-label="Ersetzen durch">
                <button class="btn" id="ce-one">Ersetzen</button><button class="btn" id="ce-all">Alle ersetzen</button></div>
              <div class="ce-frow ce-fopts">${[['case', 'Groß- und Kleinschreibung beachten'], ['word', 'Nur ganze Wörter'], ['regex', 'Regulärer Ausdruck']].map(([k, l]) => `<button class="col-check" type="button" role="checkbox" aria-checked="false" data-fopt="${k}"><span class="cbox">${g('IconCheck')}</span>${l}</button>`).join('')}</div>
            </div>
          </div>
          ${isMd ? '<div class="ce-preview"><article class="cv-md"></article></div>' : ''}
        </div>
        <div class="ce-status glim-num" role="status"><button class="ce-pos"></button><span class="grow"></span>
          ${CC.admin.dropdown([['s2', 'Leerzeichen: 2'], ['s4', 'Leerzeichen: 4'], ['s8', 'Leerzeichen: 8'], ['t2', 'Tabs: 2'], ['t4', 'Tabs: 4'], ['t8', 'Tabs: 8']], indentOf(saved, node), 'id="ce-indent" data-tip="Einrückung"', 'ce-dd')}
          ${CC.admin.dropdown([['lf', 'Unix (LF)'], ['crlf', 'Windows (CRLF)'], ['cr', 'Altes Mac OS (CR)']], 'lf', 'id="ce-eol" data-tip="Zeilenende"', 'ce-dd')}
          ${CC.admin.dropdown(['UTF-8', 'UTF-8 BOM', 'UTF-16 LE', 'UTF-16 BE', 'Windows-1252', 'ISO-8859-1', 'ISO-8859-15'].map(e => [e, e]), 'UTF-8', 'id="ce-enc" data-tip="Zeichenkodierung"', 'ce-dd')}
          ${CC.admin.dropdown([[CC.ck.langOf(node), CC.ck.langOf(node) === 'Text' ? 'Normaler Text' : CC.ck.langOf(node)]], CC.ck.langOf(node), 'id="ce-lang" data-tip="Sprache"', 'ce-dd')}
          <span class="cv-swl ce-wrapsw">Zeilen umbrechen ${sw(opts.wrap, 'id="ce-wrap"')}</span></div>`,
      foot: [{ start: ' ' }, { label: 'Schließen', g: 'IconClose', run: () => (dirty ? (ask(), false) : undefined) }, { label: 'Speichern', g: 'IconSave', primary: true, run: () => { save(); return false; } }],
      onClose: () => { if (changed && CC.view === 'files') CC.render(); },
    });
    const ta = $('.ce-ta', w), back = $('.ce-back', w), scroll = $('.ce-scroll', w), find = $('.ce-find', w), q = $('#ce-q', w), rep = $('#ce-r', w);
    const preview = $('.ce-preview .cv-md', w);
    const fopt = { case: false, word: false, regex: false };
    ta.value = saved;

    const setDirty = (d) => {
      dirty = d;
      $('.notch', w).innerHTML = `${esc(node.name)}${d ? '<i class="ce-dot" aria-label="Nicht gespeichert"></i>' : ''}`;
      $('.ce-tabname', w).innerHTML = `${esc(node.name)}${d ? '<i class="ce-tabdot" aria-label="Nicht gespeichert"></i>' : ''}`;
    };
    const lineOf = (pos) => ta.value.slice(0, pos).split('\n').length - 1;
    // The text stays in the textarea; the layer behind it draws line numbers, the caret's line and the find marks.
    const paint = () => {
      const text = ta.value, cur = lineOf(ta.selectionStart);
      const starts = [0];
      for (let i = 0; i < text.length; i++) if (text[i] === '\n') starts.push(i + 1);
      back.innerHTML = starts.map((s, li) => {
        const end = li + 1 < starts.length ? starts[li + 1] - 1 : text.length;
        let html = '', at = s;
        matches.forEach(([a, b], k) => {
          if (b <= s || a >= end) return;
          const x = Math.max(a, s), y = Math.min(b, end);
          html += esc(text.slice(at, x)) + `<mark class="${k === hit ? 'cur' : ''}">${esc(text.slice(x, y))}</mark>`;
          at = y;
        });
        html += esc(text.slice(at, end));
        return `<div class="ln ${li === cur ? 'cur' : ''}">${html || ' '}</div>`;
      }).join('');
      const col = ta.selectionStart - starts[cur] + 1;
      $('.ce-pos', w).textContent = `Zeile ${cur + 1}, Spalte ${col}${ta.selectionEnd > ta.selectionStart ? `, ${ta.selectionEnd - ta.selectionStart} markiert` : ''}`;
    };
    const reveal = (li) => {
      const el = back.children[li];
      if (!el) return;
      const top = el.offsetTop, h = el.offsetHeight;
      if (top < scroll.scrollTop + 8) scroll.scrollTop = top - 40;
      else if (top + h > scroll.scrollTop + scroll.clientHeight - 8) scroll.scrollTop = top + h - scroll.clientHeight + 40;
    };
    let mdTimer;
    const renderMd = () => { if (preview) { clearTimeout(mdTimer); mdTimer = setTimeout(() => { preview.innerHTML = md(ta.value); }, 120); } };

    const search = (keep) => {
      matches = [];
      let re = null;
      const src = fopt.regex ? q.value : q.value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
      try { re = q.value ? new RegExp(fopt.word ? `\\b(?:${src})\\b` : src, 'gmu' + (fopt.case ? '' : 'i')) : null; } catch { re = null; }
      for (let m = re && re.exec(ta.value); m; m = re.exec(ta.value)) {
        if (!m[0].length) { re.lastIndex++; continue; }
        matches.push([m.index, m.index + m[0].length]);
      }
      const needle = q.value;
      if (!keep) hit = matches.length ? Math.max(0, matches.findIndex(([a]) => a >= ta.selectionStart)) : -1;
      hit = Math.min(hit, matches.length - 1);
      $('#ce-n', w).textContent = needle ? (matches.length ? `${hit + 1} von ${matches.length}` : 'keine Treffer') : '';
      paint();
      if (hit >= 0) reveal(lineOf(matches[hit][0]));
    };
    const step = (d) => {
      if (!matches.length) return;
      hit = (hit + d + matches.length) % matches.length;
      ta.setSelectionRange(...matches[hit]);
      search(true);
    };
    const openFind = (focusReplace) => {
      const s = ta.value.slice(ta.selectionStart, ta.selectionEnd);
      if (s && !s.includes('\n')) q.value = s;
      find.hidden = false;
      (focusReplace ? rep : q).focus();
      (focusReplace ? rep : q).select();
      search();
    };
    const closeFind = () => { find.hidden = true; matches = []; hit = -1; paint(); ta.focus(); };
    const edited = () => { setDirty(ta.value !== saved); if (!find.hidden) search(true); else paint(); renderMd(); };

    $('#ce-one', w).onclick = () => {
      if (hit < 0) return;
      const [a, b] = matches[hit];
      ta.value = ta.value.slice(0, a) + rep.value + ta.value.slice(b);
      ta.setSelectionRange(a + rep.value.length, a + rep.value.length);
      edited();
      search();
    };
    $('#ce-all', w).onclick = () => {
      if (!matches.length) return;
      const n = matches.length;
      for (const [a, b] of matches.slice().reverse()) ta.value = ta.value.slice(0, a) + rep.value + ta.value.slice(b);
      edited();
      search();
      CC.toast(`${n} ${n === 1 ? 'Stelle' : 'Stellen'} ersetzt`);
    };
    q.addEventListener('input', () => search());
    q.addEventListener('keydown', (e) => { if (e.key === 'Enter') { e.preventDefault(); step(e.shiftKey ? -1 : 1); } });
    rep.addEventListener('keydown', (e) => { if (e.key === 'Enter') { e.preventDefault(); $('#ce-one', w).click(); } });
    find.addEventListener('click', (e) => { const b = e.target.closest('[data-hit]'); if (b) step(+b.dataset.hit); });
    $('#ce-fclose', w).onclick = closeFind;
    $$('[data-fopt]', w).forEach((b) => b.addEventListener('click', () => {
      const on = b.getAttribute('aria-checked') !== 'true';
      fopt[b.dataset.fopt] = on;
      b.setAttribute('aria-checked', String(on));
      b.firstElementChild.classList.toggle('on', on);
      search();
    }));
    $('#ce-findbtn', w).onclick = () => openFind();

    ta.addEventListener('input', () => { edited(); reveal(lineOf(ta.selectionStart)); });
    ['keyup', 'click', 'select'].forEach(ev => ta.addEventListener(ev, () => { paint(); if (ev === 'keyup') reveal(lineOf(ta.selectionStart)); }));
    ta.addEventListener('keydown', (e) => {
      if (e.key === 'Tab' && !e.shiftKey && !e.ctrlKey) { e.preventDefault(); document.execCommand('insertText', false, FS.ext(node.name) === 'go' ? '\t' : '  '); }
    });
    $('#ce-wrap', w).addEventListener('change', (e) => { opts.wrap = swVal(e.target); scroll.classList.toggle('wrap', opts.wrap); ta.setAttribute('wrap', opts.wrap ? 'soft' : 'off'); });
    $('#ce-split', w)?.addEventListener('change', (e) => { opts.split = swVal(e.target); $('.ce-main', w).classList.toggle('split', opts.split); });

    function save() {
      const n = real(folder, node.name) || node, t = target(n);
      t.text = saved = ta.value;
      t.size = new Blob([saved]).size;
      t.time = Date.now();
      changed = true;
      setDirty(false);
      CC.toast(`${node.name} gespeichert`);
    }
    const closeEditor = () => winStack.find(x => x.el === w)?.close();
    function ask() {
      win({
        title: 'Nicht gespeichert',
        body: `<div class="well"><div class="ce-ask">${g('IconWarning')}<span>Deine Änderungen an <b>${esc(node.name)}</b> sind noch nicht gespeichert.</span></div></div>`,
        foot: [{ label: 'Weiter bearbeiten', g: 'IconPencil', run: () => { setTimeout(() => ta.focus(), 0); } },
          { label: 'Verwerfen', g: 'IconUndo', run: () => { dirty = false; setTimeout(closeEditor, 0); } },
          { label: 'Speichern', g: 'IconSave', primary: true, run: () => { save(); setTimeout(closeEditor, 0); } }],
      });
    }
    // A click beside the window would close it; with unsaved text it asks first.
    w.addEventListener('mousedown', (e) => { if (e.target === w && dirty) { e.stopImmediatePropagation(); ask(); } }, true);
    winKeys(w, (e, combo) => {
      if (combo === 'Ctrl+s') return save(), true;
      if (combo === 'Ctrl+f') return openFind(), true;
      if (combo === 'Ctrl+h') return openFind(true), true;
      if (combo === 'F3' || combo === 'Shift+F3') return step(e.shiftKey ? -1 : 1), true;
      if (combo === 'Escape' && !find.hidden) return closeFind(), true;
      if (combo === 'Escape' && dirty) return ask(), true;
    });

    setDirty(false);
    paint();
    renderMd();
    if (preview) preview.innerHTML = md(saved);
    setTimeout(() => { ta.focus(); ta.setSelectionRange(0, 0); scroll.scrollTop = 0; paint(); }, 30);
    return w;
  }
  CC.ck.edit = openEditor;

  CC.action('edit', { label: 'Bearbeiten', g: 'IconPencil', key: 'F4', needs: ['file', 'writable'], run: () => {
    const c = CC.ck.current();
    if (!c) return;
    if (viewKind(c.node) === 'office') return CC.toast(`OnlyOffice öffnet ${c.node.name} in einem neuen Tab`);
    if (!editable(c.node)) return CC.toast('Diese Datei lässt sich nur ansehen (F3)', 'warn');
    openEditor(c.node, c.folder);
  } });

  CC.action('newFile', { label: 'Neue Datei', g: 'IconFileText', key: 'Umschalt+F4', needs: 'writable', run: () => {
    if (CC.view !== 'files') CC.go('files');
    const folder = Files.T().path;
    nameWindow('Neue Datei', 'Neue Datei.txt', 'Anlegen', 'IconFileText', (name) => {
      const n = { name, size: 0, time: Date.now(), owner: CC.user.name === 'jdp' ? 'nobody' : CC.user.name, group: 'users', mode: '0664', text: '' };
      FS.nodeAt(folder).children.push(n);
      setTimeout(() => openEditor(n, folder), 0);
      return name;
    });
  } });

  CC.css(`
.ce-win .win-body { padding-bottom: 0; }
.ce-bar { display: flex; align-items: center; gap: 1.1rem; min-height: var(--btn-h); flex: none; }
.ce-bar .grow { flex: 1; }
.scrim:has(> .ce-win) { padding-bottom: 5.25rem; }
.ce-win.full { height: calc(100vh - 6.75rem); }
.ce-main { contain: inline-size; flex: 1; min-height: 0; display: grid; grid-template-columns: 1fr; gap: 1rem; }
.ce-main.split { grid-template-columns: 1fr 1fr; }
.ce-main:not(.split) .ce-preview { display: none; }
.ce-pane { position: relative; min-width: 0; min-height: 0; display: flex; }
.ce-scroll { flex: 1; overflow: auto; border-radius: var(--radius-card); background: var(--carbon-surface2); }
.ce-grid { position: relative; min-width: 100%; width: max-content; min-height: 100%; }
.ce-scroll.wrap .ce-grid { width: 100%; }
.ce-back, .ce-ta { font-family: ui-monospace, "Cascadia Code", Menlo, monospace; font-size: 13px; line-height: 1.6; tab-size: 4; letter-spacing: 0; }
.ce-back { padding: .7rem 0 3rem; color: transparent; counter-reset: ln; pointer-events: none; }
.ce-back .ln { position: relative; padding: 0 1rem 0 4.2rem; white-space: pre; min-height: 1.6em; }
.ce-scroll.wrap .ce-back .ln { white-space: pre-wrap; overflow-wrap: anywhere; }
.ce-back .ln::before { counter-increment: ln; content: counter(ln); position: absolute; left: 0; width: 3.1rem; text-align: right; color: var(--carbon-text-muted); font-variant-numeric: tabular-nums; }
.ce-back .ln.cur { background: var(--carbon-hover); }
.ce-back .ln.cur::before { color: var(--carbon-text); }
.ce-back mark { color: transparent; background: color-mix(in srgb, var(--accent) 35%, transparent); border-radius: 3px; }
.ce-back mark.cur { background: var(--accent); }
.ce-ta { position: absolute; inset: 0; width: 100%; height: 100%; padding: .7rem 1rem 3rem 4.2rem; margin: 0; border: 0; outline: none; resize: none; overflow: hidden; background: transparent; color: var(--carbon-text); caret-color: var(--accent-ink); white-space: pre; }
.ce-scroll.wrap .ce-ta { white-space: pre-wrap; overflow-wrap: anywhere; }
.ce-ta::selection { background: color-mix(in srgb, var(--accent) 30%, transparent); }
.ce-find { position: absolute; top: .6rem; right: 1.4rem; z-index: 2; padding: .55rem; display: flex; flex-direction: column; gap: .45rem; width: min(30rem, calc(100% - 2rem)); }
.ce-find[hidden] { display: none; }
.ce-frow { display: flex; align-items: center; gap: .4rem; }
.ce-frow .field { flex: 1; min-width: 0; }
.ce-frow .pill { min-width: 4.5rem; justify-content: center; }
.ce-preview { min-width: 0; overflow: auto; border-radius: var(--radius-card); background: var(--carbon-surface2); }
.ce-dot { display: inline-block; width: .5rem; height: .5rem; border-radius: 50%; background: var(--accent-contrast); margin-left: .5rem; }
.ce-ask { display: flex; gap: .75rem; align-items: flex-start; line-height: 1.5; }
.ce-ask svg { width: 1.2rem; height: 1.2rem; color: var(--accent-glyph); flex: none; margin-top: .1rem; }
.ce-tabs { display: flex; align-items: center; gap: .35rem; min-width: 0; overflow-x: auto; scrollbar-width: none; flex: 0 1 auto; }
.ce-tab { height: 1.75rem; padding: 0 .3rem 0 .85rem; border-radius: var(--radius-pill); background: var(--carbon-surface2); color: var(--carbon-text-sub); font-size: 13px; font-weight: 600; display: inline-flex; align-items: center; gap: .25rem; white-space: nowrap; flex: none; }
.ce-tab.cur { background: var(--accent); color: var(--accent-contrast); }
.ce-tab button { all: unset; cursor: pointer; display: inline-flex; align-items: center; }
.ce-tab .ce-tabname { gap: .4rem; }
.ce-tabdot { display: inline-block; width: .45rem; height: .45rem; border-radius: 50%; background: currentColor; }
.ce-tab .x { width: 1.2rem; height: 1.2rem; border-radius: var(--radius-pill); justify-content: center; opacity: .75; }
.ce-tab .x svg { width: .55rem; height: .55rem; }
.ce-fopts { gap: .4rem 1.1rem; flex-wrap: wrap; padding: .15rem .2rem 0; font-size: 13px; }
.ce-status { display: flex; align-items: center; gap: .35rem .9rem; min-height: 2.1rem; padding: .3rem .2rem 0; flex: none; font-size: 12px; color: var(--carbon-text-sub); }
.ce-pos { all: unset; cursor: pointer; white-space: nowrap; padding: .15rem .35rem; border-radius: var(--radius-control); }
.ce-status .grow { flex: 1; }
.ce-status .ce-dd { height: 1.6rem; font-size: 12px; padding: 0 .55rem; gap: .3rem; width: auto; }
.ce-wrapsw { white-space: nowrap; }
@media (max-width: 760px) {
  .scrim:has(> .ce-win) { padding: .5rem .5rem 4.75rem; }
  .ce-win.full { height: calc(100vh - 5.25rem); max-height: none; padding: 1.4rem .75rem .75rem; }
  .ce-bar { flex-wrap: wrap; gap: .6rem 1rem; }
  .ce-bar { gap: .6rem; flex-wrap: nowrap; }
  #ce-findbtn { font-size: 0; gap: 0; width: var(--btn-h); padding: 0; }
  .ce-main.split { grid-template-columns: 1fr; grid-template-rows: 1fr 1fr; }
  .ce-find { right: .5rem; left: .5rem; width: auto; }
  .ce-frow { flex-wrap: wrap; }
  .ce-status { flex-wrap: wrap; row-gap: .2rem; }
  .ce-status .grow { flex-basis: 100%; height: 0; order: 1; }
  .ce-status > :not(.ce-pos, .grow) { order: 2; }
}
`);
})();
