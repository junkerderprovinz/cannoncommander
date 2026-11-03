// The everyday actions of the file view. Feature modules add the rest under the ids the menu bar lists.
const toggle = (k) => () => CC.setPref(k, !CC.prefs[k]);
const labelOf = (path) => (FS.LABEL[path] || FS.split(path).pop() || 'Server').replace('::', '');

CC.action('single', { label: 'Ein Fenster', g: 'IconViewSimple', key: 'Strg+1', checked: () => !CC.prefs.dual, run: () => CC.setPref('dual', false) });
CC.action('dual', { label: 'Zwei Fenster', g: 'IconCompare', key: 'Strg+2', checked: () => CC.prefs.dual, run: () => CC.setPref('dual', true) });
CC.action('sidebar', { label: 'Seitenleiste', g: 'IconMenu', key: 'Strg+B', checked: () => CC.prefs.side, run: () => innerWidth <= 760 ? $('#app')?.classList.toggle('side-open') : CC.setPref('side', !CC.prefs.side) });
CC.action('quickview', { label: 'Vorschau im anderen Fenster', g: 'IconEye', key: 'Strg+Q', checked: () => CC.prefs.quickview, run: () => { if (!CC.prefs.dual) CC.prefs.dual = true; CC.setPref('quickview', !CC.prefs.quickview); } });
CC.action('fkeys', { label: 'Funktionstastenleiste', g: 'IconKeyboard', checked: () => CC.prefs.fkeys, run: toggle('fkeys') });
CC.action('hidden', { label: 'Versteckte Dateien', g: 'IconEye', key: 'Strg+H', checked: () => CC.prefs.showHidden, run: toggle('showHidden') });
CC.action('extensions', { label: 'Dateiendungen', g: 'IconFileText', checked: () => CC.prefs.showExt, run: toggle('showExt') });
CC.action('syncBrowse', { label: 'Synchron navigieren', g: 'IconSync', checked: () => CC.prefs.syncBrowse, run: () => { if (!CC.prefs.dual) CC.prefs.dual = true; toggle('syncBrowse')(); CC.toast(CC.prefs.syncBrowse ? 'Das andere Fenster geht jetzt mit' : 'Die Fenster gehen wieder getrennt'); } });
CC.action('history', { label: 'Ordnerverlauf', g: 'IconLog', key: 'Alt+Runter', run: () => {
  const r = $('.panel.active .pathbar')?.getBoundingClientRect() || { left: 300, bottom: 120 };
  const list = Files.visited.filter(p => p !== Files.T().path);
  menu(r.left, r.bottom + 6, list.length ? [{ head: 'Zuletzt besucht' }, ...list.map(p => ({ label: p, g: 'IconFolder', run: () => Files.go(p) }))] : [{ head: 'Noch kein Verlauf' }]);
} });

// Krusader keeps a selection to bring back later, for instance after a copy cleared it.
CC.action('saveSel', { label: 'Auswahl merken', g: 'IconPin', needs: 'sel', run: () => { Files.savedSel = { path: Files.T().path, names: Files.sel().map(k => k.name) }; CC.toast(`${Files.savedSel.names.length} gemerkt`); } });
CC.action('restoreSel', { label: 'Auswahl wiederherstellen', g: 'IconRestore', when: () => !!Files.savedSel, run: () => {
  const s = Files.savedSel;
  if (Files.T().path !== s.path) Files.go(s.path);
  const here = new Set(Files.entries().map(k => k.name));
  Files.T().marked = new Set(s.names.filter(n => here.has(n)));
  CC.render();
} });

