// The file manager itself: one or two panels, each with its own tabs, history, selection and filter.
// Words in a search match anywhere in the name; a word with * or ? is a pattern for the whole name, so *.mkv finds
// every film, and a word starting with # finds a tag or a colour.
function nameMatcher(query) {
  const parts = query.trim().toLowerCase().split(/\s+/).filter(Boolean).map(p => /[*?]/.test(p)
    ? new RegExp('^' + p.replace(/[.+^${}()|[\]\\]/g, '\\$&').replace(/\*/g, '.*').replace(/\?/g, '.') + '$', 'i')
    : p);
  return (k) => parts.some(p => typeof p !== 'string' ? p.test(k.name) : p.startsWith('#') ? Labels.matches(k, p) : k.name.toLowerCase().includes(p));
}
const newTab = (path) => ({ path, marked: new Set(), cursor: 0, sort: 'name', dir: 1, filter: '', qf: false, hist: [], fwd: [], editPath: false });

const Files = {
  active: 0,
  panels: [
    { tabs: [newTab('/mnt/user/downloads'), newTab('/mnt/user/fotos/2026')], cur: 0 },
    { tabs: [newTab('/mnt/user/backups')], cur: 0 },
  ],
  openTree: new Set(['/mnt', '/mnt/user']),
  visited: [],
  following: false,
  marks: [{}, {}],
  clip: null,
  // Feature modules add rows under the shortcuts, each a function that returns its HTML or nothing.
  sideRows: [],
  P: (i = Files.active) => Files.panels[i],
  T: (i = Files.active) => Files.panels[i].tabs[Files.panels[i].cur],
  other: () => (CC.prefs.dual ? 1 - Files.active : Files.active),
  entries(i = Files.active) {
    const t = Files.T(i);
    const shown = (k) => CC.prefs.showHidden || (!k.hidden && !k.name.startsWith('.'));
    let list = FS.kids(FS.nodeAt(t.path)).filter(shown).map(k => ({ ...k, kind: FS.kindOf(k) }));
    if (t.filter && t.deep !== false) list = Files.search(t.path, t.filter, shown);
    else if (t.flat) list = Files.search(t.path, '*', shown).filter(k => !k.dir);
    else if (t.filter) list = list.filter(k => nameMatcher(t.filter)({ ...k, at: t.path }));
    const by = {
      name: (a, b) => a.name.localeCompare(b.name, 'de', { numeric: true }),
      size: (a, b) => FS.totalSize(a) - FS.totalSize(b),
      time: (a, b) => a.time - b.time,
      type: (a, b) => FS.ext(a.name).localeCompare(FS.ext(b.name)) || a.name.localeCompare(b.name, 'de'),
    }[t.sort] || Columns.comparer(t.sort) || ((a, b) => a.name.localeCompare(b.name, 'de', { numeric: true }));
    list.sort((a, b) => (CC.prefs.foldersFirst ? (b.dir ? 1 : 0) - (a.dir ? 1 : 0) : 0) || by(a, b) * t.dir);
    const grp = GROUPS[groupOf(t)];
    if (grp) list.sort((a, b) => grp.rank(a) - grp.rank(b));
    return list;
  },
  // The search walks the whole tree below the folder; each hit remembers the folder it was found in.
  search(root, query, shown) {
    const match = nameMatcher(query), hits = [];
    const walk = (n, path) => {
      for (const k of FS.kids(n).filter(shown)) {
        if (hits.length >= 2000) return;
        if (match({ ...k, at: path })) hits.push({ ...k, kind: FS.kindOf(k), at: path });
        if (k.dir && !k.link) walk(k, FS.join(path, k.name));
      }
    };
    walk(FS.nodeAt(root), root);
    return hits;
  },
  sel(i = Files.active) {
    const t = Files.T(i), list = Files.entries(i);
    const marked = list.filter(k => t.marked.has(k.name));
    if (marked.length) return marked;
    const c = list[t.cursor];
    return c && !c.up ? [c] : [];
  },
  cursorEntry: (i = Files.active) => Files.entries(i)[Files.T(i).cursor],
  go(path, i = Files.active, record = true) {
    const n = FS.nodeAt(path);
    if (!n || !n.dir) return CC.toast('Diesen Ordner gibt es nicht', 'warn');
    const t = Files.T(i), from = t.path;
    // A locked tab keeps its folder; going elsewhere from it opens the new folder in a tab beside it, as in Krusader.
    if (record && t.locked && t.path !== path && !Files.following) { const p = Files.P(i); p.tabs.splice(p.cur + 1, 0, newTab(path)); p.cur += 1; return Files.go(path, i, false); }
    if (record && t.path !== path) { t.hist.push(t.path); t.fwd = []; if (!Files.following) Files.pushNav(i); }
    Object.assign(t, { path, marked: new Set(), cursor: 0, filter: '', qf: false, editPath: false, flat: false });
    Files.visited = [path, ...Files.visited.filter(p => p !== path)].slice(0, 20);
    // Synchronised browsing: one step down or up in this panel takes the other panel the same step, where it can follow.
    if (CC.prefs.syncBrowse && CC.prefs.dual && record && !Files.following) {
      const o = Files.T(1 - i);
      const target = FS.parent(path) === from ? FS.join(o.path, FS.split(path).pop()) : FS.parent(from) === path ? FS.parent(o.path) : null;
      if (target && FS.nodeAt(target)?.dir) { Files.following = true; Files.go(target, 1 - i); Files.following = false; }
    }
    FS.split(path).forEach((_, k, a) => { const p = '/' + a.slice(0, k + 1).join('/'); if (!p.includes('::')) Files.openTree.add(p); });
    Files.marks = [{}, {}];
    if (CC.view !== 'files') CC.go('files'); else CC.render();
  },
  open(i, k) {
    const t = Files.T(i);
    // A hit from deeper down opens where it lives, so everything after works on real paths again.
    if (k.at && k.at !== t.path) {
      if (k.dir) return Files.go(FS.join(k.at, k.name), i);
      Files.go(k.at, i);
      const idx = Files.entries(i).findIndex(e => e.name === k.name);
      Files.T(i).cursor = Math.max(0, idx);
      CC.render();
      return Files.open(i, Files.entries(i)[idx]);
    }
    if (k.up) {
      const from = FS.split(t.path).pop().replace('::', '');
      Files.go(FS.parent(t.path), i);
      const idx = Files.entries(i).findIndex(e => e.name === from);
      if (idx >= 0) { Files.T(i).cursor = idx; CC.render(); }
      return;
    }
    if (k.link) { const target = FS.nodeAt(k.link); if (target?.dir) return Files.go(k.link, i); }
    if (k.dir) return Files.go(FS.join(t.path, k.name), i);
    if (k.zip) return Files.go(FS.join(t.path, k.name + '::'), i);
    const how = CC.prefs.dblclick;
    if (how === 'download') return CC.run('download');
    if (how === 'open' && ['text', 'code'].includes(FS.kindOf(k)) && CC.actions.edit) return CC.run('edit');
    CC.run('view');
  },
  // Copy or move as a background job; the preset rule or the conflict window decides what happens to names that already exist.
  async transfer(kind, items, dest, from = Files.T().path, rule = 'ask') {
    const target = FS.nodeAt(dest);
    if (!target || !target.dir || FS.inArchive(dest)) return CC.toast('Diesen Zielordner gibt es nicht', 'warn');
    if (dest === from) return CC.toast('Quelle und Ziel sind derselbe Ordner', 'warn');
    const clashes = items.filter(k => FS.kids(target).some(c => c.name === k.name));
    let decide = {};
    if (clashes.length) {
      decide = CC.resolveConflicts ? await CC.resolveConflicts(clashes, dest, rule) : Object.fromEntries(clashes.map(k => [k.name, 'replace']));
      if (!decide) return;
    }
    const total = items.reduce((a, k) => a + FS.totalSize(k), 0);
    const label = `${kind === 'copy' ? 'Kopieren' : 'Verschieben'}: ${items.length === 1 ? items[0].name : items.length + ' Objekte'} nach ${FS.split(dest).pop()}`;
    Jobs.start({ kind, label, total, files: Jobs.filesOf(items), done: () => {

      const placed = [], replaced = [], moved = [];
      for (const k of items) {
        const choice = decide[k.name] || 'replace';
        if (choice === 'skip') continue;
        // The server keeps each file's own time, so a copy is no newer than what it copies.
        const copy = JSON.parse(JSON.stringify(k));
        delete copy.kind;
        if (choice === 'both') copy.name = Files.freeName(target, k.name);
        else { replaced.push(...target.children.filter(c => c.name === copy.name)); target.children = target.children.filter(c => c.name !== copy.name); }
        target.children.push(copy);
        placed.push(copy);
        if (kind === 'move') { const src = FS.nodeAt(k.at || from); moved.push([src, src.children.find(c => c.name === k.name)]); src.children = src.children.filter(c => c.name !== k.name); }
      }
      Files.T().marked = new Set();
      DATA.activity.unshift({ t: Date.now(), who: CC.user.name, what: kind === 'copy' ? 'kopiert' : 'verschoben', obj: `${items.length} ${items.length === 1 ? 'Objekt' : 'Objekte'} nach ${dest}` });
      CC.toast(label.replace('Kopieren', 'Kopiert').replace('Verschieben', 'Verschoben'), 'ok', { label: 'Rückgängig', run: () => {
        target.children = [...target.children.filter(c => !placed.includes(c)), ...replaced];
        for (const [src, n] of moved) if (n) src.children.push(JSON.parse(JSON.stringify(n)));
        CC.render();
      } });
      if (CC.view === 'files') CC.render();
    } });
    CC.toast(`${label} läuft`, 'ok', { label: 'Aufgaben', run: () => CC.run('jobs') });
  },
  freeName(dir, name) {
    const dot = name.lastIndexOf('.'), base = dot > 0 ? name.slice(0, dot) : name, ext = dot > 0 ? name.slice(dot) : '';
    for (let n = 1; ; n++) { const c = `${base} (${n})${ext}`; if (!FS.kids(dir).some(k => k.name === c)) return c; }
  },
};

