// Colours and tags on files and folders. They live on the server, keyed by path, so every user sees them.
const files = (n) => n ? (n === 1 ? '1 Datei' : `${n} Dateien`) : 'noch keine Datei';
// A file left without colour and tags has no label entry at all.
const tidy = () => { for (const [p, l] of Object.entries(DATA.labels)) if (!l.color && !(l.tags || []).length) delete DATA.labels[p]; };
// Named shades for the colour picker; a picked colour is called after the nearest of them.
const COLOR_NAMES = [
  ['Rot', '#ff6b72'], ['Koralle', '#ff8c6b'], ['Orange', '#ff8a3d'], ['Bernstein', '#ffb020'], ['Gelb', '#f5c518'], ['Limette', '#a3d64b'],
  ['Grün', '#4cc977'], ['Smaragd', '#1fae7a'], ['Türkis', '#2ec4b6'], ['Cyan', '#2bb8e0'], ['Blau', '#3b9cf5'], ['Indigo', '#5b6cf0'],
  ['Lila', '#a98bff'], ['Violett', '#c06cf0'], ['Pink', '#f06cb8'], ['Rosa', '#ff9ec4'], ['Braun', '#a87a55'], ['Grau', '#9a9a9a'],
];
const rgbOf = (hex) => [1, 3, 5].map(i => parseInt(hex.slice(i, i + 2), 16));
const nameOf = (hex) => {
  const [r, g1, b] = rgbOf(hex);
  return COLOR_NAMES.map(([n, h]) => { const [r2, g2, b2] = rgbOf(h); return [n, (r - r2) ** 2 + (g1 - g2) ** 2 + (b - b2) ** 2]; }).sort((a, z) => a[1] - z[1])[0][0];
};
const hexOfHue = (h) => {
  const f = (n) => { const k = (n + h / 30) % 12, a = .62 * Math.min(.62, 1 - .62); return Math.round(255 * (.62 - a * Math.max(-1, Math.min(k - 3, 9 - k, 1)))).toString(16).padStart(2, '0'); };
  return '#' + f(0) + f(8) + f(4);
};
// The picker opens under its swatch: the named shades, a hue strip for anything between them and the hex value.
function pickColour(anchor, hex, done) {
  $$('.cpick, .bubble').forEach(p => p.remove());
  const p = document.createElement('div');
  p.className = 'cpick glim-card';
  p.innerHTML = `<div class="cp-grid">${COLOR_NAMES.map(([n, h]) => `<button class="cp-sw ${h === hex ? 'on' : ''}" data-hex="${h}" style="--c:${h}" data-tip="${n}" aria-label="${n}"></button>`).join('')}</div>
    <input type="range" class="cp-hue" min="0" max="359" value="0" aria-label="Farbton">
    <div class="cp-foot"><i class="cp-now" style="--c:${hex}"></i><input class="field cp-hex mono" value="${hex}" maxlength="7" aria-label="Farbwert" spellcheck="false"><button class="btn primary cp-ok">${g('IconCheck')}Übernehmen</button></div>`;
  document.body.appendChild(p);
  const r = anchor.getBoundingClientRect(), box = p.getBoundingClientRect();
  p.style.left = Math.max(8, Math.min(r.left, innerWidth - box.width - 8)) + 'px';
  p.style.top = (r.bottom + box.height + 8 > innerHeight ? r.top - box.height - 6 : r.bottom + 6) + 'px';
  let cur = hex;
  const set = (h) => { cur = h; $('.cp-now', p).style.setProperty('--c', h); $('.cp-hex', p).value = h; $$('.cp-sw', p).forEach(s => s.classList.toggle('on', s.dataset.hex === h)); };
  const close = () => { p.remove(); document.removeEventListener('mousedown', outside, true); document.removeEventListener('keydown', key, true); };
  const ok = () => { close(); done(cur, nameOf(cur)); };
  const outside = (e) => { if (!p.contains(e.target) && e.target !== anchor) close(); };
  const key = (e) => { if (e.key === 'Escape') { e.stopPropagation(); e.preventDefault(); close(); } if (e.key === 'Enter' && p.contains(e.target)) { e.preventDefault(); ok(); } };
  p.addEventListener('click', e => { const s = e.target.closest('[data-hex]'); if (s) { set(s.dataset.hex); ok(); } if (e.target.closest('.cp-ok')) ok(); });
  $('.cp-hue', p).addEventListener('input', e => set(hexOfHue(+e.target.value)));
  $('.cp-hex', p).addEventListener('input', e => { const v = e.target.value.trim(); if (/^#[0-9a-f]{6}$/i.test(v)) { cur = v.toLowerCase(); $('.cp-now', p).style.setProperty('--c', cur); } });
  document.addEventListener('mousedown', outside, true);
  document.addEventListener('keydown', key, true);
}

const LABEL_COLORS = [['red', 'Rot', '#ff6b72'], ['orange', 'Orange', '#ff8a3d'], ['yellow', 'Gelb', '#f5c518'], ['green', 'Grün', '#4cc977'], ['blue', 'Blau', '#3b9cf5'], ['purple', 'Lila', '#a98bff']];
// Tags that no file carries yet, created in the Markierungen window.
DATA.tags = [];
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
    const all = Labels.tags();
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
  tags: () => [...new Set([...Object.values(DATA.labels).flatMap(l => l.tags || []), ...DATA.tags])].sort((a, b) => a.localeCompare(b, 'de')),
  counts() {
    const counts = {};
    for (const l of Object.values(DATA.labels)) {
      if (l.color) counts['c:' + l.color] = (counts['c:' + l.color] || 0) + 1;
      for (const t of l.tags || []) counts['t:' + t] = (counts['t:' + t] || 0) + 1;
    }
    return counts;
  },
  // Every colour and tag, for the labels button; picking one searches the shares for it.
  menu(btn) {
    const counts = Labels.counts();
    const search = (word) => () => { Files.go('/mnt/user'); Object.assign(Files.T(), { filter: word, qf: true, deep: true, cursor: 0 }); CC.render(); };
    const colours = LABEL_COLORS.map(([id, name, hex]) => ({ label: name, color: hex, count: counts['c:' + id] || '', run: search('#' + name.toLowerCase()) }));
    const tags = Labels.tags().map(t => ({ label: t, g: 'IconHash', count: counts['t:' + t] || '', run: search('#' + t.toLowerCase()) }));
    const r = btn.getBoundingClientRect();
    menu(r.left, r.bottom + 6, [
      { head: 'Farben', g: 'IconPencil', tip: 'Farben bearbeiten', run: () => Labels.editColours() }, ...colours,
      { head: 'Schlagworte', g: 'IconPencil', tip: 'Schlagworte bearbeiten', run: () => Labels.editTags() },
      ...(tags.length ? tags : [{ label: 'Noch keine Schlagworte', g: 'IconInfo', run: () => Labels.editTags() }]),
    ]);
  },
  // The colours: rename one, give it another shade, delete it or add a new one.
  editColours() {
    const counts = Labels.counts();
    const rows = LABEL_COLORS.map(([id, name, hex]) => ({ id, name, hex }));
    const fresh = { hex: '#2ec4b6', named: '' };
    const row = (c, i) => `<div class="mk-row"><button class="mk-swatch" data-colpick="${i}" style="--c:${c.hex}" data-tip="Farbton wählen" aria-label="Farbton für ${esc(c.name)}"></button><input class="field" data-colname="${i}" value="${esc(c.name)}" aria-label="Name für ${esc(c.name)}"><span class="muted">${files(counts['c:' + c.id])}</span><button class="btn icon" data-coldel="${i}" data-tip="Farbe entfernen" aria-label="${esc(c.name)} entfernen">${g('IconTrash')}</button></div>`;
    const list = () => rows.length ? rows.map(row).join('') : '<div class="muted mk-none">Keine Farben</div>';
    const w = win({ title: 'Farben',
      body: `<div class="well">${frow('Neue Farbe', `<span class="mk-new"><button class="mk-swatch" data-colpick="new" style="--c:${fresh.hex}" data-tip="Farbton wählen, der Name kommt von selbst" aria-label="Farbton der neuen Farbe"></button><input class="field m" id="mk-new" placeholder="etwa Türkis" autocomplete="off"><button class="btn icon" data-coladd data-tip="Hinzufügen" aria-label="Hinzufügen">${g('IconAdd')}</button></span>`, 'Wähl am Punkt einen Farbton, der Name wird vorgeschlagen. Mit Enter oder Plus kommt die Farbe in die Liste.')}<div id="mk-list">${list()}</div></div>`,
      foot: [cancel, { label: 'Übernehmen', g: 'IconCheck', primary: true, run: (w) => {
        keep(w);
        const kept = new Set(rows.map(r => r.id));
        for (const l of Object.values(DATA.labels)) if (l.color && !kept.has(l.color)) delete l.color;
        tidy();
        LABEL_COLORS.splice(0, LABEL_COLORS.length, ...rows.map(r => [r.id, r.name.trim() || 'Farbe', r.hex]));
        CC.render();
        CC.toast('Farben gespeichert');
      } }] });
    // The list is redrawn after a delete or an add, so what was typed is read back first.
    const keep = () => $$('[data-colname]', w).forEach(i => { rows[+i.dataset.colname].name = i.value; });
    const add = () => {
      const input = $('#mk-new', w), v = input.value.trim() || nameOf(fresh.hex);
      keep();
      rows.push({ id: 'c' + Date.now(), name: v, hex: fresh.hex });
      input.value = '';
      fresh.named = '';
      $('#mk-list', w).innerHTML = list();
    };
    w.addEventListener('click', e => {
      const d = e.target.closest('[data-coldel]'), pick = e.target.closest('[data-colpick]');
      if (e.target.closest('[data-coladd]')) return add();
      if (pick) {
        const i = pick.dataset.colpick, isNew = i === 'new';
        return pickColour(pick, isNew ? fresh.hex : rows[+i].hex, (hex, name) => {
          pick.style.setProperty('--c', hex);
          if (!isNew) { rows[+i].hex = hex; return; }
          fresh.hex = hex;
          // The suggestion only fills the field while nobody has typed a name of their own.
          const input = $('#mk-new', w);
          if (!input.value.trim() || input.value === fresh.named) { input.value = name; fresh.named = name; }
          input.focus();
        });
      }
      if (!d) return;
      keep();
      rows.splice(+d.dataset.coldel, 1);
      $('#mk-list', w).innerHTML = list();
    });
    $('#mk-new', w).addEventListener('keydown', e => { if (e.key === 'Enter') { e.preventDefault(); add(); } });
  },
  // The tags: rename one, delete it or add a new one before any file carries it.
  editTags() {
    const counts = Labels.counts();
    const tags = Labels.tags().map(t => ({ from: t, to: t }));
    const list = () => tags.length ? tags.map((t, i) => `<div class="mk-row">${g('IconHash')}<input class="field" data-tagname="${i}" value="${esc(t.to)}" aria-label="Schlagwort ${esc(t.from)}"><span class="muted">${files(counts['t:' + t.from])}</span><button class="btn icon" data-tagdel="${i}" data-tip="Schlagwort entfernen" aria-label="${esc(t.from)} entfernen">${g('IconTrash')}</button></div>`).join('') : '<div class="muted mk-none">Noch keine Schlagworte</div>';
    const w = win({ title: 'Schlagworte',
      body: `<div class="well">${frow('Neues Schlagwort', '<input class="field m" id="mk-new" placeholder="etwa Urlaub" autocomplete="off">', 'Mit Enter kommt es in die Liste.')}<div id="mk-list">${list()}</div></div>`,
      foot: [cancel, { label: 'Übernehmen', g: 'IconCheck', primary: true, run: () => {
        keep();
        const rename = new Map(tags.map(t => [t.from, t.to.trim().replace(/^#/, '') || t.from]));
        for (const l of Object.values(DATA.labels)) l.tags = [...new Set((l.tags || []).filter(t => rename.has(t)).map(t => rename.get(t)))];
        tidy();
        DATA.tags = [...rename.values()];
        CC.render();
        CC.toast('Schlagworte gespeichert');
      } }] });
    const keep = () => $$('[data-tagname]', w).forEach(i => { tags[+i.dataset.tagname].to = i.value; });
    w.addEventListener('click', e => {
      const d = e.target.closest('[data-tagdel]');
      if (!d) return;
      keep();
      tags.splice(+d.dataset.tagdel, 1);
      $('#mk-list', w).innerHTML = list();
    });
    $('#mk-new', w).addEventListener('keydown', e => {
      const v = e.target.value.trim().replace(/^#/, '');
      if (e.key !== 'Enter' || !v) return;
      e.preventDefault();
      keep();
      if (!tags.some(t => t.to === v)) tags.push({ from: v, to: v });
      e.target.value = '';
      $('#mk-list', w).innerHTML = list();
    });
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
.mk-row { display: grid; grid-template-columns: 1.4rem minmax(0, 1fr) 8rem 1.9rem; align-items: center; gap: .6rem; min-height: 2.5rem; }
.mk-row > svg { width: .9rem; height: .9rem; color: var(--accent-glyph); justify-self: center; }
.mk-row .cdot.big { justify-self: center; margin: 0; }
.mk-row .muted { font-size: 12px; text-align: right; }
.mk-row .btn { width: 1.9rem; height: 1.9rem; }
#mk-list { margin-top: .4rem; }
.mk-swatch { width: 1.3rem; height: 1.3rem; border-radius: 50%; background: var(--c); box-shadow: 0 0 0 2px color-mix(in srgb, var(--c) 30%, transparent); justify-self: center; flex: none; }
.mk-swatch:hover { box-shadow: 0 0 0 3px color-mix(in srgb, var(--c) 45%, transparent); }
.mk-new { display: flex; align-items: center; gap: .55rem; }
.mk-new .btn { width: 2rem; height: 2rem; }
.cpick { position: fixed; z-index: 95; width: 17.5rem; padding: .8rem; display: flex; flex-direction: column; gap: .75rem; }
.cp-grid { display: grid; grid-template-columns: repeat(6, 1fr); gap: .45rem; }
.cp-sw { aspect-ratio: 1; border-radius: 50%; background: var(--c); }
.cp-sw:hover { box-shadow: 0 0 0 3px color-mix(in srgb, var(--c) 40%, transparent); }
.cp-sw.on { box-shadow: 0 0 0 2px var(--carbon-surface), 0 0 0 4px var(--carbon-text); }
.cp-hue { -webkit-appearance: none; appearance: none; width: 100%; height: .7rem; border-radius: var(--radius-pill); background: linear-gradient(90deg, #ff6b6b, #ffd166, #6bdc8c, #4dd4e6, #6b8cff, #c56bff, #ff6b6b); cursor: pointer; }
.cp-hue::-webkit-slider-thumb { -webkit-appearance: none; width: 1.1rem; height: 1.1rem; border-radius: 50%; background: var(--carbon-surface); box-shadow: 0 0 0 2px var(--carbon-text); }
.cp-hue::-moz-range-thumb { width: 1.1rem; height: 1.1rem; border: 0; border-radius: 50%; background: var(--carbon-surface); box-shadow: 0 0 0 2px var(--carbon-text); }
.cp-foot { display: flex; align-items: center; gap: .5rem; }
.cp-now { width: 1.6rem; height: 1.6rem; border-radius: 50%; background: var(--c); flex: none; }
.cp-hex { flex: 1; min-width: 0; }
.mk-none { font-size: 13px; padding: .4rem 0; }
.tagchips { display: flex; flex-wrap: wrap; gap: .4rem; margin-top: .8rem; }
.tagchip .x { width: 1.2rem; height: 1.2rem; margin-right: -.45rem; display: grid; place-items: center; border-radius: 50%; }
.tagchip .x svg { width: .6rem; height: .6rem; color: currentColor; }
.tagchip .x:hover { background: color-mix(in srgb, var(--accent-contrast) 18%, transparent); }
.tagchip { height: 1.8rem; padding: 0 .8rem; display: inline-flex; align-items: center; gap: .35rem; border-radius: var(--radius-pill); background: var(--carbon-surface3); color: var(--carbon-text-sub); }
.tagchip.on { background: var(--accent); color: var(--accent-contrast); }
`);