// A profile is the layout of both panels: their tabs, the active one and the columns.
const PROFILES = (() => { try { return JSON.parse(localStorage.getItem('cannoncommander-profiles') || '[]'); } catch { return []; } })();
const saveProfiles = () => { try { localStorage.setItem('cannoncommander-profiles', JSON.stringify(PROFILES)); } catch {} };
CC.action('profiles', { label: 'Fenster-Profile', g: 'IconDashboard', run: () => {
  const body = () => `<div class="well"><div class="eyebrow">Gespeichert</div>${PROFILES.length ? PROFILES.map((p, i) => `<div class="frow"><span class="lab">${g('IconDashboard')}<b>${esc(p.name)}</b><small>${esc(p.panels.map(x => x.tabs.map(t => FS.split(t).pop() || 'Server').join(', ')).join(' | '))}</small></span><button class="btn" data-pload="${i}">${g('IconUpload')}Laden</button><button class="btn icon" data-pdel="${i}" data-tip="Löschen">${g('IconTrash')}</button></div>`).join('') : '<div class="muted">Noch keine Profile. Gib unten einen Namen ein und speichere die jetzige Anordnung.</div>'}</div>
    <div class="well">${frow('Name', `<input class="field m" id="pname" placeholder="zum Beispiel Fotos sortieren">`, 'Speichert beide Fenster mit ihren Tabs und die Spalten der Liste.')}</div>`;
  const w = win({ title: 'Fenster-Profile', body: body(), foot: [closeBtn, { label: 'Jetzige Anordnung speichern', g: 'IconSave', primary: true, run: (el) => {
    const name = $('#pname', el).value.trim();
    if (!name) { CC.toast('Erst einen Namen eingeben', 'warn'); return false; }
    PROFILES.push({ name, columns: Columns.shown(), dual: CC.prefs.dual, panels: Files.panels.map(p => ({ cur: p.cur, tabs: p.tabs.map(t => t.path) })) });
    saveProfiles();
    CC.toast(`Profil „${name}“ gespeichert`);
  } }] });
  w.addEventListener('click', e => {
    const l = e.target.closest('[data-pload]'), d = e.target.closest('[data-pdel]');
    if (l) {
      const p = PROFILES[+l.dataset.pload];
      p.panels.forEach((x, i) => { Files.panels[i] = { cur: x.cur, tabs: x.tabs.map(newTab) }; });
      CC.prefs.dual = p.dual;
      CC.closeWin();
      CC.setPref('columns', p.columns);
      CC.toast(`Profil „${p.name}“ geladen`);
    }
    if (d) { PROFILES.splice(+d.dataset.pdel, 1); saveProfiles(); $('.win-body', w).innerHTML = body(); }
  });
} });
for (const [id, s, l] of [['sortName', 'name', 'Name'], ['sortSize', 'size', 'Größe'], ['sortTime', 'time', 'Änderungsdatum'], ['sortType', 'type', 'Dateityp']]) {
  CC.action(id, { label: l, checked: () => Files.T().sort === s, run: () => { const t = Files.T(); if (t.sort === s) t.dir *= -1; else Object.assign(t, { sort: s, dir: 1 }); CC.render(); } });
}
CC.action('swapPanels', { label: 'Fenster tauschen', g: 'IconSync', key: 'Strg+U', needs: 'dual', run: () => { Files.panels.reverse(); CC.render(); } });
CC.action('equalPanels', { label: 'Anderes Fenster hierher', g: 'IconCompare', key: 'Strg+.', needs: 'dual', run: () => Files.go(Files.T().path, Files.other()) });
CC.action('refresh', { label: 'Aktualisieren', g: 'IconRefresh', key: 'Strg+R', run: () => { CC.render(); CC.toast('Neu eingelesen'); } });
CC.action('newTab', { label: 'Neuer Tab', g: 'IconAdd', key: 'Strg+T', run: () => { const p = Files.P(); p.tabs.push(newTab(Files.T().path)); p.cur = p.tabs.length - 1; CC.render(); } });
CC.action('closeTab', { label: 'Tab schließen', g: 'IconClose', key: 'Strg+W', run: () => { const p = Files.P(); if (p.tabs.length > 1) { p.tabs.splice(p.cur, 1); p.cur = Math.max(0, p.cur - 1); CC.render(); } } });
CC.action('dupTab', { label: 'Tab duplizieren', g: 'IconCopy', run: () => { const p = Files.P(), t = Files.T(); p.tabs.splice(p.cur + 1, 0, { ...newTab(t.path), sort: t.sort, dir: t.dir, view: t.view }); p.cur += 1; CC.render(); } });
CC.action('lockTab', { label: 'Tab sperren', g: 'IconPin', checked: () => !!Files.T().locked, run: () => { Files.T().locked = !Files.T().locked; CC.render(); } });
CC.action('closeOtherTabs', { label: 'Andere Tabs schließen', g: 'IconClose', when: () => Files.P().tabs.length > 1, run: () => { const p = Files.P(); p.tabs = [p.tabs[p.cur]]; p.cur = 0; CC.render(); } });
VIEWS.forEach(([id, label, glyph]) => CC.action('view-' + id, { label, g: glyph, radio: true, checked: () => viewOf(Files.T()) === id, run: () => { Files.T().view = id; CC.render(); } }));
CC.action('nextView', { label: 'Nächste Ansicht', g: 'IconTabApps', key: 'Strg+Umschalt+V', run: () => { const t = Files.T(), k = VIEWS.findIndex(v => v[0] === viewOf(t)); t.view = VIEWS[(k + 1) % VIEWS.length][0]; CC.render(); CC.toast(`Ansicht: ${VIEWS.find(v => v[0] === t.view)[1]}`); } });