function renderFiles(host) {
  const app = $('#app');
  app.classList.toggle('dual', CC.prefs.dual);
  app.classList.toggle('no-side', !CC.prefs.side);
  app.classList.toggle('with-fkeys', CC.prefs.fkeys);
  if (!CC.prefs.dual) Files.active = 0;
  host.innerHTML = `<aside class="side" id="side">
      <section class="side-card"><span class="notch">Verknüpfungen</span><button class="ptab add side-add" data-run="addBookmark" data-tip="Aktuellen Ordner als Verknüpfung anlegen">${g('IconAdd')}</button><div id="shortcuts"></div></section>
      <section class="side-card side-tree"><span class="notch">Ordnerbaum</span><div id="tree"></div></section>
    </aside><div class="panels" id="panels"></div>`;
  renderSide();
  const panels = $('#panels');
  for (let i = 0; i < (CC.prefs.dual ? 2 : 1); i++) panels.appendChild(CC.prefs.quickview && CC.prefs.dual && i !== Files.active ? renderQuickview(i) : renderPanel(i));
  renderFkeys();
  $('.panel.active .list')?.focus({ preventScroll: true });
  $$('.list.view-columns').forEach(l => { l.scrollLeft = l.scrollWidth; });
}

// How full a volume is, red from nine tenths on; the free space stays as words beside it.
const volMeter = (p) => {
  if (!FS.TOTAL[p]) return '';
  const used = 1 - FS.FREE[p] / FS.TOTAL[p];
  return `<span class="vmeter ${used > .9 ? 'full' : ''}" title="${fmtSize(FS.FREE[p])} frei von ${fmtSize(FS.TOTAL[p])}"><i style="width:${Math.round(used * 100)}%"></i></span>`;
};

function renderSide() {
  const cur = Files.T().path;
  $('#shortcuts').innerHTML = DATA.bookmarks.map((s, i) => `<div class="nav-row ${cur === s.path ? 'cur' : ''}" data-go="${esc(s.path)}" data-bm="${i}">${g(s.g)}<span>${esc(s.name)}</span><button class="unpin" data-unpin="${i}" data-tip="Vom Schnellzugriff lösen" aria-label="${esc(s.name)} vom Schnellzugriff lösen">${g('IconPin')}</button></div>`).join('')
    + `<div class="nav-row" data-run="openTrash">${g('IconTrash')}<span>Papierkorb</span><span class="meta glim-num">${DATA.trash.length}</span></div>`
    + Files.sideRows.map(f => f()).join('');
  const rows = [];
  const walk = (n, path, depth) => {
    for (const k of FS.kids(n).filter(k => k.dir)) {
      const p = FS.join(path, k.name);
      const hasKids = FS.kids(k).some(c => c.dir);
      const open = Files.openTree.has(p);
      const glyph = p === '/mnt' ? 'IconDatabase' : p === '/boot' ? 'IconFlash' : p === '/remotes' ? 'IconCloud' : k.remote ? 'IconNetwork' : FS.FREE[p] ? 'IconZFS' : open ? 'IconFolderOpen' : 'IconFolder';
      rows.push(`<div class="nav-row tree-row ${cur === p ? 'cur' : ''}" style="padding-left:${.3 + depth * .85}rem" data-go="${esc(p)}" ${k.remote ? `data-tip="${esc(k.remote)}"` : ''}>
        ${hasKids ? `<button class="tree-tog ${open ? 'open' : ''}" data-tog="${esc(p)}" aria-label="${open ? 'Zuklappen' : 'Aufklappen'}">${g('IconExpand')}</button>` : '<span class="tree-tog"></span>'}
        ${g(glyph)}<span>${esc(FS.LABEL[p] || k.name)}</span>${FS.FREE[p] ? `<span class="meta glim-num">${fmtSize(FS.FREE[p])} frei</span>` : ''}</div>`);
      if (open) walk(k, p, depth + 1);
    }
  };
  walk(ROOT, '/', 0);
  $('#tree').innerHTML = rows.join('');
}

