// The frame around every view: head row with the menu bar, full pages, the phone bar and the keys.
const MENUS = [
  ['Datei', ['newFolder', 'newFile', 'upload', 'download', '-', 'view', 'edit', 'share', 'props', '-', 'pack', 'unpack', 'checksum', 'checksumVerify', 'split', 'combine', 'symlink', '-', 'delete', 'openTrash', '-', 'logout']],
  ['Bearbeiten', ['copy', 'move', 'rename', 'multiRename', '-', 'clipCut', 'clipCopy', 'clipPaste', '-', 'selectAll', 'invertSel', 'selectPattern', 'unselect', 'saveSel', 'restoreSel', '-', 'copyPath']],
  ['Ansicht', ['single', 'dual', 'sidebar', 'quickview', 'fkeys', 'hidden', 'extensions', 'syncBrowse', '-', { label: 'Darstellung', g: 'IconTabApps', sub: ['view-details', 'view-compact', 'view-list', 'view-content', 'view-tiles', 'view-large', 'view-gallery'] }, 'columns', { head: 'Sortieren' }, 'sortName', 'sortSize', 'sortTime', 'sortType', '-', 'swapPanels', 'equalPanels', 'profiles', 'refresh']],
  ['Gehe zu', ['back', 'forward', 'up', 'home', 'gotoPath', 'history', '-', 'BOOKMARKS', '-', 'openTrash', 'openShares', 'openActivity']],
  ['Werkzeuge', ['search', 'quickFilter', '-', 'compareDirs', 'sync', 'diff', 'duplicates', 'diskUsage', '-', 'jobs', 'mounts']],
  ['Hilfe', ['palette', 'shortcuts', 'whatsNew', 'docs', 'glyphs', '-', 'about']],
];

const Shell = {
  openIdx: -1,
  menuItems(i) {
    return MENUS[i][1].flatMap(id => id === 'BOOKMARKS'
      ? [{ head: 'Verknüpfungen' }, ...DATA.bookmarks.map(b => ({ label: b.name, g: b.g, run: () => { CC.go('files'); Files.go(b.path); } }))]
      : [id]);
  },
  openMenu(i) {
    const btn = $$('.menubar button')[i];
    if (!btn) return;
    closeMenu();
    $$('.menubar button').forEach(b => b.classList.remove('open'));
    btn.classList.add('open');
    Shell.openIdx = i;
    const r = btn.getBoundingClientRect();
    menu(r.left, r.bottom + 6, Shell.menuItems(i), { anchor: btn });
  },
  stepMenu(d) { Shell.openMenu((Shell.openIdx + d + MENUS.length) % MENUS.length); },
  updateJobs() {
    const n = Jobs.running().length, c = $('#job-count');
    if (!c) return;
    c.textContent = n;
    c.classList.toggle('idle', !n);
  },
  head() {
    const mode = CC.view === 'files';
    return `<header class="head">
      <button class="brand glim-card" data-go-view="files" data-tip="Zu den Dateien"><span class="brand-mark">${g('IconFolderOpen')}</span><span class="brand-name">CannonCommander</span></button>
      <nav class="menubar glim-card" aria-label="Menü">${MENUS.map(([n], i) => `<button data-menu="${i}">${n}</button>`).join('')}</nav>
      <button class="btn icon narrow-only" data-all-menus data-tip="Menü">${g('IconMenu')}</button>
      <span class="spacer"></span>
      ${mode ? `<div class="seg glim-card" role="group" aria-label="Ansicht">
        <button data-run="single" aria-pressed="${!CC.prefs.dual}" data-tip="Ein Fenster (Strg+1)">${g('IconViewSimple')}<span class="wide-only">Ein Fenster</span></button>
        <button data-run="dual" aria-pressed="${CC.prefs.dual}" data-tip="Zwei Fenster (Strg+2)">${g('IconCompare')}<span class="wide-only">Zwei Fenster</span></button>
      </div>` : ''}
      <button class="btn icon" data-run="search" data-tip="Suchen (Alt+F7)">${g('IconSearch')}</button>
      <button class="btn" data-run="jobs" data-tip="Aufgaben">${g('IconQueued')}<span class="wide-only">Aufgaben</span><span class="count idle glim-num" id="job-count">0</span></button>
      <button class="avatar glim-card" data-account data-tip="${esc(CC.user.full)}"><i>${esc(CC.user.name.slice(0, 2).toUpperCase())}</i></button>
      <button class="btn icon" data-go-view="settings" data-tip="Einstellungen">${g('IconGear')}</button>
    </header>`;
  },
  bottombar() {
    const tabs = [['files', 'Dateien', 'IconFiles'], ['search', 'Suchen', 'IconSearch'], ['jobs', 'Aufgaben', 'IconQueued'], ['settings', 'Einstellungen', 'IconGear']];
    return `<nav class="bottombar glim-card">${tabs.map(([id, l, gl]) => `<button data-bottom="${id}" ${CC.view === id ? 'aria-current="page"' : ''}>${g(gl)}<span>${l}</span></button>`).join('')}</nav>`;
  },
};

