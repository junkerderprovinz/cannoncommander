// Screen readers: Alt+1 to Alt+4 jump between the regions, and a live region says what changed out of sight.
(() => {
  const live = document.createElement('div');
  live.id = 'cc-live';
  live.className = 'cc-sr';
  live.setAttribute('role', 'status');
  live.setAttribute('aria-live', 'polite');
  document.body.appendChild(live);
  let clear;
  // The text goes in after a short pause, so the same sentence twice is read twice.
  function say(text) {
    live.textContent = '';
    clearTimeout(clear);
    setTimeout(() => { live.textContent = text; clear = setTimeout(() => { live.textContent = ''; }, 6000); }, 60);
  }
  CC.say = say;

  const side = (i) => CC.prefs.dual ? (i ? 'Rechtes Fenster' : 'Linkes Fenster') : 'Fenster';
  const folderName = (path) => FS.LABEL[path] || FS.split(path).pop()?.replace('::', '') || 'Server';
  function where(i = Files.active) {
    const n = Files.entries(i).filter(k => !k.up).length;
    return `${side(i)}, Ordner ${folderName(Files.T(i).path)}, ${n} ${n === 1 ? 'Objekt' : 'Objekte'}`;
  }

  // After each render the state is compared with the last one, and only a change is spoken.
  let last = null;
  function watch() {
    if (CC.view !== 'files') { last = null; return; }
    const t = Files.T(), marked = Files.entries().filter(k => t.marked.has(k.name));
    const now = { panel: Files.active, path: t.path, n: marked.length };
    if (last && (now.path !== last.path || now.panel !== last.panel)) say(where());
    else if (last && now.n !== last.n) say(now.n ? `${now.n} markiert, ${fmtSize(marked.reduce((a, k) => a + FS.totalSize(k), 0)) || '0 B'}` : 'Markierung aufgehoben');
    last = now;
  }
  function label() {
    const s = $('#side');
    if (!s) return;
    s.setAttribute('aria-label', 'Seitenleiste');
    $$('.nav-row', s).forEach(r => { r.tabIndex = -1; r.setAttribute('role', 'button'); if (r.classList.contains('cur')) r.setAttribute('aria-current', 'true'); });
    $$('.panel[data-panel] .list').forEach(l => l.setAttribute('aria-label', `${side(+l.closest('.panel').dataset.panel)}: ${folderName(Files.T(+l.closest('.panel').dataset.panel).path)}`));
  }
  const render = CC.views.files.render;
  CC.views.files.render = (host) => { render(host); label(); };
  const frame = CC.render;
  CC.render = () => { frame(); watch(); };

  const seen = new Set(Jobs.list.map(j => j.id));
  Jobs.listeners.push(() => {
    for (const j of Jobs.list) {
      if (seen.has(j.id) || !['fertig', 'abgebrochen'].includes(j.state)) continue;
      seen.add(j.id);
      say(`${j.state === 'fertig' ? 'Fertig' : 'Abgebrochen'}: ${j.label}`);
    }
  });

  function toList(i) {
    if (CC.view !== 'files') CC.go('files');
    if (i === 1 && !CC.prefs.dual) CC.setPref('dual', true);
    if (CC.prefs.quickview && i !== Files.active) CC.setPref('quickview', false);
    Files.active = i;
    CC.render();
    $(`.panel[data-panel="${i}"] .list`)?.focus();
  }
  function toSide() {
    if (CC.view !== 'files') CC.go('files');
    if (innerWidth <= 760) $('#app')?.classList.add('side-open');
    else if (!CC.prefs.side) CC.setPref('side', true);
    const row = $('#side .nav-row.cur') || $('#side .nav-row');
    row?.focus();
    say('Seitenleiste. Pfeiltasten wählen, Enter öffnet');
  }
  function toJobs() {
    CC.run('jobs');
    const n = Jobs.running().length;
    setTimeout(() => ($('#jobs-body [data-job].open') || $('#jobs-mode button[aria-pressed="true"]'))?.focus(), 30);
    say(`Aufgaben, ${n ? `${n} offen` : 'gerade läuft nichts'}`);
  }
  CC.action('focusSide', { label: 'Zur Seitenleiste springen', g: 'IconMenu', key: 'Alt+1', run: toSide });
  CC.action('focusLeft', { label: 'Ins linke Fenster springen', g: 'IconViewSimple', key: 'Alt+2', run: () => toList(0) });
  CC.action('focusRight', { label: 'Ins rechte Fenster springen', g: 'IconCompare', key: 'Alt+3', run: () => toList(1) });
  CC.action('focusJobs', { label: 'Zu den Aufgaben springen', g: 'IconQueued', key: 'Alt+4', run: toJobs });
  [['Alt+1', 'focusSide'], ['Alt+2', 'focusLeft'], ['Alt+3', 'focusRight'], ['Alt+4', 'focusJobs']].forEach(([k, id]) => CC.bindKey(k, id));

  // In the sidebar the arrows walk its rows; the frame's keys would move the list's cursor instead.
  document.addEventListener('keydown', e => {
    const row = e.target.closest?.('#side .nav-row');
    if (!row || e.altKey || e.ctrlKey) return;
    const rows = $$('#side .nav-row'), i = rows.indexOf(row);
    const to = { ArrowDown: i + 1, ArrowUp: i - 1, Home: 0, End: rows.length - 1 }[e.key];
    if (to !== undefined) rows[Math.max(0, Math.min(rows.length - 1, to))].focus();
    else if (e.key === 'Enter' || e.key === ' ') row.click();
    else if (e.key === 'Escape') { $('#app')?.classList.remove('side-open'); $('.panel.active .list')?.focus(); }
    else if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
      const tog = $('[data-tog]', row);
      if (tog && tog.classList.contains('open') !== (e.key === 'ArrowRight')) {
        tog.click();
        label();
        $(`#side .nav-row[data-go="${CSS.escape(row.dataset.go)}"]`)?.focus();
      }
    }
    else return;
    e.preventDefault();
    e.stopPropagation();
  }, true);

  CC.css(`
.cc-sr { position: absolute; width: 1px; height: 1px; margin: -1px; padding: 0; overflow: hidden; clip-path: inset(50%); white-space: nowrap; border: 0; }
#side .nav-row:focus-visible { outline: 2px solid var(--accent-ink); outline-offset: -2px; }
`);
})();