function renderPanel(i) {
  const p = Files.P(i), t = Files.T(i), list = Files.entries(i);
  if (t.cursor >= list.length) t.cursor = Math.max(0, list.length - 1);
  const el = document.createElement('section');
  el.className = 'panel glim-card' + (i === Files.active ? ' active' : '');
  el.dataset.panel = i;
  const parts = FS.split(t.path);
  const sortGlyph = (s) => t.sort === s ? g(t.dir > 0 ? 'IconMoveUp' : 'IconMoveDown') : '';
  // The tabs are the panel's heading badges on its top edge.
  el.innerHTML = `
    <div class="ptabs">${p.tabs.map((tb, k) => {
      const n = (FS.LABEL[tb.path] || FS.split(tb.path).pop() || 'Server').replace('::', '');
      const open = k === p.cur;
      return `<button class="ptab ${open ? 'cur' : ''}" data-tab="${k}" data-tip="${esc(tb.path)}${tb.locked ? ', gesperrt' : ''}">${tb.locked ? g('IconPin') : ''}${esc(n)}${p.tabs.length > 1 ? `<span class="x" data-close="${k}" data-tip="Tab schließen (Strg+W)">${g('IconClose')}</span>` : ''}</button>`;
    }).join('')}<button class="ptab add" data-newtab data-tip="Neuer Tab (Strg+T)">${g('IconAdd')}</button></div>
    <div class="crumbs">
      <button class="btn icon" data-run="back" data-tip="Zurück (Alt+Links)">${g('IconBack')}</button>
      ${t.editPath ? `<input class="pathbar pathedit mono" id="pathedit-${i}" value="${esc(t.path)}" aria-label="Pfad">` : `<div class="pathbar ${t.pathOpen ? 'open' : ''}" data-pathbar data-tip="Klick in die Zeile, um den Pfad zu tippen (Strg+L)">
        ${parts.length > 1 ? '<button class="pmore" data-pathmore aria-label="Ganzen Pfad zeigen">…</button>' : ''}<button class="pseg" data-go="/">Server</button>${parts.map((part, k) => `${psep('/' + parts.slice(0, k).join('/'))}<button class="pseg ${k === parts.length - 1 ? 'last' : ''}" data-go="${esc('/' + parts.slice(0, k + 1).join('/'))}">${esc(part.replace('::', ''))}</button>`).join('')}${FS.kids(FS.nodeAt(t.path)).some(c => c.dir) ? psep(t.path) : ''}</div>`}
      <span class="tools">
        ${t.qf ? `<div class="qfilter">${g('IconSearch')}<input id="qf-${i}" value="${esc(t.filter)}" placeholder="Suchen, etwa *.mkv" aria-label="Im Ordner suchen" spellcheck="false" autocomplete="off">${t.filter ? `<button class="btn icon" data-qfsave data-tip="Diese Suche speichern">${g('IconSave')}</button>` : ''}<button class="qdeep ${t.deep === false ? '' : 'on'}" aria-pressed="${t.deep !== false}" aria-label="Unterordner" data-qfdeep data-tip="${t.deep === false ? 'Nur dieser Ordner. Klick sucht auch in Unterordnern' : 'Mit Unterordnern. Klick sucht nur hier'}">${g('IconFolderOpen')}</button><button class="btn icon" data-qfclose data-tip="Suche schließen (Esc)">${g('IconClose')}</button></div>` : `<button class="btn icon" data-run="quickFilter" data-tip="In diesem Ordner und darunter suchen (Strg+F oder /)">${g('IconSearch')}</button>`}
        <button class="btn icon" data-labelsmenu data-tip="Markierungen: nach Farbe oder Schlagwort suchen">${g('IconHash')}</button>
        <button class="btn icon" data-viewmenu data-tip="Ansicht: ${VIEWS.find(v => v[0] === viewOf(t))[1]}">${g(VIEWS.find(v => v[0] === viewOf(t))[2])}</button>
        ${CC.prefs.dual ? `<button class="btn icon" data-run="equalPanels" data-tip="Anderes Fenster auf diesen Ordner (Strg+.)">${g('IconSync')}</button>` : ''}
      </span>
    </div>
    <div class="list view-${viewOf(t)} ${CARD_VIEWS.includes(viewOf(t)) ? 'grid-view' : ''}" tabindex="0" style="--cols:${Columns.template()};--minw:${Columns.minWidth()}">
      ${!['details', 'compact'].includes(viewOf(t)) ? '' : `<div class="lhead" data-lhead>${Columns.visible().map(id => { const c = COL[id], key = Columns.sortKey(c); return `<button class="lh ${c.r ? 'r' : ''} ${t.sort === key ? 'sorted' : ''}" data-sort="${key}" data-colid="${id}">${esc(c.label)}${sortGlyph(key)}${c.fixed ? '' : `<span class="lh-grip" data-grip="${id}" aria-hidden="true"></span>`}</button>`; }).join('')}</div>`}
      ${viewOf(t) === 'columns' ? millerHtml(i, t, list) : list.map((k, idx) => groupHead(t, list, idx) + ITEM_HTML[viewOf(t)](i, k, idx)).join('')}
      ${!list.length ? `<div class="empty">${g(t.filter ? 'IconSearch' : 'IconFolder')}<div>${t.filter ? `Nichts gefunden für ${esc(t.filter)}` : 'Dieser Ordner ist leer'}</div>${t.filter ? `<div class="muted" style="font-size:12px">${t.deep === false ? 'Nur in diesem Ordner gesucht' : 'Auch in allen Unterordnern gesucht'}</div>` : '<div class="muted" style="font-size:12px">Dateien hierher ziehen zum Hochladen</div>'}</div>` : ''}
    </div>
    <div class="lfoot glim-num">${footer(i, list)}</div>
    `;
  return el;
}

// Every view renders the same .row elements, so clicks, marks, drags and menus work the same in all of them.
const VIEWS = [
  ['details', 'Details', 'IconViewAdvanced'], ['compact', 'Kompakt', 'IconViewSimple'], ['list', 'Liste', 'IconLog'], ['content', 'Inhalt', 'IconFileDoc'],
  ['columns', 'Spalten', 'IconCompare'], ['tiles', 'Kacheln', 'IconTabApps'], ['large', 'Große Kacheln', 'IconFileImage'], ['gallery', 'Galerie', 'IconFileRaw'],
  ['drives', 'Laufwerke', 'IconDatabase'],
];
const CARD_VIEWS = ['tiles', 'large', 'gallery', 'drives'];
// Each folder remembers the view it was last shown in, so photos stay a gallery and logs stay a list.
// A folder without its own choice opens where the volumes live as drives, and below a column view as columns, as in the Finder.
const viewOf = (t) => {
  const own = CC.prefs.folderViews?.[t.path];
  if (own) return own;
  if (FS.kids(FS.nodeAt(t.path)).some(k => FS.FREE[FS.join(t.path, k.name)])) return 'drives';
  for (let p = t.path; p !== '/'; p = FS.parent(p)) if (CC.prefs.folderViews?.[FS.parent(p)] === 'columns') return 'columns';
  return 'details';
};
const setView = (t, id) => CC.setPref('folderViews', { ...CC.prefs.folderViews, [t.path]: id });

