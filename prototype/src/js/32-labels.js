// Colours and tags on files and folders. They live on the server, keyed by path, so every user sees them.
const LABEL_COLORS = [['red', 'Rot', '#ff6b72'], ['orange', 'Orange', '#ff8a3d'], ['yellow', 'Gelb', '#f5c518'], ['green', 'Grün', '#4cc977'], ['blue', 'Blau', '#3b9cf5'], ['purple', 'Lila', '#a98bff'], ['gray', 'Grau', '#9a9a9a']];
DATA.labels = {
  '/mnt/user/dokumente/Mietvertrag.pdf': { color: 'red', tags: ['wichtig'] },
  '/mnt/user/dokumente/Steuer 2025.xlsx': { color: 'orange', tags: ['prüfen', 'Steuer'] },
  '/mnt/user/dokumente/Rechnung-Oktober.pdf': { tags: ['Steuer'] },
  '/mnt/user/downloads/ubuntu-24.04.3-desktop-amd64.iso': { color: 'green', tags: ['fertig'] },
  '/mnt/user/fotos/2026/IMG_0203.jpg': { color: 'purple', tags: ['für Anna'] },
};

const Labels = {
  pathOf: (k, folder) => FS.join(k.at || folder, k.name),
  of: (k, folder) => DATA.labels[Labels.pathOf(k, folder)] || {},
  color: (id) => LABEL_COLORS.find(c => c[0] === id),
  set(items, folder, change) {
    for (const k of items) {
      const p = Labels.pathOf(k, folder), next = change({ tags: [], ...DATA.labels[p] });
      if (!next.color && !next.tags.length) delete DATA.labels[p]; else DATA.labels[p] = next;
    }
    CC.render();
  },
  // A search word starting with # finds a tag or a colour by its name, so #rot and #prüfen both work.
  matches(k, word) {
    const l = DATA.labels[FS.join(k.at, k.name)];
    if (!l) return false;
    const w = word.slice(1).toLowerCase();
    return (l.tags || []).some(t => t.toLowerCase() === w) || (l.color && Labels.color(l.color)[1].toLowerCase() === w);
  },
  marks(k, folder) {
    const l = Labels.of(k, folder), c = l.color && Labels.color(l.color);
    return `${c ? `<i class="cdot" style="--c:${c[2]}" title="${c[1]}"></i>` : ''}${(l.tags || []).map(t => `<span class="pill tag">#${esc(t)}</span>`).join('')}`;
  },
  menuItems(sel, folder) {
    const now = sel.length === 1 ? Labels.of(sel[0], folder).color : null;
    return [
      { label: 'Farbe', g: 'IconTabLook', sub: [
        ...LABEL_COLORS.map(([id, name, hex]) => ({ label: now === id ? name + ', gesetzt' : name, color: hex, run: () => Labels.set(sel, folder, l => ({ ...l, color: id })) })),
        '-', { label: 'Keine Farbe', g: 'IconClose', run: () => Labels.set(sel, folder, l => ({ ...l, color: undefined })) },
      ] },
      { label: 'Schlagworte', g: 'IconHash', run: () => Labels.edit(sel, folder) },
    ];
  },
  edit(sel, folder) {
    const all = [...new Set(Object.values(DATA.labels).flatMap(l => l.tags || []))].sort((a, b) => a.localeCompare(b, 'de'));
    const has = (t) => sel.every(k => (Labels.of(k, folder).tags || []).includes(t));
    const picked = new Set(all.filter(has));
    const chips = () => [...picked].sort((a, b) => a.localeCompare(b, 'de')).map(t => `<span class="tagchip on">#${esc(t)}<button class="x" data-untag="${esc(t)}" data-tip="Entfernen" aria-label="${esc(t)} entfernen">${g('IconClose')}</button></span>`).join('')
      + [...all].filter(t => !picked.has(t)).map(t => `<button class="tagchip" data-tag="${esc(t)}" data-tip="Hinzufügen">#${esc(t)}</button>`).join('');
    const w = win({ title: sel.length === 1 ? `Schlagworte für ${sel[0].name}` : `Schlagworte für ${sel.length} Objekte`,
      body: `<div class="well">${frow('Schlagwort', '<input class="field m" id="newtag" placeholder="etwa prüfen" autocomplete="off">', 'Mit Enter kommt es dazu. Das x an einem Schlagwort nimmt es wieder weg, ein Klick auf ein graues setzt es.')}<div class="tagchips" id="tags">${chips()}</div></div>`,
      foot: [cancel, { label: 'Übernehmen', g: 'IconCheck', primary: true, run: () => Labels.set(sel, folder, l => ({ ...l, tags: [...picked] })) }] });
    w.addEventListener('click', e => {
      const off = e.target.closest('[data-untag]'), on = e.target.closest('[data-tag]');
      if (off) picked.delete(off.dataset.untag); else if (on) picked.add(on.dataset.tag); else return;
      $('#tags', w).innerHTML = chips();
    });
    $('#newtag', w).addEventListener('keydown', e => {
      if (e.key !== 'Enter' || !e.target.value.trim()) return;
      e.preventDefault();
      picked.add(e.target.value.trim().replace(/^#/, ''));
      e.target.value = '';
      $('#tags', w).innerHTML = chips();
    });
    $('#newtag', w).focus();
  },
  // Every colour and tag in use, for the labels button; picking one searches the shares for it.
  menu(btn) {
    const counts = {};
    for (const l of Object.values(DATA.labels)) {
      if (l.color) counts['c:' + l.color] = (counts['c:' + l.color] || 0) + 1;
      for (const t of l.tags || []) counts['t:' + t] = (counts['t:' + t] || 0) + 1;
    }
    const search = (word) => () => { Files.go('/mnt/user'); Object.assign(Files.T(), { filter: word, qf: true, deep: true, cursor: 0 }); CC.render(); };
    const colours = LABEL_COLORS.filter(c => counts['c:' + c[0]]).map(([id, name, hex]) => ({ label: name, color: hex, count: counts['c:' + id], run: search('#' + name.toLowerCase()) }));
    const tags = Object.keys(counts).filter(k => k.startsWith('t:')).sort().map(k => ({ label: k.slice(2), g: 'IconHash', count: counts[k], run: search('#' + k.slice(2).toLowerCase()) }));
    const r = btn.getBoundingClientRect();
    menu(r.left, r.bottom + 6, colours.length || tags.length
      ? [...(colours.length ? [{ head: 'Farben' }, ...colours] : []), ...(tags.length ? [{ head: 'Schlagworte' }, ...tags] : [])]
      : [{ label: 'Noch nichts markiert. Rechtsklick auf eine Datei setzt Farbe und Schlagworte', g: 'IconInfo', run: () => {} }]);
  },
};

document.addEventListener('click', e => {
  const b = CC.view === 'files' && e.target.closest('[data-labelsmenu]');
  if (b) Labels.menu(b);
});

CC.css(`
.cdot { width: .6rem; height: .6rem; border-radius: 50%; background: var(--c); flex: none; box-shadow: 0 0 0 2px color-mix(in srgb, var(--c) 25%, transparent); }
.cdot.big { width: .75rem; height: .75rem; margin: 0 .2rem; }
.pill.tag { height: 1.2rem; padding: 0 .45rem; font-size: 11px; background: color-mix(in srgb, var(--accent) 14%, var(--carbon-surface2)); color: var(--carbon-text-sub); }
.tagchips { display: flex; flex-wrap: wrap; gap: .4rem; margin-top: .8rem; }
.tagchip .x { width: 1.2rem; height: 1.2rem; margin-right: -.45rem; display: grid; place-items: center; border-radius: 50%; }
.tagchip .x svg { width: .6rem; height: .6rem; color: currentColor; }
.tagchip .x:hover { background: color-mix(in srgb, var(--accent-contrast) 18%, transparent); }
.tagchip { height: 1.8rem; padding: 0 .8rem; display: inline-flex; align-items: center; gap: .35rem; border-radius: var(--radius-pill); background: var(--carbon-surface3); color: var(--carbon-text-sub); }
.tagchip.on { background: var(--accent); color: var(--accent-contrast); }
`);