CC.render = () => {
  const root = $('#root');
  const v = CC.views[CC.view] ? CC.view : 'files';
  if (!CC.signedIn && CC.views.login) { root.innerHTML = ''; CC.views.login.render(root); return; }
  const def = CC.views[v];
  if (def.page) {
    root.innerHTML = `<div class="app page-app">${Shell.head()}<main class="page" id="page"></main>${Shell.bottombar()}</div>`;
    def.render($('#page'), CC.viewArg);
  } else {
    root.innerHTML = `<div class="app" id="app">${Shell.head()}<div class="main" id="main"></div><footer class="fkeys glim-card" id="fkeys"></footer>${Shell.bottombar()}</div>`;
    def.render($('#main'));
  }
  wireControls(root);
  Shell.updateJobs();
};

// A full page: cards in a centred column, back to the files in the head row.
CC.card = (title, inner, extra = '') => `<section class="card glim-card" ${extra}><span class="notch">${esc(title)}</span><div class="card-body">${inner}</div></section>`;

// The person's own order of the settings tiles; a tile the stored order does not know yet follows its predecessor.
CC.orderedTabs = () => {
  const order = CC.prefs.tabOrder || [];
  const out = order.map(id => CC.settingsTabs.find(t => t.id === id)).filter(Boolean);
  CC.settingsTabs.forEach((t, i) => {
    if (out.includes(t)) return;
    const prev = CC.settingsTabs[i - 1];
    const at = prev ? out.indexOf(prev) + 1 : 0;
    out.splice(at, 0, t);
  });
  return out;
};

// Settings: a column of tiles beside the content, the tab selector turned on its side, and only the content scrolls.
CC.registerView('settings', {
  page: true,
  render(host, tab) {
    const tabs = CC.orderedTabs();
    const cur = tabs.find(t => t.id === (tab || CC.settingsTab_)) || tabs[0];
    if (!cur) { host.innerHTML = `<div class="page-inner">${CC.card('Einstellungen', '<p class="muted">Noch keine Einstellungen gebaut.</p>')}</div>`; return; }
    CC.settingsTab_ = cur.id;
    host.innerHTML = `<div class="settings">
      <nav class="tiles" role="tablist" aria-orientation="vertical" aria-label="Einstellungen">${tabs.map(t => `<button class="stile ${t.id === cur.id ? 'cur' : ''}" role="tab" aria-selected="${t.id === cur.id}" draggable="true" data-stab="${t.id}" data-tip="${esc(t.label)}">${g(t.g)}<span>${esc(t.label)}</span></button>`).join('')}</nav>
      <div class="settings-body" id="settings-body"><div class="settings-col" id="settings-col"></div></div></div>`;
    cur.render($('#settings-col'));
    $('.stile.cur')?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
    wireControls(host);
  },
});
CC.css(`
.page-app .page:has(.settings) { overflow: hidden; }
.settings { display: grid; grid-template-columns: 12.75rem minmax(0, 1fr); gap: .5rem; height: 100%; }
.tiles { display: flex; flex-direction: column; gap: 4px; overflow: auto; padding: 0 .5rem 0 .25rem; scrollbar-width: none; }
.stile { flex: 1 0 0; min-height: 2.5rem; display: flex; align-items: center; gap: .65rem; padding: 0 .9rem; border-radius: var(--radius-control); background: var(--carbon-surface2); color: var(--carbon-text); font-size: 15px; line-height: 20px; text-align: left; }
.stile svg { width: 1.25rem; height: 1.25rem; flex: none; color: var(--accent-glyph); }
.stile span { display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; text-wrap: balance; }
.stile:hover { background: var(--carbon-surface3); }
.stile.cur { background: var(--accent); color: var(--accent-contrast); }
.stile.cur svg { color: var(--accent-contrast); }
.stile.dragging { opacity: .4; }
.settings-body { overflow: auto; min-height: 0; padding: 0 2rem 2rem; }
.settings-col { max-width: 52rem; display: flex; flex-direction: column; gap: 2.5rem; padding-top: .8rem; }
/* A wide screen sets the cards in two balanced columns; the margin keeps room for the notch of the card below. */
@media (min-width: 1500px) {
  .settings-col { display: block; max-width: 106rem; columns: 2; column-gap: 2rem; }
  .settings-col > * { break-inside: avoid; margin-bottom: 2.5rem; }
}
@media (max-width: 1024px) { .settings { grid-template-columns: 3.25rem minmax(0, 1fr); } .stile { justify-content: center; padding: 0; } .stile span { display: none !important; } }
@media (max-width: 600px) {
  .settings { grid-template-columns: minmax(0, 1fr); grid-template-rows: auto minmax(0, 1fr); }
  .tiles { flex-direction: row; padding: 0 0 .25rem; }
  .stile { flex: none; width: 2.75rem; }
  .stile.cur { width: auto; padding: 0 .9rem; gap: .5rem; }
  .stile.cur span { display: inline !important; white-space: nowrap; }
  .settings-body { padding: 0 .25rem 2rem; }
}
`);