// Grouping splits the list into sections with a heading; it is kept per folder like the view.
const AGE = [['Heute', 1], ['Gestern', 2], ['Diese Woche', 7], ['Diesen Monat', 31], ['Dieses Jahr', 365], ['Älter', Infinity]];
const SIZE = [['Riesig, über 1 GB', 1e9], ['Groß, über 100 MB', 1e8], ['Mittel, über 1 MB', 1e6], ['Klein', 1], ['Leer', 0]];
const KIND_ORDER = ['dir', 'video', 'audio', 'img', 'pdf', 'office', 'text', 'code', 'arch', 'disk', 'file'];
const GROUPS = {
  kind: { label: 'Art', g: 'IconFile', rank: (k) => KIND_ORDER.indexOf(k.dir ? 'dir' : k.kind), name: (k) => k.dir ? 'Ordner' : KIND_NAMES[k.kind] || 'Sonstiges' },
  date: { label: 'Änderungsdatum', g: 'IconCalendar', rank: (k) => AGE.findIndex(([, d]) => NOW - k.time < d * DAY), name: (k) => AGE.find(([, d]) => NOW - k.time < d * DAY)[0] },
  size: { label: 'Größe', g: 'IconDatabase', rank: (k) => k.dir ? -1 : SIZE.findIndex(([, s]) => (k.size || 0) >= s), name: (k) => k.dir ? 'Ordner' : SIZE.find(([, s]) => (k.size || 0) >= s)[0] },
};
const groupOf = (t) => CC.prefs.folderGroups?.[t.path] || '';
const setGroup = (t, id) => CC.setPref('folderGroups', { ...CC.prefs.folderGroups, [t.path]: id });
function groupHead(t, list, idx) {
  const grp = GROUPS[groupOf(t)];
  if (!grp) return '';
  const name = grp.name(list[idx]);
  if (idx > 0 && grp.name(list[idx - 1]) === name) return '';
  const n = list.filter(k => grp.name(k) === name).length;
  return `<div class="lgroup"><span>${esc(name)}</span><span class="glim-num">${n}</span></div>`;
}

const rowClass = (t, k, idx) => {
  const cut = Files.clip?.cut && Files.clip.from === t.path && Files.clip.items.includes(k.name);
  return `${k.dir ? 'dir' : ''} ${k.hidden || k.name.startsWith('.') ? 'hidden' : ''} ${t.marked.has(k.name) ? 'marked' : ''} ${idx === t.cursor ? 'cursor' : ''} ${cut ? 'cut' : ''}`;
};
const check = () => `<span class="cbox on">${g('IconCheck')}</span>`;

function nameCell(i, k) {
  const t = Files.T(i), mark = Files.marks[i][k.name];
  const shown = CC.prefs.showExt || k.dir || !k.name.includes('.', 1) ? k.name : k.name.slice(0, k.name.lastIndexOf('.'));
  return `<span class="name"><span class="glyph">${t.marked.has(k.name) ? check() : g(FS.glyphOf(k))}</span><span class="t">${esc(shown)}</span>${Labels.marks(k, t.path)}${k.at && k.at !== t.path ? `<span class="link">in ${esc(k.at.slice(t.path.length + 1))}</span>` : ''}${k.link ? `<span class="link">→ ${esc(k.link)}</span>` : ''}${k.zip ? '<span class="pill state">Archiv</span>' : ''}${k.remote ? `<span class="pill state">${esc(k.remote.split(':')[0])}</span>` : ''}${mark ? `<span class="pill ${mark.tone} state">${esc(mark.label)}</span>` : ''}</span>`;
}

function rowHtml(i, k, idx) {
  const t = Files.T(i);
  const vol = k.dir && FS.FREE[FS.join(t.path, k.name)] && FS.join(t.path, k.name);
  const cells = Columns.visible().map(id => id === 'name' ? nameCell(i, k)
    : id === 'size' && vol ? `<span class="cell r glim-num vcell">${volMeter(vol)}${fmtSize(FS.FREE[vol])} frei</span>`
    : `<span class="cell ${COL[id].r ? 'r' : ''} ${COL[id].mono ? 'mono' : ''} glim-num">${esc(COL[id].value(k, t.path))}</span>`).join('');
  return `<div class="row ${rowClass(t, k, idx)}" data-idx="${idx}" draggable="true">${cells}</div>`;
}

const flowHtml = (i, k, idx) => `<div class="row fitem ${rowClass(Files.T(i), k, idx)}" data-idx="${idx}" draggable="true">${nameCell(i, k)}</div>`;

// The cards show what a file looks like: the viewer's own pictures for photos and films, the first lines of
// text files, and a sketched page for documents, sheets and slides.
const FORMAT_ID = Object.fromEntries(FORMATS.flatMap(f => f.ext.map(e => [e, f.id])));
const TEXT_LIKE = ['text', 'code', 'config', 'log', 'subtitle', 'playlist', 'checksum', 'key', 'calendar', 'mail', 'weblink'];
const fill = (svg) => svg.replace('<svg ', '<svg class="scene" preserveAspectRatio="xMidYMid slice" ');
const lines = (n, short = []) => Array.from({ length: n }, (_, i) => `<i class="${short.includes(i) ? 's' : ''}"></i>`).join('');

function thumb(k, folder) {
  const f = FORMAT_ID[FS.ext(k.name)] || '', hue = hashOf(k.name) % 360, stem = k.name.replace(/\.[^.]+$/, '').replace(/[-_.]+/g, ' ');
  if (k.dir) return `<span class="thumb">${g(FS.glyphOf(k))}</span>`;
  if (f === 'image' || f === 'raw') return `<span class="thumb pic">${fill(CC.ck.picture(k, folder))}</span>`;
  if (f === 'video') return `<span class="thumb pic">${fill(CC.ck.poster(k.name, 640, 360, false))}<span class="play">${g('IconPlay')}</span><span class="badge glim-num">${esc(COL.duration.value(k))}</span></span>`;
  if (f === 'audio') return `<span class="thumb cover" style="--h:${hue}">${g('IconFileAudio')}<span class="wave">${Array.from({ length: 18 }, (_, i) => `<i style="height:${20 + (hashOf(k.name + i) % 80)}%"></i>`).join('')}</span></span>`;
  if (TEXT_LIKE.includes(f)) return `<span class="thumb paper"><span class="page code">${esc(CC.ck.textFor(k).split('\n').slice(0, 16).join('\n'))}</span></span>`;
  if (f === 'pdf' || f === 'doc') return `<span class="thumb paper"><span class="page"><b>${esc(stem)}</b>${lines(9, [3, 8])}</span><span class="tag ${f}">${f === 'pdf' ? 'PDF' : FS.ext(k.name).toUpperCase()}</span></span>`;
  if (f === 'sheet') return `<span class="thumb paper"><span class="page grid"></span><span class="tag sheet">${FS.ext(k.name).toUpperCase()}</span></span>`;
  if (f === 'slides') return `<span class="thumb paper"><span class="slide"><b>${esc(stem)}</b><span class="bars"><i style="height:45%"></i><i style="height:80%"></i><i style="height:60%"></i></span></span><span class="tag slides">${FS.ext(k.name).toUpperCase()}</span></span>`;
  if (f === 'ebook') return `<span class="thumb paper"><span class="book" style="--h:${hue}"><b>${esc(stem)}</b></span></span>`;
  if (f === 'font') return `<span class="thumb font"><span>Aa</span><small>${esc(stem)}</small></span>`;
  return `<span class="thumb">${g(FS.glyphOf(k))}</span>`;
}