CC.action('back', { label: 'Zurück', g: 'IconBack', key: 'Alt+Links', run: () => { const t = Files.T(); if (!t.hist.length) return; t.fwd.push(t.path); Files.go(t.hist.pop(), Files.active, false); } });
CC.action('forward', { label: 'Vor', g: 'IconForward', key: 'Alt+Rechts', run: () => { const t = Files.T(); if (!t.fwd.length) return; t.hist.push(t.path); Files.go(t.fwd.pop(), Files.active, false); } });
CC.action('up', { label: 'Eine Ebene hoch', g: 'IconFolderUp', key: 'Rücktaste', run: () => Files.T().path !== '/' && Files.open(Files.active, { up: true }) });
CC.action('home', { label: 'Zu den Shares', g: 'IconFolder', run: () => Files.go('/mnt/user') });
CC.action('gotoPath', { label: 'Pfad eingeben', g: 'IconPencil', key: 'Strg+L', run: () => {
  if (CC.view !== 'files') CC.go('files');
  Files.T().editPath = true;
  CC.render();
  const i = $(`#pathedit-${Files.active}`);
  i.focus(); i.select();
  i.onkeydown = (e) => { if (e.key === 'Enter') Files.go(i.value.trim() || '/'); if (e.key === 'Escape') { Files.T().editPath = false; CC.render(); } };
  i.onblur = () => { if (Files.T().editPath) { Files.T().editPath = false; CC.render(); } };
} });
CC.action('addBookmark', { label: 'Verknüpfung anlegen', g: 'IconStar', run: (path = Files.T().path) => addBookmark(path) });
function addBookmark(path) {
  if (FS.inArchive(path)) return CC.toast('Ein Ordner im Archiv kann keine Verknüpfung werden', 'warn');
  if (DATA.bookmarks.some(b => b.path === path)) return CC.toast('Diese Verknüpfung gibt es schon', 'warn');
  const glyph = FS.LABEL[path] ? (path === '/boot' ? 'IconFlash' : path === '/mnt/remotes' ? 'IconCloud' : 'IconZFS') : 'IconFolder';
  DATA.bookmarks.push({ name: labelOf(path), path, g: glyph });
  CC.render();
  CC.toast(`Verknüpfung „${labelOf(path)}“ angelegt`);
}
function bookmarkMenu(x, y, b) {
  menu(x, y, [
    { label: 'Öffnen', g: 'IconFolderOpen', run: () => Files.go(b.path) },
    { label: 'Im anderen Fenster öffnen', g: 'IconCompare', run: () => { CC.prefs.dual = true; Files.go(b.path, Files.other()); } },
    { label: 'In neuem Tab öffnen', g: 'IconAdd', run: () => { const p = Files.P(); p.tabs.push(newTab(b.path)); p.cur = p.tabs.length - 1; CC.render(); } },
    '-',
    { label: 'Umbenennen', g: 'IconPencil', run: () => win({ title: 'Verknüpfung umbenennen', body: `<div class="well">${frow('Name', `<input class="field m" id="bmn" value="${esc(b.name)}">`)}</div>`,
      foot: [cancel, { label: 'Umbenennen', g: 'IconPencil', primary: true, run: (w) => { const v = $('#bmn', w).value.trim(); if (!v) return false; b.name = v; CC.render(); } }] }) },
    { label: 'Nach oben', g: 'IconMoveUp', run: () => { const i = DATA.bookmarks.indexOf(b); if (i > 0) { DATA.bookmarks.splice(i - 1, 0, DATA.bookmarks.splice(i, 1)[0]); CC.render(); } } },
    { label: 'Nach unten', g: 'IconMoveDown', run: () => { const i = DATA.bookmarks.indexOf(b); if (i < DATA.bookmarks.length - 1) { DATA.bookmarks.splice(i + 1, 0, DATA.bookmarks.splice(i, 1)[0]); CC.render(); } } },
    '-',
    { label: 'Verknüpfung entfernen', g: 'IconUnlink', run: () => {
      const i = DATA.bookmarks.indexOf(b);
      DATA.bookmarks.splice(i, 1);
      CC.render();
      CC.toast(`„${b.name}“ entfernt`, 'ok', { label: 'Rückgängig', run: () => { DATA.bookmarks.splice(i, 0, b); CC.render(); } });
    } },
  ]);
}
function folderItems(path) {
  return [
    { label: 'Öffnen', g: 'IconFolderOpen', run: () => Files.go(path) },
    { label: 'Im anderen Fenster öffnen', g: 'IconCompare', run: () => { CC.prefs.dual = true; Files.go(path, Files.other()); } },
    { label: 'In neuem Tab öffnen', g: 'IconAdd', run: () => { const p = Files.P(); p.tabs.push(newTab(path)); p.cur = p.tabs.length - 1; CC.render(); } },
    { label: 'Als Verknüpfung anlegen', g: 'IconStar', run: () => addBookmark(path) },
  ];
}
const folderMenu = (x, y, path) => menu(x, y, folderItems(path));
document.addEventListener('contextmenu', e => {
  if (CC.view !== 'files') return;
  const bm = e.target.closest('[data-bm]'), tree = e.target.closest('#tree [data-go]');
  if (!bm && !tree) return;
  e.preventDefault();
  e.stopImmediatePropagation();
  if (bm) bookmarkMenu(e.clientX, e.clientY, DATA.bookmarks[+bm.dataset.bm]);
  else folderMenu(e.clientX, e.clientY, tree.dataset.go);
}, true);

