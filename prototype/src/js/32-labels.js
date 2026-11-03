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
    const chips = () => [...new Set([...all, ...picked])].map(t => `<button class="tagchip ${picked.has(t) ? 'on' : ''}" data-tag="${esc(t)}">#${esc(t)}</button>`).join('');
    const w = win({ title: sel.length === 1 ? `Schlagworte für ${sel[0].name}` : `Schlagworte für ${sel.length} Objekte`,
      body: `<div class="well"><div class="tagchips" id="tags">${chips()}</div>
        ${frow('Neues Schlagwort', '<input class="field m" id="newtag" placeholder="etwa prüfen" autocomplete="off">', 'Mit Enter kommt es dazu. Ein Klick auf ein Schlagwort schaltet es an oder aus.')}</div>`,
      foot: [cancel, { label: 'Übernehmen', g: 'IconCheck', primary: true, run: () => Labels.set(sel, folder, l => ({ ...l, tags: [...picked] })) }] });
    w.addEventListener('click', e => { const c = e.target.closest('[data-tag]'); if (!c) return; picked.has(c.dataset.tag) ? picked.delete(c.dataset.tag) : picked.add(c.dataset.tag); $('#tags', w).innerHTML = chips(); });
    $('#newtag', w).addEventListener('keydown', e => {
      if (e.key !== 'Enter' || !e.target.value.trim()) return;
      e.preventDefault();
      picked.add(e.target.value.trim().replace(/^#/, ''));
      e.target.value = '';
      $('#tags', w).innerHTML = chips();
    });
    $('#newtag', w).focus();
  },
  // The sidebar card lists every colour and tag in use; a click searches the shares for it.
  card() {
    const counts = {};
    for (const l of Object.values(DATA.labels)) {
      if (l.color) counts['c:' + l.color] = (counts['c:' + l.color] || 0) + 1;
      for (const t of l.tags || []) counts['t:' + t] = (counts['t:' + t] || 0) + 1;
    }
    const keys = Object.keys(counts);
    if (!keys.length) return '';
    const rows = [...LABEL_COLORS.filter(c => counts['c:' + c[0]]).map(([id, name, hex]) => `<div class="nav-row" data-labelsearch="#${name.toLowerCase()}"><i class="cdot big" style="--c:${hex}"></i><span>${name}</span><span class="meta glim-num">${counts['c:' + id]}</span></div>`),
      ...keys.filter(k => k.startsWith('t:')).sort().map(k => `<div class="nav-row" data-labelsearch="#${esc(k.slice(2).toLowerCase())}">${g('IconHash')}<span>${esc(k.slice(2))}</span><span class="meta glim-num">${counts[k]}</span></div>`)];
    return `<section class="side-card"><span class="notch">Markierungen</span>${rows.join('')}</section>`;
  },
};

document.addEventListener('click', e => {
  const r = CC.view === 'files' && e.target.closest('[data-labelsearch]');
  if (!r) return;
  Files.go('/mnt/user');
  Object.assign(Files.T(), { filter: r.dataset.labelsearch, qf: true, deep: true, cursor: 0 });
  CC.render();
});

CC.css(`
.cdot { width: .6rem; height: .6rem; border-radius: 50%; background: var(--c); flex: none; box-shadow: 0 0 0 2px color-mix(in srgb, var(--c) 25%, transparent); }
.cdot.big { width: .75rem; height: .75rem; margin: 0 .2rem; }
.pill.tag { height: 1.2rem; padding: 0 .45rem; font-size: 11px; background: color-mix(in srgb, var(--accent) 14%, var(--carbon-surface2)); color: var(--carbon-text-sub); }
.tagchips { display: flex; flex-wrap: wrap; gap: .4rem; margin-bottom: .8rem; }
.tagchip { height: 1.8rem; padding: 0 .8rem; border-radius: var(--radius-pill); background: var(--carbon-surface3); color: var(--carbon-text-sub); }
.tagchip.on { background: var(--accent); color: var(--accent-contrast); }
`);