function tileHtml(i, k, idx) {
  const t = Files.T(i);
  const meta = k.dir ? `${FS.kids(k).length} Obj.` : fmtSize(k.size);
  return `<div class="row gcard ${rowClass(t, k, idx)}" data-idx="${idx}" draggable="true" data-tip="${esc(k.name)}">${thumb(k, k.at || t.path)}${t.marked.has(k.name) ? check() : ''}<span class="t">${esc(k.name)}</span><span class="meta glim-num">${meta}${Labels.marks(k, t.path)}</span></div>`;
}

// A drive card as in the Explorer's This PC: the volume's glyph, its name, how full it is and what is left.
function driveHtml(i, k, idx) {
  const t = Files.T(i), p = FS.join(t.path, k.name), free = FS.FREE[p], total = FS.TOTAL[p];
  const glyph = p === '/boot' ? 'IconFlash' : p === '/remotes' ? 'IconCloud' : p === '/mnt/user' ? 'IconFiles' : free ? 'IconDatabase' : FS.glyphOf(k);
  const used = free && total ? 1 - free / total : 0;
  return `<div class="row dcard ${rowClass(t, k, idx)}" data-idx="${idx}" draggable="true">${t.marked.has(k.name) ? check() : ''}<span class="dglyph">${g(glyph)}</span>
    <span class="dtext"><span class="t">${esc(FS.LABEL[p] ? `${FS.LABEL[p]} (${k.name})` : k.name)}</span>
    ${free && total ? `<span class="dbar ${used > .9 ? 'full' : ''}"><i style="width:${Math.round(used * 100)}%"></i></span><span class="meta glim-num">${fmtSize(free)} frei von ${fmtSize(total)}</span>` : `<span class="meta glim-num">${k.dir ? `${FS.kids(k).length} Obj.` : fmtSize(k.size)}</span>`}</span></div>`;
}

function contentHtml(i, k, idx) {
  const t = Files.T(i);
  const extra = ['dimensions', 'duration', 'pages'].map(id => COL[id].value(k, t.path)).find(Boolean);
  return `<div class="row citem ${rowClass(t, k, idx)}" data-idx="${idx}" draggable="true">${thumb(k, k.at || t.path)}${t.marked.has(k.name) ? check() : ''}
    <span class="c-main"><span class="t">${esc(k.name)}</span><span class="c-sub">${esc(COL.type.value(k))} · ${esc(fmtListDate(k.time))}</span></span>
    <span class="c-side glim-num"><span>${k.dir ? `${FS.kids(k).length} Obj.` : fmtSize(k.size)}</span>${extra ? `<span>${esc(extra)}</span>` : ''}</span></div>`;
}

// Column view: the folders above the current one stay visible as columns to the left, and a folder under the
// cursor already shows its contents on the right, so the arrows walk the tree sideways.
function millerHtml(i, t, list) {
  const parts = FS.split(t.path).filter(p => !p.includes('::'));
  const column = (folder, on) => {
    const kids = FS.kids(FS.nodeAt(folder)).filter(k => CC.prefs.showHidden || !k.name.startsWith('.')).sort((a, b) => (b.dir ? 1 : 0) - (a.dir ? 1 : 0) || a.name.localeCompare(b.name, 'de', { numeric: true }));
    return `<div class="mcol">${kids.map(k => `<div class="mitem ${k.name === on ? 'on' : ''}" ${k.dir ? `data-go="${esc(FS.join(folder, k.name))}"` : ''}>${g(FS.glyphOf(k))}<span>${esc(FS.LABEL[FS.join(folder, k.name)] || k.name)}</span>${k.dir ? g('IconExpand') : ''}</div>`).join('')}</div>`;
  };
  const ancestors = parts.map((p, j) => [FS.join('/', parts.slice(0, j).join('/')), p]).slice(-2);
  const cur = list[t.cursor];
  const peek = cur?.dir ? column(FS.join(cur.at || t.path, cur.name), null) : '';
  return ancestors.map(([folder, on]) => column(folder, on)).join('')
    + `<div class="mcol cur">${list.map((k, idx) => flowHtml(i, k, idx)).join('')}</div>${peek}`;
}

const ITEM_HTML = { details: rowHtml, compact: rowHtml, list: flowHtml, columns: flowHtml, content: contentHtml, tiles: tileHtml, large: tileHtml, gallery: tileHtml, drives: driveHtml };

// A separator in the path opens the folders below the part on its left, as Explorer's arrows do.
const psep = (under) => FS.nodeAt(under)?.dir ? `<button class="psep" data-psub="${esc(under)}" aria-label="Ordner in ${esc(FS.split(under).pop() || 'Server')}">›</button>` : '<span class="psep" aria-hidden="true">›</span>';

function footer(i, list) {
  const t = Files.T(i);
  const items = list.filter(k => !k.up);
  const marked = items.filter(k => t.marked.has(k.name));
  const vol = FS.volumeOf(t.path);
  const used = 1 - FS.FREE[vol] / FS.TOTAL[vol];
  const sel = marked.length ? `${marked.length} markiert, ${fmtSize(marked.reduce((a, k) => a + FS.totalSize(k), 0))}<button class="lclear" data-run="unselect" data-tip="Auswahl aufheben (Esc)" aria-label="Auswahl aufheben">${g('IconClose')}</button>` : `${items.length} ${t.filter || t.smart ? 'Treffer' : t.flat ? 'Dateien aus allen Unterordnern' : items.length === 1 ? 'Objekt' : 'Objekte'}`;
  const dots = CC.prefs.dual ? `<span class="pdots" aria-hidden="true">${[0, 1].map(n => `<i class="${n === i ? 'on' : ''}" data-pswitch="${n}"></i>`).join('')}</span>` : '';
  if (FS.inArchive(t.path)) return `<span>${sel}</span>${dots}<span class="grow"></span><span>im Archiv, nur lesen</span>`;
  return `<span>${sel}</span>${dots}<span class="grow"></span><button class="lfree" data-du="${esc(vol)}" data-tip="Speicherbelegung von ${esc(FS.LABEL[vol] || vol)}"><span>${fmtSize(FS.FREE[vol])} frei</span><span class="meter"><i style="width:${Math.round(used * 100)}%"></i></span></button>`;
}

// With the preview bar on, the other panel shows what the cursor stands on, as Krusader's quick view does.
function renderQuickview(i) {
  const el = document.createElement('section');
  el.className = 'panel glim-card';
  el.dataset.panel = i;
  const k = Files.cursorEntry();
  el.innerHTML = `<span class="notch">Vorschau</span><div class="quickview" id="quickview"></div>`;
  const host = $('#quickview', el);
  if (!k || k.up) host.innerHTML = `<div class="empty">${g('IconEye')}<div>Wähle links eine Datei</div></div>`;
  else if (CC.renderPreview) CC.renderPreview(k, host, Files.T().path);
  else host.innerHTML = `<div class="well"><div class="frow"><span class="lab">${esc(k.name)}</span><span class="pill">${fmtSize(FS.totalSize(k))}</span></div></div>`;
  return el;
}