CC.action('selectAll', { label: 'Alles markieren', g: 'IconSelectAll', key: 'Strg+A', run: () => { const t = Files.T(); Files.entries().filter(k => !k.up).forEach(k => t.marked.add(k.name)); CC.render(); } });
CC.action('invertSel', { label: 'Auswahl umkehren', g: 'IconSync', key: '*', run: () => { const t = Files.T(); Files.entries().filter(k => !k.up).forEach(k => t.marked.has(k.name) ? t.marked.delete(k.name) : t.marked.add(k.name)); CC.render(); } });
CC.action('unselect', { label: 'Markierung aufheben', g: 'IconClearSelection', key: '-', run: () => { Files.T().marked = new Set(); CC.render(); } });
CC.action('quickFilter', { label: 'Im Ordner suchen', g: 'IconSearch', key: 'Strg+F', run: openFilter });
CC.action('copyPath', { label: 'Pfad kopieren', g: 'IconLink', needs: 'sel', run: () => { const p = Files.sel().map(k => FS.join(Files.T().path, k.name)).join('\n'); navigator.clipboard?.writeText(p); CC.toast('Pfad kopiert'); } });

CC.action('clipCut', { label: 'Ausschneiden', g: 'IconMove', key: 'Strg+X', needs: ['sel', 'writable'], run: () => { Files.clip = { cut: true, from: Files.T().path, items: Files.sel().map(k => k.name) }; CC.render(); CC.toast(`${Files.clip.items.length} ausgeschnitten`); } });
CC.action('clipCopy', { label: 'Kopieren in die Ablage', g: 'IconCopy', key: 'Strg+C', needs: 'sel', run: () => { Files.clip = { cut: false, from: Files.T().path, items: Files.sel().map(k => k.name) }; CC.toast(`${Files.clip.items.length} in der Ablage`); } });
CC.action('clipPaste', { label: 'Einfügen', g: 'IconPaste', key: 'Strg+V', needs: 'writable', when: () => !!Files.clip, run: () => {
  const c = Files.clip;
  if (!c) return CC.toast('Die Ablage ist leer', 'warn');
  const items = FS.kids(FS.nodeAt(c.from)).filter(k => c.items.includes(k.name));
  Files.transfer(c.cut ? 'move' : 'copy', items, Files.T().path, c.from);
  if (c.cut) Files.clip = null;
} });

