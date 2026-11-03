// Papierkorb: a full page listing what was deleted, with restore, delete forever and empty.
(() => {
  const { plural } = CC.ck;
  const KEEP = 30;
  const T = { sel: new Set(), sort: 'deleted', dir: -1, anchor: -1 };
  const COLS = [['name', 'Name'], ['from', 'Ursprung'], ['size', 'Größe'], ['deleted', 'Gelöscht']];
  const sizeOf = (e) => e.node ? FS.totalSize(e.node) : e.size;

  const sorted = () => {
    const by = {
      name: (a, b) => a.name.localeCompare(b.name, 'de', { numeric: true }),
      from: (a, b) => a.from.localeCompare(b.from, 'de'),
      size: (a, b) => sizeOf(a) - sizeOf(b),
      deleted: (a, b) => a.deleted - b.deleted,
    }[T.sort];
    return DATA.trash.slice().sort((a, b) => by(a, b) * T.dir);
  };
  const glyphOf = (e) => e.dir ? 'IconFolder' : FS.glyphOf({ name: e.name });

  function ensureDir(path) {
    let n = FS.root;
    for (const part of FS.split(path)) {
      let next = FS.kids(n).find(k => k.name === part && k.dir);
      if (!next) { next = { name: part, dir: true, children: [], time: Date.now(), owner: 'nobody', group: 'users', mode: '0775' }; n.children.push(next); }
      n = next;
    }
    return n;
  }
  const rebuild = (e) => e.dir
    ? { name: e.name, dir: true, time: e.deleted, owner: 'nobody', group: 'users', mode: '0775', children: [{ name: 'daten.bin', size: e.size, time: e.deleted - DAY, owner: 'nobody', group: 'users', mode: '0664' }] }
    : { name: e.name, size: e.size, time: e.deleted - 3 * DAY, owner: 'nobody', group: 'users', mode: '0664' };

  function restore(list) {
    if (!list.length) return;
    let renamed = 0;
    for (const e of list) {
      const dir = ensureDir(e.from), node = e.node || rebuild(e);
      delete node.kind;
      if (FS.kids(dir).some(k => k.name === node.name)) { node.name = Files.freeName(dir, node.name); renamed++; }
      dir.children.push(node);
    }
    DATA.trash = DATA.trash.filter(x => !list.includes(x));
    DATA.activity.unshift({ t: Date.now(), who: CC.user.name, what: 'wiederhergestellt', obj: list.length === 1 ? list[0].name : plural(list.length, 'Objekt', 'Objekte') });
    T.sel.clear();
    CC.render();
    const from = [...new Set(list.map(e => e.from))];
    CC.toast(`${list.length === 1 ? list[0].name : plural(list.length, 'Objekt', 'Objekte')} wiederhergestellt${from.length === 1 ? ` nach ${from[0]}` : ''}${renamed ? ', mit neuem Namen, weil der alte schon vergeben war' : ''}`, 'ok',
      from.length === 1 ? { label: 'Hingehen', run: () => Files.go(from[0]) } : null);
  }

  function purge(list, all) {
    if (!list.length) return;
    const size = list.reduce((a, e) => a + sizeOf(e), 0);
    win({
      title: all ? 'Papierkorb leeren' : 'Endgültig löschen',
      body: `<div class="well"><div class="eyebrow">${plural(list.length, 'Objekt', 'Objekte')}, ${fmtSize(size)}</div>
          <table class="tbl">${list.slice(0, 8).map(e => `<tr><td>${esc(e.name)}</td><td class="glim-num r">${fmtSize(sizeOf(e))}</td></tr>`).join('')}${list.length > 8 ? `<tr><td colspan="2">und ${list.length - 8} weitere</td></tr>` : ''}</table>
          <div class="ce-ask">${g('IconWarning')}<span>Danach ist es weg und lässt sich nicht wiederherstellen.</span></div></div>`,
      foot: [cancel, { label: all ? 'Papierkorb leeren' : 'Endgültig löschen', g: all ? 'IconEmpty' : 'IconTrash', primary: true, run: () => {
        DATA.trash = DATA.trash.filter(x => !list.includes(x));
        T.sel.clear();
        CC.render();
        CC.toast(`${plural(list.length, 'Objekt', 'Objekte')} endgültig gelöscht, ${fmtSize(size)} frei`);
      } }],
    });
  }

  function render(host) {
    const list = sorted(), picked = list.filter(e => T.sel.has(e));
    if (!list.length) {
      host.innerHTML = `<div class="page-inner ct">${CC.card('Papierkorb', `<div class="empty ct-empty">${g('IconTrash')}<div class="t">Der Papierkorb ist leer</div><div class="muted">Was du löschst, bleibt hier ${KEEP} Tage, bevor es endgültig weg ist.</div><button class="btn" data-go-view="files">${g('IconFiles')}Zu den Dateien</button></div>`)}</div>`;
      return;
    }
    const total = list.reduce((a, e) => a + sizeOf(e), 0);
    const next = DATA.trash.reduce((a, e) => (!a || e.deleted < a.deleted ? e : a), null);
    host.innerHTML = `<div class="page-inner ct">
      ${CC.card('Papierkorb', `
        ${frow('Inhalt', `<span class="pill glim-num">${plural(list.length, 'Objekt', 'Objekte')}, ${fmtSize(total)}</span>`)}
        ${frow('Als Nächstes endgültig weg', `<span class="pill">${esc(next.name)}</span><span class="pill glim-num">am ${fmtDate(next.deleted + KEEP * DAY)}</span>`, `Alles im Papierkorb wird ${KEEP} Tage nach dem Löschen endgültig entfernt. Die Frist stellst du unter Einstellungen, Dateien ein.`)}`)}
      <section class="card glim-card ct-card"><span class="notch">Gelöschte Objekte</span>
        <div class="ct-head" role="row"><span></span><span></span>${COLS.map(([k, l]) => `<button data-tsort="${k}" class="${T.sort === k ? 'sorted' : ''} ${k === 'size' || k === 'deleted' ? 'r' : ''} col-${k}">${l}${T.sort === k ? g(T.dir > 0 ? 'IconMoveUp' : 'IconMoveDown') : ''}</button>`).join('')}
          <button class="btn" id="ct-more">${g('IconMore')}Mehr</button></div>
        <div class="ct-list ${picked.length ? 'picking' : ''}" role="listbox" aria-multiselectable="true">${list.map((e, i) => `
          <div class="ct-row ${T.sel.has(e) ? 'sel' : ''}" data-i="${i}" role="option" aria-selected="${T.sel.has(e)}">
            <button class="ct-mark" data-mark aria-label="Auswählen">${g('IconCheck')}</button>
            <span class="tile">${g(glyphOf(e))}</span>
            <span class="t">${esc(e.name)}</span>
            <span class="pill mono col-from" data-tip="${esc(e.from)}">${esc(e.from)}</span>
            <span class="pill glim-num col-size">${fmtSize(sizeOf(e)) || '0 B'}</span>
            <span class="pill glim-num col-deleted" data-tip="Von ${esc(e.by)} gelöscht, endgültig weg am ${fmtDate(e.deleted + KEEP * DAY)}">${fmtDate(e.deleted)}, ${esc(e.by)}</span>
            <span class="ct-acts"><button class="btn icon" data-act="restore" data-tip="Wiederherstellen" aria-label="Wiederherstellen">${g('IconRestore')}</button><button class="btn icon" data-act="purge" data-tip="Endgültig löschen" aria-label="Endgültig löschen">${g('IconTrash')}</button></span>
          </div>`).join('')}</div>
      </section>
      ${picked.length ? `<div class="ct-fab"><button class="btn key primary" id="ct-restore">${g('IconRestore')}${picked.length === 1 ? 'Wiederherstellen' : `${picked.length} wiederherstellen`}</button></div>` : ''}
    </div>`;

    host.onclick = (ev) => {
      const s = ev.target.closest('[data-tsort]');
      if (s) { const k = s.dataset.tsort; if (T.sort === k) T.dir *= -1; else { T.sort = k; T.dir = k === 'deleted' || k === 'size' ? -1 : 1; } return CC.render(); }
      if (ev.target.closest('#ct-restore')) return restore(picked);
      if (ev.target.closest('#ct-more')) {
        const r = ev.target.closest('#ct-more').getBoundingClientRect();
        return setTimeout(() => menu(r.right - 260, r.bottom + 6, [
          ...(picked.length ? [{ label: `Auswahl wiederherstellen (${picked.length})`, g: 'IconRestore', run: () => restore(picked) }, { label: `Auswahl endgültig löschen (${picked.length})`, g: 'IconTrash', run: () => purge(picked) }, '-'] : []),
          { label: 'Alles auswählen', g: 'IconSelectAll', key: 'Strg+A', run: () => { list.forEach(e => T.sel.add(e)); CC.render(); } },
          { label: 'Papierkorb leeren', g: 'IconEmpty', run: () => purge(list, true) },
        ]), 0);
      }
      const row = ev.target.closest('.ct-row');
      if (!row) return;
      const i = +row.dataset.i, e = list[i];
      const act = ev.target.closest('[data-act]');
      if (act) return act.dataset.act === 'restore' ? restore([e]) : purge([e]);
      if (ev.shiftKey && T.anchor >= 0) { const [a, b] = [Math.min(T.anchor, i), Math.max(T.anchor, i)]; for (let k = a; k <= b; k++) T.sel.add(list[k]); }
      else if (ev.ctrlKey || ev.metaKey || ev.target.closest('[data-mark]') || T.sel.size) { T.sel.has(e) ? T.sel.delete(e) : T.sel.add(e); T.anchor = i; }
      else { T.sel = new Set([e]); T.anchor = i; }
      CC.render();
    };
  }

  CC.registerView('trash', {
    page: true,
    render,
    key(e, combo) {
      const list = sorted(), picked = list.filter(x => T.sel.has(x));
      const keys = {
        'Ctrl+a': () => { list.forEach(x => T.sel.add(x)); CC.render(); },
        Escape: () => { T.sel.clear(); CC.render(); },
        Delete: () => purge(picked),
        Enter: () => restore(picked),
      };
      if (keys[combo]) { e.preventDefault(); keys[combo](); }
    },
  });
  CC.action('openTrash', { label: 'Papierkorb', g: 'IconTrash', run: () => { T.sel.clear(); CC.go('trash'); } });

  CC.css(`
.ct { padding-bottom: 6rem; }
.ct-empty { padding: 2.5rem 1rem 2rem; }
.ct-empty .t { color: var(--carbon-text-sub); font-size: 15px; font-weight: 600; }
.ct-empty .muted { font-size: 13px; margin-bottom: .6rem; }
.ct-card { padding: 1.5rem .75rem .6rem; }
.ct-head, .ct-row { display: grid; grid-template-columns: 1.3rem 2.25rem minmax(0, 1fr) 13rem 6rem 9.5rem 4.4rem; align-items: center; gap: .6rem; padding: 0 .5rem; }
.ct-head { height: 2.2rem; font-size: 12px; color: var(--carbon-text-muted); }
.ct-head [data-tsort] { text-align: left; display: inline-flex; align-items: center; gap: .3rem; height: 1.6rem; padding: 0 .35rem; border-radius: var(--radius-control); color: inherit; justify-self: start; }
.ct-head [data-tsort].r { justify-self: end; }
.ct-head [data-tsort]:hover { background: var(--carbon-hover); color: var(--carbon-text); }
.ct-head [data-tsort].sorted { color: var(--carbon-text); }
.ct-head [data-tsort] svg { width: .7rem; height: .7rem; }
.ct-head .col-from { justify-self: start; }
.ct-head .btn { justify-self: end; height: 1.8rem; padding: 0 .7rem; font-size: 13px; }
.ct-head .btn svg { width: .8rem; height: .8rem; }
.ct-row { height: var(--row-h); border-radius: var(--radius-control); cursor: default; user-select: none; }
.ct-row:hover { background: var(--carbon-hover); }
.ct-row .t { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ct-row .col-from { justify-self: start; max-width: 100%; overflow: hidden; text-overflow: ellipsis; display: block; line-height: 1.35rem; font-size: 11.5px; }
.ct-row .tile svg { color: var(--carbon-text-sub); }
.ct-row:hover .tile, .ct-row.sel .tile { background: var(--accent); }
.ct-row:hover .tile svg, .ct-row.sel .tile svg { color: var(--accent-contrast); }
.ct-row.sel { background: var(--accent); color: var(--accent-contrast); }
.ct-row.sel .pill { background: color-mix(in srgb, var(--accent-contrast) 14%, transparent); color: var(--accent-contrast); }
.ct-mark { width: 1.15rem; height: 1.15rem; border-radius: var(--radius-pill); background: var(--carbon-surface3); display: grid; place-items: center; opacity: 0; }
.ct-mark svg { width: .65rem; height: .65rem; opacity: 0; }
.ct-row:hover .ct-mark, .ct-list.picking .ct-mark, .ct-mark:focus-visible { opacity: 1; }
.ct-row.sel .ct-mark { background: var(--accent-contrast); }
.ct-row.sel .ct-mark svg { opacity: 1; color: var(--accent); }
.ct-acts { display: flex; gap: .2rem; justify-content: flex-end; opacity: 0; }
.ct-row:hover .ct-acts, .ct-row:focus-within .ct-acts { opacity: 1; }
@media (hover: none) { .ct-acts { opacity: 1; } .ct-mark { opacity: 1; } }
.ct-acts .btn { width: 1.9rem; height: 1.9rem; background: transparent; }
.ct-acts .btn:hover { background: var(--carbon-surface2); }
.ct-row.sel .ct-acts .btn svg { color: var(--accent-contrast); }
.ct-fab { position: sticky; bottom: 1rem; align-self: flex-end; margin-top: -1.5rem; z-index: 2; }
.ct-fab .btn { box-shadow: var(--elevation); }
.ct .card-body .frow .pill + .pill { margin-left: .3rem; }
@media (max-width: 1000px) { .ct-head, .ct-row { grid-template-columns: 1.3rem 2.25rem minmax(0, 1fr) 6rem 9.5rem 4.4rem; } .col-from { display: none !important; } }
@media (max-width: 760px) {
  .ct-head, .ct-row { grid-template-columns: 1.3rem 2.25rem minmax(0, 1fr) auto 4.2rem; gap: .45rem; padding: 0 .25rem; }
  .ct-head .col-deleted, .ct-row .col-deleted { display: none; }
  .ct-card { padding: 1.5rem .4rem .5rem; }
  .ct .card .frow { flex-wrap: wrap; }
}
`);
})();