function renderFkeys() {
  // The bar follows the key map, so keys of your own show up here too.
  const keys = ['F1', 'F2', 'F3', 'F4', 'F5', 'F6', 'F7', 'F8', 'F9', 'F10'].map(k => [k, k === 'F10' ? 'menuBar' : CC.keymap[k], CC.keymap[k] === 'shortcuts' ? 'Hilfe' : '']);
  $('#fkeys').innerHTML = keys.map(([k, id, label]) => {
    const a = CC.actions[id];
    if (!a) return '';
    return `<button class="btn" data-run="${id}"><span class="kbd">${k}</span>${g(a.g)}${esc(label || (typeof a.label === 'function' ? a.label() : a.label))}</button>`;
  }).join('');
}

CC.registerView('files', {
  render: renderFiles,
  key(e, combo) {
    const t = Files.T(), list = Files.entries();
    const move = (to) => { t.cursor = Math.max(0, Math.min(list.length - 1, to)); CC.render(); $('.panel.active .row.cursor')?.scrollIntoView({ block: 'nearest' }); };
    // Cards wrap into rows and the list view into columns, so one arrow pair steps by a whole row or column there.
    const grid = CARD_VIEWS.includes(viewOf(t)), flow = viewOf(t) === 'list';
    const across = () => {
      const rows = $$('.panel.active .list > .row');
      return Math.max(1, rows.filter(r => flow ? r.offsetLeft === rows[0].offsetLeft : r.offsetTop === rows[0].offsetTop).length);
    };
    const markStep = () => { const k = list[t.cursor]; if (k && !k.up) t.marked.has(k.name) ? t.marked.delete(k.name) : t.marked.add(k.name); move(t.cursor + 1); };
    const keys = {
      Tab: () => { if (CC.prefs.dual && !CC.prefs.quickview) { Files.active = Files.other(); CC.render(); } },
      ArrowDown: () => move(t.cursor + (grid ? across() : 1)), ArrowUp: () => move(t.cursor - (grid ? across() : 1)), Home: () => move(0), End: () => move(list.length - 1),
      ...(grid || flow ? { ArrowRight: () => move(t.cursor + (flow ? across() : 1)), ArrowLeft: () => move(t.cursor - (flow ? across() : 1)) } : {}),
      ...(viewOf(t) === 'columns' ? { ArrowLeft: () => t.path !== '/' && Files.open(Files.active, { up: true }), ArrowRight: () => list[t.cursor]?.dir && Files.open(Files.active, list[t.cursor]) } : {}),
      PageDown: () => move(t.cursor + 10), PageUp: () => move(t.cursor - 10),
      Enter: () => list[t.cursor] && Files.open(Files.active, list[t.cursor]),
      Backspace: () => t.path !== '/' && Files.open(Files.active, { up: true }),
      Insert: markStep, ' ': () => CC.run('quickLook'),
      Escape: () => { if (t.qf || t.filter) closeFilter(); else if (Object.keys(Files.marks[0]).length || Object.keys(Files.marks[1]).length) { Files.marks = [{}, {}]; CC.render(); } else if (t.marked.size) { t.marked = new Set(); CC.render(); } },
    };
    if (keys[combo]) { e.preventDefault(); keys[combo](); }
  },
});

// Clicks, drags and the context menu inside the file view.
let lastClick = {};
document.addEventListener('click', e => {
  if (CC.view !== 'files') return;
  const el = e.target;
  const panelEl = el.closest('.panel');
  if (panelEl && +panelEl.dataset.panel !== Files.active && !CC.prefs.quickview) {
    Files.active = +panelEl.dataset.panel;
    if (!el.closest('[data-tab],[data-go],[data-sort],[data-newtab],[data-run],.row,[data-close]')) CC.render();
  }
  const tog = el.closest('[data-tog]');
  if (tog) { const p = tog.dataset.tog; Files.openTree.has(p) ? Files.openTree.delete(p) : Files.openTree.add(p); renderSide(); return; }
  const close = el.closest('[data-close]');
  if (close) { const p = Files.P(); p.tabs.splice(+close.dataset.close, 1); p.cur = Math.min(p.cur, p.tabs.length - 1); CC.render(); return; }
  const tab = el.closest('[data-tab]');
  if (tab) { Files.P().cur = +tab.dataset.tab; CC.render(); return; }
  if (el.closest('[data-newtab]')) return CC.run('newTab');
  const psub = el.closest('[data-psub]');
  if (psub) {
    const under = psub.dataset.psub, next = Files.T().path, r = psub.getBoundingClientRect();
    const dirs = FS.kids(FS.nodeAt(under)).filter(k => k.dir && (CC.prefs.showHidden || !k.name.startsWith('.'))).sort((a, b) => a.name.localeCompare(b.name, 'de', { numeric: true }));
    return menu(r.left, r.bottom + 4, dirs.map(k => {
      const p = FS.join(under, k.name), onPath = next === p || next.startsWith(p + '/');
      return { label: FS.LABEL[p] || k.name, g: onPath ? 'IconFolderOpen' : 'IconFolder', run: () => Files.go(p) };
    }));
  }
  const vm = el.closest('[data-viewmenu]');
  if (vm) { const r = vm.getBoundingClientRect(); return menu(r.right - 230, r.bottom + 6, VIEWS.map(v => 'view-' + v[0])); }
  if (el.closest('[data-pathmore]')) { Files.T().pathOpen = !Files.T().pathOpen; CC.render(); $$('.pathbar.open').forEach(p => { p.scrollLeft = p.scrollWidth; }); return; }
  const sw = el.closest('[data-pswitch]');
  if (sw) { Files.active = +sw.dataset.pswitch; return CC.render(); }
  const du = el.closest('[data-du]');
  if (du) return CC.run('diskUsage', du.dataset.du);
  const goEl = el.closest('[data-go]');
  if (goEl && !el.closest('[data-tog]')) return Files.go(goEl.dataset.go);
  if (el.closest('[data-pathbar]') && !el.closest('.pseg')) return CC.run('gotoPath');
  const sort = el.closest('[data-sort]');
  if (el.closest('[data-grip]')) return;
  if (sort) {
    if (colDrag.moved) { colDrag.moved = false; return; }
    const t = Files.T();
    if (t.sort === sort.dataset.sort) t.dir *= -1; else { t.sort = sort.dataset.sort; t.dir = 1; }
    CC.render();
    return;
  }
  if (el.closest('[data-qfclose]')) return closeFilter();
  if (el.closest('[data-qfdeep]')) { const t = Files.T(); t.deep = t.deep === false; t.cursor = 0; CC.render(); return bindFilter($(`#qf-${Files.active}`), t.filter.length); }
  const row = el.closest('.row');
  if (row && panelEl) {
    const t = Files.T(), idx = +row.dataset.idx, list = Files.entries();
    if (e.shiftKey) { const [a, b] = [Math.min(t.cursor, idx), Math.max(t.cursor, idx)]; for (let k = a; k <= b; k++) if (!list[k].up) t.marked.add(list[k].name); }
    else if (e.ctrlKey || e.metaKey) { if (!list[idx].up) t.marked.has(list[idx].name) ? t.marked.delete(list[idx].name) : t.marked.add(list[idx].name); }
    // A click re-renders the row, so the browser cannot pair two clicks into a dblclick event; pair them here.
    const again = lastClick.idx === idx && lastClick.panel === Files.active && e.timeStamp - lastClick.at < 450 && !e.shiftKey && !e.ctrlKey;
    lastClick = { idx, panel: Files.active, at: e.timeStamp };
    t.cursor = idx;
    CC.render();
    if (again) { lastClick = {}; Files.open(Files.active, list[idx]); }
  } else if (panelEl && el.closest('.list') && !el.closest('.lhead') && Files.T().marked.size) {
    // A click on empty space in the list drops the selection, as in Explorer.
    Files.T().marked = new Set();
    CC.render();
  }
});