const CLASH_RULES = [['ask', 'Fragen'], ['skip', 'Überspringen'], ['replace', 'Überschreiben'], ['newer', 'Wenn neuer'], ['both', 'Umbenennen']];
CC.action('copy', { label: 'Kopieren', g: 'IconCopy', key: 'F5', needs: 'sel', run: () => transferWindow('copy') });
CC.action('move', { label: 'Verschieben', g: 'IconMove', key: 'F6', needs: ['sel', 'writable'], run: () => transferWindow('move') });
function transferWindow(kind) {
  const sel = Files.sel();
  const dest = CC.prefs.dual ? Files.T(Files.other()).path : FS.parent(Files.T().path);
  const verb = kind === 'copy' ? 'Kopieren' : 'Verschieben';
  const total = sel.reduce((a, k) => a + FS.totalSize(k), 0);
  win({
    title: verb,
    body: `<div class="well"><div class="eyebrow">${sel.length} ${sel.length === 1 ? 'Objekt' : 'Objekte'}, ${fmtSize(total)}</div>
      <table class="tbl">${sel.slice(0, 6).map(k => `<tr><td>${esc(k.name)}</td><td class="glim-num r">${fmtSize(FS.totalSize(k))}</td></tr>`).join('')}${sel.length > 6 ? `<tr><td colspan="2">und ${sel.length - 6} weitere</td></tr>` : ''}</table></div>
      <div class="well"><div class="eyebrow">Ziel</div>
        ${frow('Ordner', CC.admin.pathField(dest, 'dest'), 'Im Zwei-Fenster-Modus steht hier der Ordner des anderen Fensters.')}
        <div class="stack">${frow('Wenn es die Datei schon gibt', seg(CLASH_RULES, 'ask', 'id="clash"'), 'Mit Fragen entscheidest du für jede Datei einzeln, sonst gilt deine Wahl hier ohne Rückfrage für alle.')}</div>
        ${frow('Rechte und Zeitstempel übernehmen', sw(true), 'Besitzer, Rechte und Änderungsdatum bleiben wie im Original. Ohne diesen Schalter gehören neue Dateien nobody:users (99:100).')}
        ${frow('Danach prüfen', sw(false), 'Liest das Ziel nach dem Kopieren noch einmal und vergleicht die Prüfsummen.')}
      </div>`,
    foot: [{ start: 'Läuft als Aufgabe im Hintergrund' }, cancel, { label: verb, g: kind === 'copy' ? 'IconCopy' : 'IconMove', primary: true, run: (w) => { Files.transfer(kind, sel, $('#dest', w).value.trim(), Files.T().path, segVal($('#clash', w))); } }],
  });
}

CC.action('delete', { label: 'Löschen', g: 'IconTrash', key: 'F8', needs: ['sel', 'writable'], run: () => {
  const sel = Files.sel(), path = Files.T().path;
  const doIt = (toTrash) => {
    const n = FS.nodeAt(path);
    n.children = n.children.filter(c => !sel.some(s => s.name === c.name));
    if (toTrash) sel.forEach(k => DATA.trash.unshift({ name: k.name, from: path, size: FS.totalSize(k), deleted: Date.now(), by: CC.user.name, dir: !!k.dir, node: k }));
    Files.T().marked = new Set();
    CC.render();
    CC.toast(`${sel.length} ${sel.length === 1 ? 'Objekt' : 'Objekte'} ${toTrash ? 'im Papierkorb' : 'gelöscht'}`, 'ok', toTrash ? { label: 'Rückgängig', run: () => { const n2 = FS.nodeAt(path); sel.forEach(k => { n2.children.push(k); DATA.trash = DATA.trash.filter(x => x.node !== k); }); CC.render(); } } : null);
  };
  if (!CC.prefs.confirmDelete) return doIt(CC.prefs.trash);
  win({
    title: 'Löschen',
    body: `<div class="well"><div class="eyebrow">${sel.length} ${sel.length === 1 ? 'Objekt' : 'Objekte'}, ${fmtSize(sel.reduce((a, k) => a + FS.totalSize(k), 0))}</div>
      <table class="tbl">${sel.slice(0, 8).map(k => `<tr><td>${esc(k.name)}</td><td class="glim-num r">${fmtSize(FS.totalSize(k))}</td></tr>`).join('')}${sel.length > 8 ? `<tr><td colspan="2">und ${sel.length - 8} weitere</td></tr>` : ''}</table>
      ${frow('In den Papierkorb', sw(CC.prefs.trash, 'id="totrash"'), `Bleibt ${CC.prefs.trashDays} Tage im Papierkorb und lässt sich wiederherstellen. Ohne den Schalter ist es sofort weg.`)}</div>`,
    foot: [cancel, { label: 'Löschen', g: 'IconTrash', primary: true, run: (w) => doIt(swVal($('#totrash', w))) }],
  });
} });

