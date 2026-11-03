// The command search: every action by its name, the recent ones on top while nothing is typed.
(() => {
  const KEY = 'cannoncommander-recent-commands';
  const recent = () => {
    try { const v = JSON.parse(localStorage.getItem(KEY) || '[]'); return Array.isArray(v) ? v : []; } catch { return []; }
  };
  const remember = (id) => { try { localStorage.setItem(KEY, JSON.stringify([id, ...recent().filter(x => x !== id)].slice(0, 6))); } catch {} };

  // A bare label such as Name only makes sense with the menu or heading it sits under.
  const groups = () => {
    const out = {};
    for (const [menuName, items] of MENUS) {
      let head = menuName;
      for (const it of items) {
        if (it === '-') head = menuName;
        else if (it.head) head = it.head;
        else if (typeof it === 'string' && !out[it]) out[it] = head;
      }
    }
    return out;
  };

  const fold = (s) => String(s).toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/ß/g, 'ss');
  const inOrder = (t, s) => { let i = 0; for (const c of s) if (c === t[i]) i++; return i === t.length; };
  // Every typed word has to show up somewhere; the start of a word counts most, letters in order least.
  function score(q, e) {
    const tokens = fold(q).split(/\s+/).filter(Boolean);
    const name = fold(e.name), words = ' ' + name.replace(/[^a-z0-9]+/g, ' '), group = fold(e.group || '');
    let s = name.startsWith(tokens.join(' ')) ? 3 : 0;
    for (const t of tokens) {
      const hit = words.includes(' ' + t) ? 3 : name.includes(t) ? 2 : group.includes(t) || inOrder(t, name) ? 1 : 0;
      if (!hit) return 0;
      s += hit;
    }
    return s;
  }

  function entries(q) {
    const grp = groups();
    const all = Object.values(CC.actions).filter(a => a.label && a.id !== 'palette').map(a => ({
      a, name: typeof a.label === 'function' ? a.label() : a.label, group: grp[a.id],
      why: CC.blocked(a) || (a.when && !a.when() ? 'Gerade nicht möglich' : null),
    }));
    const byName = (x, y) => !!x.why - !!y.why || x.name.localeCompare(y.name, 'de');
    if (!q.trim()) {
      const first = recent().map(id => all.find(e => e.a.id === id)).filter(Boolean);
      const rest = all.filter(e => !first.includes(e)).sort(byName);
      return first.length ? [{ head: 'Zuletzt benutzt' }, ...first, { head: 'Alle Befehle' }, ...rest] : rest;
    }
    return all.map(e => ({ ...e, score: score(q, e) })).filter(e => e.score).sort((x, y) => !!x.why - !!y.why || y.score - x.score || byName(x, y));
  }

  const lead = (a) => a.checked ? `<span class="cbox ${a.checked() ? 'on' : ''}">${g('IconCheck')}</span>` : a.g ? g(a.g) : '<span class="ph"></span>';
  let el = null;

  function open() {
    if (el) return $('#pal-q', el).focus();
    closeMenu();
    el = document.createElement('div');
    el.className = 'pal-scrim glim-modal-backdrop';
    el.innerHTML = `<div class="pal glim-card" role="dialog" aria-label="Befehl suchen">
      <label class="field pal-q">${g('IconSearch')}<input id="pal-q" role="combobox" aria-expanded="true" aria-controls="pal-list" aria-autocomplete="list" placeholder="Befehl suchen" autocomplete="off" spellcheck="false"></label>
      <div class="pal-list" id="pal-list" role="listbox" aria-label="Befehle"></div></div>`;
    document.body.appendChild(el);
    const input = $('#pal-q', el), list = $('#pal-list', el);
    let rows = [], cur = -1;
    const pickable = () => rows.flatMap((r, i) => r.head ? [] : [i]);

    const mark = () => {
      $$('.pal-row', list).forEach(r => { const on = +r.dataset.i === cur; r.classList.toggle('cur', on); r.setAttribute('aria-selected', on); });
      const c = $(`#pal-${cur}`, list);
      input.setAttribute('aria-activedescendant', c ? c.id : '');
      if (cur === pickable()[0]) list.scrollTop = 0;
      else c?.scrollIntoView({ block: 'nearest' });
    };
    const draw = () => {
      rows = entries(input.value);
      cur = pickable()[0] ?? -1;
      list.innerHTML = rows.length ? rows.map((r, i) => r.head ? `<div class="head" role="presentation">${esc(r.head)}</div>`
        : `<div class="pal-row ${r.why ? 'off' : ''}" id="pal-${i}" role="option" aria-selected="false" aria-disabled="${!!r.why}" data-i="${i}" ${r.why ? `data-tip="${esc(r.why)}"` : ''}>${lead(r.a)}<span class="t">${esc(r.name)}</span>${r.group ? `<span class="grp">${esc(r.group)}</span>` : ''}${r.a.key ? `<span class="kbd">${esc(r.a.key)}</span>` : ''}</div>`).join('')
        : `<div class="pal-none">Kein Befehl passt zu „${esc(input.value.trim())}“</div>`;
      mark();
    };
    const step = (d) => { const p = pickable(); if (p.length) { cur = p[(p.indexOf(cur) + d + p.length) % p.length]; mark(); } };
    const close = () => { el.remove(); el = null; $('.panel.active .list')?.focus(); };
    const go = (i) => {
      const r = rows[i];
      if (!r || r.head) return;
      if (r.why) return CC.toast(r.why, 'warn');
      close();
      remember(r.a.id);
      CC.run(r.a.id);
    };

    input.addEventListener('input', draw);
    // The frame's own keys stay out while the search is open, so typing never runs a shortcut underneath.
    input.addEventListener('keydown', e => {
      e.stopPropagation();
      const k = e.key === 'Tab' ? (e.shiftKey ? 'ArrowUp' : 'ArrowDown') : e.key;
      if (k === 'ArrowDown' || k === 'ArrowUp') { e.preventDefault(); step(k === 'ArrowDown' ? 1 : -1); }
      else if (k === 'Enter') { e.preventDefault(); go(cur); }
      else if (k === 'Escape' || comboOf(e) === 'Ctrl+k') { e.preventDefault(); close(); }
    });
    el.addEventListener('mousedown', e => { if (e.target === el) close(); else if (e.target !== input) e.preventDefault(); });
    list.addEventListener('click', e => { const r = e.target.closest('[data-i]'); if (r) go(+r.dataset.i); });
    list.addEventListener('mousemove', e => { const r = e.target.closest('[data-i]'); if (r && +r.dataset.i !== cur) { cur = +r.dataset.i; mark(); } });
    draw();
    input.focus();
  }

  CC.action('palette', { label: 'Befehl suchen', g: 'IconBolt', key: 'Strg+K', run: open });

  CC.css(`
.pal-scrim { position: fixed; inset: 0; z-index: 55; }
.pal { position: fixed; top: 4.5rem; left: 50%; translate: -50% 0; width: min(36rem, calc(100vw - 2rem)); max-height: calc(100vh - 6rem); padding: .35rem; display: flex; flex-direction: column; gap: .35rem; }
.pal-q { display: flex; align-items: center; gap: .55rem; flex: none; cursor: text; }
.pal-q:focus-within { background: var(--carbon-hover-raised); }
.pal-q svg { width: 1rem; height: 1rem; flex: none; color: var(--carbon-text-muted); }
.pal-q input { flex: 1; min-width: 0; height: 100%; border: 0; outline: none; background: none; color: inherit; font: inherit; }
.pal-list { display: flex; flex-direction: column; overflow: auto; min-height: 0; max-height: 26rem; }
.pal-list .head { font-size: 12px; letter-spacing: .14em; text-transform: uppercase; color: var(--carbon-text-muted); padding: .45rem .6rem .2rem; flex: none; }
.pal-row { height: 2rem; padding: 0 .6rem; border-radius: var(--radius-control); display: flex; align-items: center; gap: .6rem; white-space: nowrap; flex: none; cursor: pointer; }
.pal-row > svg { width: .95rem; height: .95rem; color: var(--accent-glyph); flex: none; }
.pal-row .ph { width: .95rem; flex: none; }
.pal-row .t { overflow: hidden; text-overflow: ellipsis; }
.pal-row .grp { margin-left: auto; padding-left: .5rem; font-size: 12px; color: var(--carbon-text-muted); }
.pal-row .t + .kbd { margin-left: auto; }
.pal-row.cur { background: var(--carbon-hover); }
.pal-row.off { opacity: .45; cursor: default; }
.pal-none { padding: .8rem .6rem; color: var(--carbon-text-muted); font-size: 13px; }
`);
})();