// The grip on a header's right edge sets that column's width; widths are kept in rem like the defaults.
const REM = () => parseFloat(getComputedStyle(document.documentElement).fontSize);
const relayout = () => $$('.list').forEach(l => { l.style.setProperty('--cols', Columns.template()); l.style.setProperty('--minw', Columns.minWidth()); });
const colSize = { id: null, x: 0, w: 0, moved: false };
document.addEventListener('pointerdown', e => {
  const grip = e.target.closest?.('[data-grip]');
  if (!grip || e.button !== 0) return;
  e.stopImmediatePropagation();
  e.preventDefault();
  Object.assign(colSize, { id: grip.dataset.grip, x: e.clientX, w: grip.closest('.lh').getBoundingClientRect().width / REM() - .7, moved: false });
  document.documentElement.classList.add('col-sizing');
}, true);
document.addEventListener('pointermove', e => {
  if (!colSize.id || (!colSize.moved && Math.abs(e.clientX - colSize.x) < 3)) return;
  colSize.moved = true;
  CC.prefs.colWidths = { ...CC.prefs.colWidths, [colSize.id]: Math.max(2.5, Math.round((colSize.w + (e.clientX - colSize.x) / REM()) * 4) / 4) };
  relayout();
});
// Only a real drag re-renders; a plain press must leave the grip in place so a double click can land on it.
document.addEventListener('pointerup', () => {
  if (!colSize.id) return;
  colSize.id = null;
  document.documentElement.classList.remove('col-sizing');
  if (colSize.moved) CC.setPref('colWidths', CC.prefs.colWidths);
}, true);
document.addEventListener('dblclick', e => {
  const grip = e.target.closest('[data-grip]');
  if (!grip) return;
  const id = grip.dataset.grip, n = Columns.visible().indexOf(id) + 1;
  const cells = $$(`.list .row > :nth-child(${n})`);
  const label = grip.closest('.lh');
  const widest = Math.max(label.scrollWidth, ...cells.map(c => c.scrollWidth));
  CC.setPref('colWidths', { ...CC.prefs.colWidths, [id]: Math.max(2.5, Math.ceil((widest / REM() + .3) * 4) / 4) });
});

// A column header moves live under the pointer; a press without movement stays a click that sorts.
const colDrag = { id: null, panel: 0, x: 0, moved: false };
document.addEventListener('pointerdown', e => {
  const h = e.target.closest?.('.lh');
  if (!h || e.button !== 0) return;
  Object.assign(colDrag, { id: h.dataset.colid, panel: h.closest('.panel').dataset.panel, x: e.clientX, moved: false });
});
document.addEventListener('pointermove', e => {
  if (!colDrag.id) return;
  if (!colDrag.moved && Math.abs(e.clientX - colDrag.x) < 6) return;
  colDrag.moved = true;
  const heads = $$(`.panel[data-panel="${colDrag.panel}"] .lh`);
  const over = heads.find(h => { const r = h.getBoundingClientRect(); return e.clientX >= r.left && e.clientX <= r.right; });
  const order = Columns.shown();
  if (over && over.dataset.colid !== colDrag.id) {
    const from = order.indexOf(colDrag.id), to = order.indexOf(over.dataset.colid);
    order.splice(to, 0, order.splice(from, 1)[0]);
    CC.prefs.columns = order;
    CC.render();
  }
  $$(`.lh[data-colid="${colDrag.id}"]`).forEach(h => h.classList.add('moving'));
});
document.addEventListener('pointerup', () => {
  if (!colDrag.id) return;
  const moved = colDrag.moved;
  colDrag.id = null;
  if (moved) CC.setPref('columns', Columns.shown());
});

document.addEventListener('contextmenu', e => {
  if (CC.view !== 'files') return;
  if (e.target.closest('.lhead')) { e.preventDefault(); Columns.menu(e.clientX, e.clientY); return; }
  const row = e.target.closest('.row'), list = e.target.closest('.list');
  if (!row && !list) return;
  e.preventDefault();
  Files.active = +e.target.closest('.panel').dataset.panel;
  if (row) {
    const t = Files.T(), idx = +row.dataset.idx, k = Files.entries()[idx];
    if (!t.marked.has(k.name)) { t.marked = new Set(); t.cursor = idx; }
    CC.render();
    const sel = Files.sel(), file = sel.find(x => !x.dir);
    const oneFolder = sel.length === 1 && sel[0].dir && !sel[0].up && !FS.inArchive(t.path);
    menu(e.clientX, e.clientY, [
      ...(oneFolder ? folderItems(FS.join(t.path, sel[0].name)) : ['view', 'edit', ...(file?.zip || sel[0]?.kind === 'arch' ? ['unpack', 'testArchive'] : [])]), '-',
      'copy', 'move', 'rename', 'clipCut', 'clipCopy', '-',
      'share', 'collect', 'download', 'pack', 'checksum', 'symlink', ...(sel.length === 2 && sel.every(x => !x.dir) ? ['diff'] : []), '-',
      ...Labels.menuItems(sel, t.path), '-', 'props', 'fixPerms', 'copyPath', '-', 'delete',
    ]);
  } else {
    CC.render();
    menu(e.clientX, e.clientY, ['newFolder', 'newFile', 'upload', 'clipPaste', '-', 'selectAll', 'selectPattern', '-', 'refresh', 'props', 'fixPerms']);
  }
});