CC.action('newFolder', { label: 'Neuer Ordner', g: 'IconFolderAdd', key: 'F7', needs: 'writable', run: () => nameWindow('Neuer Ordner', 'Neuer Ordner', 'Anlegen', 'IconFolderAdd', (name) => {
  FS.nodeAt(Files.T().path).children.push({ name, dir: true, children: [], time: Date.now(), owner: 'nobody', group: 'users', mode: '0775' });
  return name;
}) });
CC.action('rename', { label: 'Umbenennen', g: 'IconPencil', key: 'F2', needs: ['sel', 'writable'], run: () => {
  const k = Files.sel()[0];
  if (Files.sel().length > 1 && CC.actions.multiRename) return CC.run('multiRename');
  nameWindow('Umbenennen', k.name, 'Umbenennen', 'IconPencil', (name) => { FS.kids(FS.nodeAt(Files.T().path)).find(c => c.name === k.name).name = name; return name; }, k.name);
} });
function nameWindow(title, value, verb, glyph, apply, old) {
  win({ title, body: `<div class="well">${frow('Name', `<input class="field w" id="nm" value="${esc(value)}">`)}</div>`,
    foot: [cancel, { label: verb, g: glyph, primary: true, run: (w) => {
      const name = $('#nm', w).value.trim();
      if (!name) return false;
      if (name !== old && FS.kids(FS.nodeAt(Files.T().path)).some(c => c.name === name)) { CC.toast('Diesen Namen gibt es hier schon', 'warn'); return false; }
      apply(name);
      CC.render();
      const idx = Files.entries().findIndex(e => e.name === name);
      if (idx >= 0) { Files.T().cursor = idx; CC.render(); }
    } }],
    onOpen: (w) => { const i = $('#nm', w); const dot = value.lastIndexOf('.'); setTimeout(() => i.setSelectionRange(0, dot > 0 ? dot : value.length), 0); i.onkeydown = (e) => { if (e.key === 'Enter') $('.win-foot .btn.primary', w).click(); }; },
  });
}

const KEYS = {
  'Ctrl+1': 'single', 'Ctrl+2': 'dual', 'Ctrl+b': 'sidebar', 'Ctrl+q': 'quickview', 'Ctrl+h': 'hidden', 'Ctrl+u': 'swapPanels', 'Ctrl+.': 'equalPanels',
  'Ctrl+r': 'refresh', 'Ctrl+t': 'newTab', 'Ctrl+w': 'closeTab', 'Alt+ArrowLeft': 'back', 'Alt+ArrowRight': 'forward', 'Ctrl+l': 'gotoPath', 'Ctrl+k': 'palette',
  'Ctrl+a': 'selectAll', '*': 'invertSel', '-': 'unselect', '+': 'selectPattern', '/': 'quickFilter', 'Ctrl+f': 'quickFilter',
  'Ctrl+x': 'clipCut', 'Ctrl+c': 'clipCopy', 'Ctrl+v': 'clipPaste',
  F1: 'shortcuts', F2: 'rename', F3: 'view', F4: 'edit', 'Shift+F4': 'newFile', F5: 'copy', F6: 'move', F7: 'newFolder', F8: 'delete', F9: 'search', F10: 'menuBar', Delete: 'delete',
  'Alt+F7': 'search', 'Alt+ArrowDown': 'history', 'Ctrl+m': 'multiRename', 'Ctrl+Shift+c': 'compareDirs', 'Ctrl+Shift+v': 'nextView', 'Alt+Enter': 'props', 'Alt+p': 'pack', 'Alt+u': 'unpack', 'Alt+d': 'diskUsage',
};
Object.entries(KEYS).forEach(([k, id]) => CC.bindKey(k, id));