// A tile can be dragged to another place in the column; the order is kept like any other pref.
let tileDrag = null;
document.addEventListener('dragstart', e => { const t = e.target.closest?.('.stile'); if (!t) return; tileDrag = t.dataset.stab; t.classList.add('dragging'); e.dataTransfer.effectAllowed = 'move'; });
document.addEventListener('dragover', e => {
  const t = e.target.closest?.('.stile');
  if (!tileDrag || !t || t.dataset.stab === tileDrag) return;
  e.preventDefault();
  const col = t.parentElement, moving = col.querySelector(`[data-stab="${tileDrag}"]`);
  const r = t.getBoundingClientRect();
  col.insertBefore(moving, e.clientY < r.top + r.height / 2 ? t : t.nextSibling);
});
document.addEventListener('dragend', () => {
  if (!tileDrag) return;
  tileDrag = null;
  const ids = $$('.tiles .stile').map(t => t.dataset.stab);
  CC.setPref('tabOrder', ids);
});

// One listener for the whole frame; views add their own on top.
document.addEventListener('click', e => {
  const t = e.target;
  if (!t.closest('#menu, #submenu') && !t.closest('.menubar')) closeMenu();
  const m = t.closest('[data-menu]');
  if (m) { const i = +m.dataset.menu; if (m.classList.contains('open')) closeMenu(); else Shell.openMenu(i); return; }
  const r = t.closest('[data-run]');
  if (r) { CC.run(r.dataset.run); return; }
  const gv = t.closest('[data-go-view]');
  if (gv) { CC.go(gv.dataset.goView); return; }
  if (t.closest('[data-all-menus]')) {
    // Without room for the menu bar, every menu stacks into one list under its own heading.
    const items = MENUS.flatMap(([n], i) => [{ head: n }, ...Shell.menuItems(i).filter(x => typeof x !== 'object' || !x.head)]);
    const r2 = t.closest('[data-all-menus]').getBoundingClientRect();
    menu(r2.left, r2.bottom + 6, items);
    return;
  }
  const st = t.closest('[data-stab]');
  if (st) { CC.go('settings', st.dataset.stab); return; }
  const bt = t.closest('[data-bottom]');
  if (bt) { const id = bt.dataset.bottom; id === 'files' || id === 'settings' ? CC.go(id) : CC.run(id); return; }
  if (t.closest('[data-account]')) {
    const r2 = t.closest('[data-account]').getBoundingClientRect();
    menu(r2.right - 260, r2.bottom + 6, [{ head: `${CC.user.full}, ${CC.user.role}` }, { label: 'Konto & Sicherheit', g: 'IconKey', run: () => CC.go('settings', 'security') }, { label: 'Aktivität', g: 'IconLog', run: () => CC.run('openActivity') }, '-', 'logout']);
  }
});
document.addEventListener('mouseover', e => {
  const m = e.target.closest('[data-menu]');
  if (m && $('#menu')?._anchor && !m.classList.contains('open')) Shell.openMenu(+m.dataset.menu);
});

// Keys: windows and menus first, then the active view.
CC.keymap = {};
CC.bindKey = (combo, id) => { CC.keymap[combo] = id; };
// Shift counts for named keys and with Ctrl or Alt; on its own it only picks the character, such as * or +.
const comboOf = (e) => {
  const mod = e.ctrlKey || e.metaKey || e.altKey;
  return [e.ctrlKey || e.metaKey ? 'Ctrl' : '', e.altKey ? 'Alt' : '', e.shiftKey && (e.key.length > 1 || mod) ? 'Shift' : '', e.key.length === 1 ? e.key.toLowerCase() : e.key].filter(Boolean).join('+');
};
document.addEventListener('keydown', e => {
  if ($('#menu')) { if (e.key === 'Escape') { e.preventDefault(); closeMenu(); } return; }
  if (winStack.length) { if (e.key === 'Escape') { e.preventDefault(); CC.closeWin(); } return; }
  if (e.key === 'F10') { e.preventDefault(); Shell.openMenu(0); return; }
  if (e.target.closest('input, textarea, select, [contenteditable]')) return;
  const combo = comboOf(e);
  const id = CC.keymap[combo];
  if (id && CC.actions[id]) { e.preventDefault(); CC.run(id); return; }
  CC.views[CC.view]?.key?.(e, combo);
});
addEventListener('resize', () => CC.view === 'files' && CC.render());