// Dragging rows copies onto a folder, a tab, a piece of the path, the sidebar or the other panel; Shift moves.
// Holding still over a folder, tab or path piece opens it, so a drop can go deeper, as in Finder.
let drag = null, spring = { el: null, timer: 0 };
const dropTarget = (e) => {
  const at = e.target.closest?.bind(e.target);
  if (!at) return null;
  const panelEl = at('.panel'), panel = panelEl ? +panelEl.dataset.panel : Files.active;
  const row = at('.row.dir');
  if (row && panelEl) {
    const k = Files.entries(panel)[+row.dataset.idx];
    if (k && !drag.items.some(x => x.name === k.name && (x.at || '') === (k.at || ''))) {
      const dest = FS.join(k.at || Files.T(panel).path, k.name);
      return { el: row, dest, open: () => Files.go(dest, panel) };
    }
  }
  const tab = at('[data-tab]');
  if (tab && panelEl) { const n = +tab.dataset.tab; return { el: tab, dest: Files.P(panel).tabs[n].path, open: () => { Files.P(panel).cur = n; CC.render(); } }; }
  const go = at('[data-go]');
  if (go) return { el: go, dest: go.dataset.go, open: () => Files.go(go.dataset.go, panelEl ? panel : Files.active) };
  const list = at('.list');
  if (list && panel !== drag.from) return { el: list, dest: Files.T(panel).path };
  return null;
};
const clearDrop = () => $$('.dragover, .drop').forEach(x => x.classList.remove('dragover', 'drop'));
document.addEventListener('dragstart', e => {
  const row = e.target.closest?.('.row');
  if (!row) return;
  const panelEl = row.closest('.panel');
  Files.active = +panelEl.dataset.panel;
  const t = Files.T(), k = Files.entries()[+row.dataset.idx];
  if (!t.marked.has(k.name)) { t.marked = new Set([k.name]); }
  drag = { from: Files.active, items: Files.sel() };
  e.dataTransfer.effectAllowed = 'copyMove';
  e.dataTransfer.setData('text/plain', drag.items.map(x => x.name).join('\n'));
});
document.addEventListener('dragover', e => {
  const external = !drag && [...(e.dataTransfer?.types || [])].includes('Files');
  if (external && e.target.closest?.('.list')) { e.preventDefault(); clearDrop(); e.target.closest('.list').classList.add('dragover'); return; }
  if (!drag) return;
  const tgt = dropTarget(e);
  if (!tgt) { clearDrop(); return; }
  e.preventDefault();
  if (!tgt.el.classList.contains('drop') && !tgt.el.classList.contains('dragover')) { clearDrop(); tgt.el.classList.add(tgt.el.classList.contains('list') ? 'dragover' : 'drop'); }
  if (spring.el !== tgt.el) {
    clearTimeout(spring.timer);
    spring = { el: tgt.el, timer: tgt.open ? setTimeout(() => { tgt.open(); spring.el = null; }, 900) : 0 };
  }
});
document.addEventListener('dragend', () => { drag = null; clearTimeout(spring.timer); spring.el = null; clearDrop(); });
document.addEventListener('drop', e => {
  clearDrop();
  clearTimeout(spring.timer);
  spring.el = null;
  const list = e.target.closest?.('.list');
  if (!drag && list && e.dataTransfer.files.length) {
    e.preventDefault();
    Files.active = +list.closest('.panel').dataset.panel;
    if (CC.uploadFiles) CC.uploadFiles([...e.dataTransfer.files], Files.T().path); else CC.toast('Hochladen ist noch nicht gebaut', 'warn');
    return;
  }
  if (!drag) return;
  e.preventDefault();
  const tgt = dropTarget(e), items = drag.items, from = Files.T(drag.from).path;
  drag = null;
  if (tgt) Files.transfer(e.shiftKey ? 'move' : 'copy', items, tgt.dest, from);
});

function openFilter() {
  if (CC.view !== 'files') CC.go('files');
  Files.T().qf = true;
  CC.render();
  bindFilter($(`#qf-${Files.active}`), Files.T().filter.length);
}
// Every keystroke re-renders the panel, so the field is rebuilt and has to be bound again.
function bindFilter(input, pos) {
  if (!input) return;
  input.focus();
  input.setSelectionRange(pos, pos);
  input.oninput = (e) => {
    Object.assign(Files.T(), { filter: e.target.value, cursor: 0 });
    const at = e.target.selectionStart;
    CC.render();
    bindFilter($(`#qf-${Files.active}`), at);
  };
  input.onkeydown = (e) => {
    if (e.key === 'Escape') { e.preventDefault(); closeFilter(); }
    if (e.key === 'Enter' || e.key === 'ArrowDown') { e.preventDefault(); $('.panel.active .list').focus(); }
  };
}
function closeFilter() { Object.assign(Files.T(), { qf: false, filter: '', cursor: 0 }); CC.render(); }

// Firefox keeps the mouse's side buttons for its own history and never shows them to the page (bug 1720538),
// so every folder change gets a history entry and stepping through the history steps through the folders.
const Nav = { at: 0, owner: [0], skip: false };
history.replaceState({ ccNav: 0 }, '');
const pushNav = (owner) => { Nav.at += 1; Nav.owner[Nav.at] = owner; Nav.owner.length = Nav.at + 1; history.pushState({ ccNav: Nav.at }, ''); };
Files.pushNav = pushNav;
// A window with its own folders takes part in the same history; closing it takes its steps back out.
CC.winNav = (w, steps) => {
  const base = Nav.at;
  steps.el = w;
  return {
    push: () => pushNav(steps),
    close: () => { if (Nav.at > base) { Nav.skip = true; history.go(base - Nav.at); } },
  };
};
addEventListener('popstate', e => {
  const to = e.state?.ccNav;
  if (to === undefined || to === Nav.at) return;
  const back = to < Nav.at, owner = Nav.owner[back ? Nav.at : to];
  Nav.at = to;
  if (Nav.skip) { Nav.skip = false; return; }
  if (owner && typeof owner === 'object') { if (winStack.at(-1)?.el === owner.el) (back ? owner.back : owner.forward)(); return; }
  if (CC.view !== 'files' || winStack.length) return;
  Files.active = owner ?? Files.active;
  CC.run(back ? 'back' : 'forward');
});

document.addEventListener('contextmenu', e => {
  const tab = CC.view === 'files' && e.target.closest('[data-tab]');
  if (!tab) return;
  e.preventDefault();
  Files.active = +tab.closest('.panel').dataset.panel;
  Files.P().cur = +tab.dataset.tab;
  CC.render();
  menu(e.clientX, e.clientY, ['newTab', 'dupTab', 'lockTab', '-', 'closeTab', 'closeOtherTabs']);
});
document.addEventListener('mousedown', e => { if (e.button === 1 && e.target.closest('[data-tab]')) e.preventDefault(); });
document.addEventListener('auxclick', e => {
  const tab = e.button === 1 && CC.view === 'files' && e.target.closest('[data-tab]');
  if (!tab) return;
  e.preventDefault();
  const p = Files.P(+tab.closest('.panel').dataset.panel);
  if (p.tabs.length < 2) return;
  p.tabs.splice(+tab.dataset.tab, 1);
  p.cur = Math.min(p.cur, p.tabs.length - 1);
  CC.render();
});
document.addEventListener('dblclick', e => {
  if (CC.view !== 'files' || !e.target.matches('.ptabs')) return;
  Files.active = +e.target.closest('.panel').dataset.panel;
  CC.run('newTab');
});

// On a phone only one panel fits; a sideways swipe over the list switches to the other one.
let swipe = null;
document.addEventListener('touchstart', e => { swipe = CC.view === 'files' && CC.prefs.dual && e.target.closest('.panel') ? { x: e.touches[0].clientX, y: e.touches[0].clientY } : null; }, { passive: true });
document.addEventListener('touchend', e => {
  if (!swipe || !matchMedia('(max-width: 1100px)').matches) return;
  const dx = e.changedTouches[0].clientX - swipe.x, dy = e.changedTouches[0].clientY - swipe.y;
  swipe = null;
  if (Math.abs(dx) < 60 || Math.abs(dy) > Math.abs(dx) / 2) return;
  Files.active = 1 - Files.active;
  CC.render();
});
