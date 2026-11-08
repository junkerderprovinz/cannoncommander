// Kontextmenü anpassen: the settings tile that hides rows of the file view's own menus and moves them within their
// group. The computer's own menu only exists in the desktop app, so its card is not drawn here.
(() => {
  const nameOf = (a) => typeof a.label === 'function' ? a.label() : a.label;
  const hinted = (title, hint, inner) => CC.card(title, inner).replace('</span><div class="card-body">', ` ${info(hint)}</span><div class="card-body">`);
  const GROUPS = {
    row: [
      ['folderOpen', 'folderLink', 'folderOther', 'folderTab', 'folderBookmark', 'view', 'edit', 'unpack', 'testArchive'],
      ['copy', 'move', 'rename', 'clipCut', 'clipCopy'],
      ['share', 'collect', 'download', 'pack', 'checksum', 'symlink', 'diff'],
      ['labelColour', 'labelTags'],
      ['props', 'fixPerms', 'copyPath'],
      ['delete'],
    ],
    list: [
      ['newFolder', 'newFile', 'upload', 'clipPaste'],
      ['selectAll', 'selectPattern'],
      ['refresh', 'props', 'fixPerms', 'terminal'],
    ],
  };
  const OWN = {
    folderOpen: ['Öffnen', 'IconFolderOpen'], folderLink: ['Internen Link kopieren', 'IconLink'], folderOther: ['Im anderen Fenster öffnen', 'IconPaneSend'],
    folderTab: ['In neuem Tab öffnen', 'IconAdd'], folderBookmark: ['Als Verknüpfung anlegen', 'IconStar'], labelColour: ['Farbe', 'IconTabLook'], labelTags: ['Schlagworte', 'IconHash'],
  };
  const TEXT = {
    row: ['Menü für Dateien und Ordner', 'Blende aus, was du nie brauchst, und schieb Einträge innerhalb ihrer Gruppe nach oben oder unten.'],
    list: ['Menü für den freien Bereich', 'Erscheint beim Rechtsklick neben die Dateien.'],
  };

  const hidden = () => (Array.isArray(CC.prefs.menuHidden) ? CC.prefs.menuHidden : []);
  const orders = () => (CC.prefs.menuOrder && typeof CC.prefs.menuOrder === 'object' ? CC.prefs.menuOrder : {});
  const isHidden = (kind, id) => hidden().includes(`${kind}:${id}`);
  const rowOf = (id) => {
    if (OWN[id]) return { id, label: OWN[id][0], g: OWN[id][1] };
    const a = CC.actions[id];
    return a ? { id, label: nameOf(a), g: a.g || 'IconFile' } : null;
  };
  // The rows the order names fill the places they held, so the ones it does not name stay where they are.
  const reorder = (list, order) => {
    const rank = (id) => order.indexOf(id);
    const slots = list.flatMap((id, i) => (rank(id) >= 0 ? [i] : []));
    const moved = slots.map(i => list[i]).sort((a, b) => rank(a) - rank(b));
    const out = [...list];
    slots.forEach((slot, n) => { out[slot] = moved[n]; });
    return out;
  };
  const groups = (kind) => GROUPS[kind].map(g => reorder(g, orders()[kind] || []));
  const customised = (kind) => (orders()[kind] || []).length > 0 || hidden().some(k => k.startsWith(kind + ':'));

  function move(kind, id, step) {
    const now = groups(kind);
    const list = now.find(l => l.includes(id));
    const i = list.indexOf(id), j = i + step;
    if (j < 0 || j >= list.length) return;
    [list[i], list[j]] = [list[j], list[i]];
    CC.setPref('menuOrder', { ...orders(), [kind]: now.flat() });
  }
  function setHidden(kind, id, off) {
    const key = `${kind}:${id}`;
    CC.setPref('menuHidden', off ? [...new Set([...hidden(), key])] : hidden().filter(k => k !== key));
  }
  function reset(kind) {
    CC.setPref('menuHidden', hidden().filter(k => !k.startsWith(kind + ':')));
    const { [kind]: _, ...rest } = orders();
    CC.setPref('menuOrder', rest);
  }

  const row = (kind, r, i, n) => `<div class="frow ${isHidden(kind, r.id) ? 'sm-off' : ''}" data-menu-row="${kind}:${r.id}">
      <span class="lab"><span class="sm-name">${g(r.g)}${esc(r.label)}</span></span>
      <span class="sm-acts"><button class="btn icon sm-move" data-sm-up="${kind}:${r.id}" ${i === 0 ? 'disabled' : ''} data-tip="${esc(r.label)} nach oben" aria-label="${esc(r.label)} nach oben">${g('IconMoveUp')}</button><button class="btn icon sm-move" data-sm-down="${kind}:${r.id}" ${i === n - 1 ? 'disabled' : ''} data-tip="${esc(r.label)} nach unten" aria-label="${esc(r.label)} nach unten">${g('IconMoveDown')}</button>${sw(!isHidden(kind, r.id), `data-sm-show="${kind}:${r.id}" aria-label="${esc(r.label)}"`)}</span></div>`;

  CC.settingsTab({ id: 'ctxmenu', label: 'Kontextmenü', g: 'IconMenu', order: 45, render(host) {
    host.innerHTML = ['row', 'list'].map(kind => {
      const shown = groups(kind).map(list => list.map(rowOf).filter(Boolean)).filter(list => list.length);
      const body = `<div class="sm-groups">${shown.map(list => `<div class="sm-group">${list.map((r, i) => row(kind, r, i, list.length)).join('')}</div>`).join('')}</div>`
        + frow('Ursprüngliches Menü', `<button class="btn" data-sm-reset="${kind}" ${customised(kind) ? '' : 'disabled'}>${g('IconResetArrow')}Zurücksetzen</button>`, 'Zeigt wieder alle Einträge in der gewohnten Reihenfolge.');
      return hinted(TEXT[kind][0], TEXT[kind][1], body);
    }).join('');
    host.addEventListener('click', e => {
      const up = e.target.closest('[data-sm-up]'), down = e.target.closest('[data-sm-down]'), show = e.target.closest('[data-sm-show]'), rs = e.target.closest('[data-sm-reset]');
      if (up || down) {
        const [kind, id] = (up || down).dataset[up ? 'smUp' : 'smDown'].split(':');
        move(kind, id, up ? -1 : 1);
      } else if (show) {
        const [kind, id] = show.dataset.smShow.split(':');
        setHidden(kind, id, !isHidden(kind, id));
      } else if (rs) reset(rs.dataset.smReset);
      else return;
      CC.render();
    });
  } });

  CC.css(`
.sm-groups { display: flex; flex-direction: column; gap: .5rem; }
.sm-group + .sm-group { padding-top: .5rem; border-top: 1px solid color-mix(in srgb, var(--carbon-text) 12%, transparent); }
.sm-name { display: inline-flex; align-items: center; gap: .6rem; }
.sm-name svg { width: 1rem; height: 1rem; flex: none; color: var(--accent-glyph); }
.sm-acts { display: inline-flex; align-items: center; gap: .25rem; }
.sm-off .sm-name { opacity: .45; }
.sm-acts .btn.sm-move { opacity: 0; transition: opacity var(--motion-control-dur) ease; }
.sm-group > :is(:hover, :focus-within) .btn.sm-move { opacity: 1; }
@media (hover: none) { .sm-acts .btn.sm-move { opacity: 1; } }
`);
})();