// Head-row and help actions that belong to no single view.
CC.action('jobs', { label: 'Aufgaben', g: 'IconQueued', run: jobsWindow });
CC.action('settings', { label: 'Einstellungen', g: 'IconGear', run: () => CC.go('settings') });
CC.action('shortcuts', { label: 'Tastenkürzel', g: 'IconKeyboard', key: 'F1', run: shortcutsWindow });
CC.action('menuBar', { label: 'Menü', g: 'IconMenu', key: 'F10', run: () => Shell.openMenu(0) });

function jobsWindow() {
  const body = () => Jobs.list.length ? Jobs.list.map(j => `<div class="job"><div><div>${esc(j.label)}</div><div class="sub glim-num" style="font-size:12px">${j.total ? `${fmtSize(j.total * j.pct / 100)} von ${fmtSize(j.total)}, ` : ''}${j.state}</div></div>
      <div class="bar"><i style="width:${j.pct}%"></i></div><span class="pill glim-num">${Math.round(j.pct)} %</span>
      <span class="job-acts">${j.state === 'läuft' || j.state === 'pausiert' ? `<button class="btn icon" data-jp="${j.id}" data-tip="${j.state === 'läuft' ? 'Anhalten' : 'Weiter'}">${g(j.state === 'läuft' ? 'IconPause' : 'IconPlay')}</button><button class="btn icon" data-jc="${j.id}" data-tip="Abbrechen">${g('IconStop')}</button>` : ''}</span></div>`).join('')
    : `<div class="empty">${g('IconQueued')}<div>Gerade läuft nichts</div></div>`;
  const w = win({
    title: 'Aufgaben', wide: true,
    body: `<div class="well" id="jobs-body">${body()}</div>`,
    foot: [{ start: 'Aufgaben laufen auf dem Server weiter, auch wenn du die Seite schließt' }, { label: 'Fertige entfernen', g: 'IconEmpty', run: () => { Jobs.list = Jobs.running(); Jobs.changed(); return false; } }, closeBtn],
    onClose: () => { Jobs.listeners = Jobs.listeners.filter(f => f !== refresh); },
  });
  const refresh = () => { const b = $('#jobs-body', w); if (b) b.innerHTML = body(); };
  Jobs.listeners.push(refresh);
  w.addEventListener('click', e => {
    const p = e.target.closest('[data-jp]'), c = e.target.closest('[data-jc]');
    const find = (id) => Jobs.list.find(j => String(j.id) === id);
    if (p) Jobs.pause(find(p.dataset.jp));
    if (c) Jobs.cancel(find(c.dataset.jc));
  });
}
CC.css(`.job { display: grid; grid-template-columns: 1fr 9rem auto auto; gap: .9rem; align-items: center; min-height: 2.6rem; } .job-acts { display: flex; gap: .3rem; }`);

function shortcutsWindow() {
  const rows = Object.values(CC.actions).filter(a => a.key).sort((a, b) => (a.menu || '').localeCompare(b.menu || ''));
  const extra = [['Tab', 'Fenster wechseln'], ['Enter', 'Öffnen'], ['Einfg', 'Markieren und weiter'], ['Leertaste', 'Schnellansicht, darin I für Dateiinfos'], ['Strg+Klick', 'Zur Auswahl'], ['Umschalt+Klick', 'Bereich auswählen'], ['Rechtsklick', 'Kontextmenü'], ['F10', 'Menüleiste'], ['Esc', 'Filter, Menü oder Fenster schließen']];
  win({ title: 'Tastenkürzel', wide: true,
    body: `<div class="well"><div class="kbd-grid">${[...extra.map(([k, l]) => [l, k]), ...rows.map(a => [typeof a.label === 'function' ? a.label() : a.label, a.key])].map(([l, k]) => `<div><span>${esc(l)}</span><span class="kbd">${esc(k)}</span></div>`).join('')}</div></div>`,
    foot: [closeBtn] });
}
CC.css(`.kbd-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: .2rem 1.5rem; font-size: 13px; } .kbd-grid div { display: flex; justify-content: space-between; gap: 1rem; align-items: center; height: 1.9rem; }`);
