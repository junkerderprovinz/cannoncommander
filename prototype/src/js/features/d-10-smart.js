// Gespeicherte Suchen: a search kept under a name sits in the sidebar and lists its current hits in the panel.
(() => {
  const { frow, cancel } = CC.ui;
  const KEY = 'castlecommander-smart';
  const base = { text: '', in: '/mnt/user', hidden: false, min: '', max: '', when: 'any', type: 'all' };
  const saved = (() => {
    try { const v = JSON.parse(localStorage.getItem(KEY)); if (Array.isArray(v)) return v; } catch {}
    return [
      { id: 1, name: 'Große Abbilder', q: { ...base, name: '*.iso *.img', min: '1 GB' } },
      { id: 2, name: 'Diesen Monat geändert', q: { ...base, when: '30' } },
    ];
  })();
  const store = () => { try { localStorage.setItem(KEY, JSON.stringify(saved)); } catch {} };
  const byId = (id) => saved.find(s => s.id === id);

  const hitsOf = (s) => CC.findFiles(s.q).hits.map(h => ({ ...h.node, kind: FS.kindOf(h.node), at: h.dir || '/' }));

  // A panel showing a saved search lists its hits instead of the folder; any step elsewhere ends that.
  const plainEntries = Files.entries;
  Files.entries = (i = Files.active) => {
    const t = Files.T(i), s = t.smart && byId(t.smart);
    if (!s) return plainEntries(i);
    const by = { size: (a, b) => FS.totalSize(a) - FS.totalSize(b), time: (a, b) => a.time - b.time }[t.sort] || ((a, b) => a.name.localeCompare(b.name, 'de', { numeric: true }));
    return hitsOf(s).sort((a, b) => (CC.prefs.foldersFirst ? (b.dir ? 1 : 0) - (a.dir ? 1 : 0) : 0) || by(a, b) * t.dir);
  };
  const plainGo = Files.go;
  Files.go = (path, i = Files.active, record) => { Files.T(i).smart = null; return plainGo(path, i, record); };

  function open(s, asTab) {
    if (CC.view !== 'files') CC.go('files');
    if (asTab) { const p = Files.P(); p.tabs.push(newTab(s.q.in)); p.cur = p.tabs.length - 1; }
    Files.go(s.q.in, Files.active, !asTab);
    Files.T().smart = s.id;
    CC.render();
  }

  function nameWin(title, value, verb, done) {
    win({ title, body: `<div class="well">${frow('Name', `<input class="field w" id="sm-name" value="${esc(value)}" autocomplete="off">`, 'So heißt die Suche in der Seitenleiste.')}</div>`,
      foot: [cancel, { label: verb, g: 'IconSave', primary: true, run: (w) => { const v = $('#sm-name', w).value.trim(); if (!v) return false; done(v); } }],
      onOpen: (w) => { const i = $('#sm-name', w); setTimeout(() => i.select(), 0); i.onkeydown = (e) => { if (e.key === 'Enter') $('.win-foot .btn.primary', w).click(); }; } });
  }

  const suggest = (q) => q.name.trim() || (q.text.trim() ? `Enthält „${q.text.trim()}“` : `Suche in ${FS.split(q.in).pop() || 'Server'}`);
  CC.smart = {
    save(q) {
      nameWin('Als intelligenten Ordner speichern', suggest(q), 'Speichern', (name) => {
        const s = { id: Math.max(0, ...saved.map(x => x.id)) + 1, name, q };
        saved.push(s);
        store();
        if (CC.view === 'files') CC.render();
        CC.toast(`„${name}“ steht jetzt in der Seitenleiste`, 'ok', { label: 'Öffnen', run: () => { while (winStack.length) CC.closeWin(); open(s); } });
      });
    },
  };

  function card() {
    const t = Files.T();
    const rows = saved.map(s => `<div class="nav-row ${t.smart === s.id ? 'cur' : ''}" data-smart="${s.id}" data-tip="${esc(`${s.q.name || 'Alle Namen'} in ${s.q.in}`)}">${g('IconFilter')}<span>${esc(s.name)}</span><span class="meta glim-num">${hitsOf(s).length}</span></div>`).join('');
    return `<section class="side-card side-smart"><span class="notch">Gespeicherte Suchen</span><button class="ptab add side-add" data-run="search" data-tip="Neue Suche (Alt+F7), dort speichern">${g('IconAdd')}</button>
      <div class="smart-list">${rows || '<div class="muted smart-none">Im Suchfenster (Alt+F7) speicherst du eine Suche als intelligenten Ordner.</div>'}</div></section>`;
  }

  function decorate() {
    $('#side .side-card')?.insertAdjacentHTML('afterend', card());
    for (let i = 0; i < (CC.prefs.dual ? 2 : 1); i++) {
      const t = Files.T(i), s = t.smart && byId(t.smart), panel = $(`.panel[data-panel="${i}"]`);
      if (!s || !panel || !$('.crumbs', panel)) continue;
      const tab = $('.ptab.cur', panel), text = tab && [...tab.childNodes].find(n => n.nodeType === Node.TEXT_NODE && n.textContent.trim());
      if (text) { text.textContent = s.name; tab.insertAdjacentHTML('afterbegin', g('IconFilter')); }
      $('.crumbs', panel).insertAdjacentHTML('afterend', `<div class="smart-bar">${g('IconFilter')}<span>Gespeicherte Suche <b>${esc(s.name)}</b>, ${Files.entries(i).length} Treffer in ${esc(s.q.in)}</span><button class="btn icon" data-smart-close data-tip="Wieder den Ordner zeigen" aria-label="Wieder den Ordner zeigen">${g('IconClose')}</button></div>`);
    }
  }
  const render = CC.views.files.render;
  CC.views.files.render = (host) => { render(host); decorate(); };

  document.addEventListener('click', e => {
    const row = e.target.closest('[data-smart]');
    if (row) return open(byId(+row.dataset.smart));
    const off = e.target.closest('[data-smart-close]');
    if (off) { const i = +off.closest('.panel').dataset.panel; Files.T(i).smart = null; Files.active = i; CC.render(); }
  });
  document.addEventListener('contextmenu', e => {
    const row = CC.view === 'files' && e.target.closest('[data-smart]');
    if (!row) return;
    e.preventDefault();
    e.stopImmediatePropagation();
    const s = byId(+row.dataset.smart);
    menu(e.clientX, e.clientY, [
      { label: 'Öffnen', g: 'IconFolderOpen', run: () => open(s) },
      { label: 'In neuem Tab öffnen', g: 'IconAdd', run: () => open(s, true) },
      '-',
      { label: 'Umbenennen', g: 'IconPencil', run: () => nameWin('Gespeicherte Suche umbenennen', s.name, 'Umbenennen', (v) => { s.name = v; store(); CC.render(); }) },
      { label: 'Entfernen', g: 'IconTrash', run: () => {
        const at = saved.indexOf(s);
        saved.splice(at, 1);
        store();
        Files.panels.forEach(p => p.tabs.forEach(t => { if (t.smart === s.id) t.smart = null; }));
        CC.render();
        CC.toast(`„${s.name}“ entfernt`, 'ok', { label: 'Rückgängig', run: () => { saved.splice(at, 0, s); store(); CC.render(); } });
      } },
    ]);
  }, true);

  CC.css(`
.side-smart .smart-list { max-height: 9.5rem; overflow: auto; }
.smart-none { font-size: 12px; padding: .2rem .55rem .3rem; }
.smart-bar { display: flex; align-items: center; gap: .55rem; margin: 0 .75rem .35rem; padding: .25rem .3rem .25rem .7rem; border-radius: var(--radius-control); background: color-mix(in srgb, var(--accent) 14%, var(--carbon-surface2)); font-size: 13px; }
.smart-bar > svg { width: 1rem; height: 1rem; color: var(--accent-glyph); flex: none; }
.smart-bar span { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.smart-bar .btn { width: 1.8rem; height: 1.8rem; background: transparent; }`);
})();
