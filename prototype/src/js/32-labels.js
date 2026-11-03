// Colours and tags on files and folders. They live on the server, keyed by path, so every user sees them.
const files = (n) => n ? (n === 1 ? '1 Datei' : `${n} Dateien`) : 'noch keine Datei';
// A file left without colour and tags has no label entry at all.
const tidy = () => { for (const [p, l] of Object.entries(DATA.labels)) if (!l.color && !(l.tags || []).length) delete DATA.labels[p]; };
// German colour names for the name a picked colour proposes; the nearest one in OKLab wins, as the eye judges it.
const COLOR_NAMES = [
  ['Weiß', '#f7f7f5'], ['Elfenbein', '#fffff0'], ['Creme', '#f5ecd0'], ['Champagner', '#f1ddb2'], ['Beige', '#d9c8a9'], ['Sand', '#c8b48a'],
  ['Hellgrau', '#c8c8c8'], ['Silber', '#b4b8bc'], ['Grau', '#8a8a8a'], ['Steingrau', '#787366'], ['Schiefer', '#5a6470'], ['Anthrazit', '#383e42'], ['Schwarz', '#141414'],
  ['Rot', '#e53935'], ['Karmin', '#c3002f'], ['Kirschrot', '#b5122e'], ['Rubin', '#9b111e'], ['Weinrot', '#7b1f2e'], ['Bordeaux', '#5e1224'],
  ['Ziegelrot', '#b5452c'], ['Rostrot', '#8f3b1b'], ['Koralle', '#ff7f61'], ['Lachs', '#fa8c73'], ['Altrosa', '#c08081'], ['Rosa', '#f4a6c0'],
  ['Pink', '#ff4fa3'], ['Magenta', '#d6008f'], ['Himbeere', '#c51f5d'],
  ['Orange', '#ff8a1f'], ['Apricot', '#fbb57c'], ['Pfirsich', '#ffcba4'], ['Kupfer', '#b87333'], ['Terrakotta', '#c4622d'], ['Bernstein', '#ffb000'],
  ['Ocker', '#cc9a2e'], ['Senf', '#d2a91f'], ['Gold', '#d4af37'], ['Gelb', '#f7d417'], ['Zitronengelb', '#fff44f'], ['Vanille', '#f3e5ab'],
  ['Limette', '#a6d608'], ['Gelbgrün', '#9acd32'], ['Maigrün', '#4caf50'], ['Grün', '#2e9e4f'], ['Grasgrün', '#5aa832'], ['Smaragd', '#079b6b'],
  ['Tannengrün', '#1e4d36'], ['Olivgrün', '#6b7a2a'], ['Oliv', '#808000'], ['Khaki', '#b5a86a'], ['Salbei', '#9caf88'], ['Mint', '#98e2c6'], ['Pistazie', '#b6d39a'],
  ['Türkis', '#1fc0b4'], ['Aquamarin', '#6fd6c8'], ['Petrol', '#1b6b73'], ['Cyan', '#18c3e6'], ['Himmelblau', '#6ec3f2'], ['Hellblau', '#a7d3f5'],
  ['Eisblau', '#cfe8f7'], ['Azurblau', '#2a8ff7'], ['Blau', '#1e6fe0'], ['Königsblau', '#2340b5'], ['Stahlblau', '#4682b4'], ['Taubenblau', '#7d93a8'],
  ['Jeansblau', '#4a6a94'], ['Marine', '#1b2a5a'], ['Nachtblau', '#0f1b3d'], ['Indigo', '#3f3fb5'],
  ['Lavendel', '#b7a5e3'], ['Flieder', '#c8a2d6'], ['Violett', '#7f3fbf'], ['Lila', '#9b59d0'], ['Purpur', '#80207a'], ['Pflaume', '#6e2a5a'],
  ['Aubergine', '#4b1f3f'], ['Mauve', '#b784a7'],
  ['Braun', '#7b4a2a'], ['Schokobraun', '#4e2c1c'], ['Kaffee', '#6f4e37'], ['Haselnuss', '#a67b5b'], ['Karamell', '#c68e4f'], ['Zimt', '#a5603a'], ['Taupe', '#8b7d6b'],
];
const rgbOf = (hex) => [1, 3, 5].map(i => parseInt(hex.slice(i, i + 2), 16));
const hexOf = (rgb) => '#' + rgb.map(v => Math.round(Math.max(0, Math.min(255, v))).toString(16).padStart(2, '0')).join('');
const oklab = (hex) => {
  const [r, g1, b] = rgbOf(hex).map(v => { v /= 255; return v <= .04045 ? v / 12.92 : ((v + .055) / 1.055) ** 2.4; });
  const l = Math.cbrt(.4122214708 * r + .5363325363 * g1 + .0514459929 * b);
  const m = Math.cbrt(.2119034982 * r + .6806995451 * g1 + .1073969566 * b);
  const s = Math.cbrt(.0883024619 * r + .2817188376 * g1 + .6299787005 * b);
  return [.2104542553 * l + .7936177850 * m - .0040720468 * s, 1.9779984951 * l - 2.4285922050 * m + .4505937099 * s, .0259040371 * l + .7827717662 * m - .8086757660 * s];
};
const NAMED_LAB = COLOR_NAMES.map(([n, h]) => [n, oklab(h)]);
const nameOf = (hex) => {
  const [L, A, B] = oklab(hex);
  return NAMED_LAB.reduce((best, [n, [l, a, b]]) => { const d = (L - l) ** 2 + (A - a) ** 2 + (B - b) ** 2; return d < best[1] ? [n, d] : best; }, ['', Infinity])[0];
};
const hsvOf = (hex) => {
  const [r, g1, b] = rgbOf(hex).map(v => v / 255), max = Math.max(r, g1, b), d = max - Math.min(r, g1, b);
  const h = !d ? 0 : max === r ? ((g1 - b) / d) % 6 : max === g1 ? (b - r) / d + 2 : (r - g1) / d + 4;
  return [(h * 60 + 360) % 360, max ? d / max : 0, max];
};
const hexOfHsv = (h, s, v) => {
  const f = (n) => { const k = (n + h / 60) % 6; return v - v * s * Math.max(0, Math.min(k, 4 - k, 1)); };
  return hexOf([f(5), f(3), f(1)].map(x => x * 255));
};
// A colour picker in the house style: a saturation and brightness field, a hue strip, the hex value and the name it proposes.
function pickColour(anchor, hex, done) {
  $$('.cpick, .bubble').forEach(p => p.remove());
  let [h, s, v] = hsvOf(hex);
  const p = document.createElement('div');
  p.className = 'cpick glim-card';
  p.innerHTML = `<div class="cp-sv" tabindex="0" role="slider" aria-label="Sättigung und Helligkeit"><i class="cp-knob"></i></div>
    <div class="cp-hue" tabindex="0" role="slider" aria-label="Farbton" aria-valuemin="0" aria-valuemax="359"><i class="cp-knob"></i></div>
    <div class="cp-foot"><i class="cp-now"></i><span class="cp-name"></span><input class="field cp-hex mono" maxlength="7" aria-label="Farbwert" spellcheck="false"></div>
    <div class="cp-act"><button class="btn primary cp-ok">${g('IconCheck')}Übernehmen</button></div>`;
  document.body.appendChild(p);
  const r = anchor.getBoundingClientRect(), box = p.getBoundingClientRect();
  p.style.left = Math.max(8, Math.min(r.left, innerWidth - box.width - 8)) + 'px';
  p.style.top = (r.bottom + box.height + 8 > innerHeight ? Math.max(8, r.top - box.height - 6) : r.bottom + 6) + 'px';
  const sv = $('.cp-sv', p), hue = $('.cp-hue', p), hexIn = $('.cp-hex', p);
  const cur = () => hexOfHsv(h, s, v);
  const paint = (fromHex) => {
    const c = cur();
    p.style.setProperty('--h', h);
    p.style.setProperty('--c', c);
    $('.cp-knob', sv).style.cssText = `left:${s * 100}%;top:${(1 - v) * 100}%`;
    $('.cp-knob', hue).style.left = `${h / 3.6}%`;
    sv.setAttribute('aria-valuetext', `Sättigung ${Math.round(s * 100)} %, Helligkeit ${Math.round(v * 100)} %`);
    hue.setAttribute('aria-valuenow', Math.round(h));
    $('.cp-name', p).textContent = nameOf(c);
    if (!fromHex) hexIn.value = c;
  };
  // Pointer capture keeps the drag going when the pointer leaves the field, with mouse, pen and finger alike.
  const drag = (el, apply) => el.addEventListener('pointerdown', e => {
    e.preventDefault();
    el.focus();
    el.setPointerCapture(e.pointerId);
    const at = (ev) => { const b = el.getBoundingClientRect(); apply(Math.max(0, Math.min(1, (ev.clientX - b.left) / b.width)), Math.max(0, Math.min(1, (ev.clientY - b.top) / b.height))); paint(); };
    at(e);
    el.onpointermove = (ev) => { if (el.hasPointerCapture(ev.pointerId)) at(ev); };
  });
  drag(sv, (x, y) => { s = x; v = 1 - y; });
  drag(hue, (x) => { h = Math.min(359, x * 360); });
  const close = () => { p.remove(); document.removeEventListener('mousedown', outside, true); document.removeEventListener('keydown', key, true); };
  const ok = () => { close(); done(cur(), nameOf(cur())); };
  const outside = (e) => { if (!p.contains(e.target) && e.target !== anchor) close(); };
  const key = (e) => {
    if (e.key === 'Escape') { e.stopPropagation(); e.preventDefault(); return close(); }
    if (e.key === 'Enter' && p.contains(e.target)) { e.preventDefault(); return ok(); }
    const step = e.shiftKey ? .1 : .02, dx = { ArrowLeft: -1, ArrowRight: 1 }[e.key] || 0, dy = { ArrowUp: 1, ArrowDown: -1 }[e.key] || 0;
    if (!dx && !dy) return;
    if (e.target === sv) { s = Math.max(0, Math.min(1, s + dx * step)); v = Math.max(0, Math.min(1, v + dy * step)); }
    else if (e.target === hue) h = (h + (dx || dy) * step * 360 + 360) % 360;
    else return;
    e.preventDefault();
    paint();
  };
  $('.cp-ok', p).addEventListener('click', ok);
  hexIn.addEventListener('input', () => { const t = hexIn.value.trim(); if (/^#[0-9a-f]{6}$/i.test(t)) { [h, s, v] = hsvOf(t.toLowerCase()); paint(true); } });
  document.addEventListener('mousedown', outside, true);
  document.addEventListener('keydown', key, true);
  paint();
  sv.focus();
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
.cpick { --h: 0; --c: #888; position: fixed; z-index: 95; width: 18.5rem; padding: .8rem; display: flex; flex-direction: column; gap: .75rem; }
.cp-sv { position: relative; height: 10rem; border-radius: var(--radius-control); background: linear-gradient(to top, #000, transparent), linear-gradient(to right, #fff, hsl(var(--h) 100% 50%)); cursor: crosshair; touch-action: none; outline: none; }
.cp-hue { position: relative; height: .8rem; border-radius: var(--radius-pill); background: linear-gradient(90deg, #f00, #ff0 17%, #0f0 33%, #0ff 50%, #00f 67%, #f0f 83%, #f00); cursor: pointer; touch-action: none; outline: none; }
.cp-knob { position: absolute; top: 50%; width: 1.1rem; height: 1.1rem; margin: -.55rem 0 0 -.55rem; border-radius: 50%; background: var(--c); box-shadow: 0 0 0 2px #fff, 0 1px 4px rgba(0, 0, 0, .45); pointer-events: none; }
.cp-hue .cp-knob { background: hsl(var(--h) 100% 50%); }
.cp-sv:focus-visible, .cp-hue:focus-visible { box-shadow: 0 0 0 2px var(--accent); }
.cp-foot { display: flex; align-items: center; gap: .55rem; }
.cp-now { width: 1.6rem; height: 1.6rem; border-radius: 50%; background: var(--c); flex: none; box-shadow: 0 0 0 2px color-mix(in srgb, var(--c) 30%, transparent); }
.cp-name { flex: 1; min-width: 0; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cp-hex { width: 6.2rem; flex: none; }
.cp-act { display: flex; justify-content: flex-end; }
.mk-none { font-size: 13px; padding: .4rem 0; }
.tagchips { display: flex; flex-wrap: wrap; gap: .4rem; margin-top: .8rem; }
.tagchip .x { width: 1.2rem; height: 1.2rem; margin-right: -.45rem; display: grid; place-items: center; border-radius: 50%; }
.tagchip .x svg { width: .6rem; height: .6rem; color: currentColor; }
.tagchip .x:hover { background: color-mix(in srgb, var(--accent-contrast) 18%, transparent); }
.tagchip { height: 1.8rem; padding: 0 .8rem; display: inline-flex; align-items: center; gap: .35rem; border-radius: var(--radius-pill); background: var(--carbon-surface3); color: var(--carbon-text-sub); }
.tagchip.on { background: var(--accent); color: var(--accent-contrast); }
`);
