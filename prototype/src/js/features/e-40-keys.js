// Tastenkürzel selbst belegen: the F1 window and the settings tile let every action take the key you press.
(() => {
  const { seg, segVal, frow, cancel } = CC.ui;
  const KEY = 'cannoncommander-keys';
  const NAMED = { ArrowLeft: 'Links', ArrowRight: 'Rechts', ArrowUp: 'Hoch', ArrowDown: 'Runter', Delete: 'Entf', Insert: 'Einfg', Escape: 'Esc', ' ': 'Leertaste', Home: 'Pos1', End: 'Ende', PageUp: 'Bild auf', PageDown: 'Bild ab', Backspace: 'Rück' };
  // A combo ending in the plus key itself, such as Ctrl++, keeps that plus as its last part.
  // A key of a scope, such as the editor's, carries the scope in front, editor:Ctrl+d, and reads without it.
  const label = (scoped) => {
    const combo = scoped.replace(/^[a-z]+:(?=.)/, '');
    const parts = combo.endsWith('++') ? [...combo.slice(0, -2).split('+'), '+'] : combo.split('+');
    return parts.map((p, i) => i < parts.length - 1 ? { Ctrl: 'Strg', Shift: 'Umschalt', Alt: 'Alt' }[p] || p : NAMED[p] || (p.length === 1 ? p.toUpperCase() : p)).join('+');
  };
  const nameOf = (a) => typeof a.label === 'function' ? a.label() : a.label;
  const MODIFIERS = ['Control', 'Shift', 'Alt', 'Meta', 'AltGraph', 'CapsLock'];

  let DEFAULT_MAP, DEFAULT_LABEL, saved = null;
  const load = () => { try { const v = JSON.parse(localStorage.getItem(KEY)); return v && typeof v.map === 'object' ? v : null; } catch { return null; } };
  const store = (v) => { try { v ? localStorage.setItem(KEY, JSON.stringify(v)) : localStorage.removeItem(KEY); } catch {} };
  const combosOf = (map, id) => Object.keys(map).filter(c => map[c] === id);

  // The label in menus, tooltips and the command search follows the first key the action has.
  function apply(map) {
    for (const c of Object.keys(CC.keymap)) delete CC.keymap[c];
    Object.assign(CC.keymap, map);
    for (const a of Object.values(CC.actions)) {
      const own = combosOf(map, a.id), keep = own.find(c => label(c) === DEFAULT_LABEL[a.id]) || own[0];
      if (keep) a.key = label(keep); else delete a.key;
    }
  }
  // Bound after every feature has added its keys, so the defaults are the whole set.
  queueMicrotask(() => {
    DEFAULT_MAP = { ...CC.keymap };
    DEFAULT_LABEL = Object.fromEntries(Object.values(CC.actions).map(a => [a.id, a.key]));
    saved = load();
    if (!saved) return;
    apply(saved.map);
    CC.render?.();
  });

  // A tooltip names its key in brackets; with a key of your own it names yours.
  CC.tipOf = (el) => {
    const tip = el.dataset.tip, id = el.dataset.run, was = id && DEFAULT_LABEL?.[id], now = id && CC.actions[id]?.key;
    if (!was || was === now) return tip;
    return now ? tip.split(was).join(now) : tip.replace(` (${was})`, '');
  };

  const SCOPES = { editor: 'Editor' };
  function groups() {
    const seen = new Set(), out = [];
    const ids = (items) => items.flatMap(it => typeof it === 'string' ? [it] : it?.sub ? ids(it.sub) : []).filter(id => CC.actions[id] && !seen.has(id) && seen.add(id));
    for (const [menuName, items] of MENUS) out.push([menuName, ids(items)]);
    out.push(['Weitere', Object.keys(CC.actions).filter(id => !seen.has(id) && !CC.actions[id].scope && nameOf(CC.actions[id]) && (combosOf(DEFAULT_MAP, id).length || DEFAULT_LABEL[id]))]);
    // Each scope gets a group of its own, as its keys only count there.
    const scoped = Object.values(CC.actions).filter(a => a.scope);
    for (const scope of new Set(scoped.map(a => a.scope))) out.push([SCOPES[scope] || scope, scoped.filter(a => a.scope === scope).map(a => a.id)]);
    return out.filter(([, list]) => list.length);
  }

  const FIXED = [['Tab', 'Fenster wechseln'], ['Enter', 'Öffnen'], ['Einfg', 'Markieren und weiter'], ['Leertaste', 'Schnellansicht'], ['F10', 'Menüleiste'], ['Esc', 'Schließen']];
  const fixedGrid = () => `<div class="kbd-grid">${FIXED.map(([k, l]) => `<div><span>${esc(l)}</span><span class="kbd">${esc(k)}</span></div>`).join('')}</div>`;
  const hits = (list, map, query) => {
    const q = query.trim().toLowerCase();
    return list.filter(id => !q || nameOf(CC.actions[id]).toLowerCase().includes(q) || combosOf(map, id).some(c => label(c).toLowerCase().includes(q)));
  };
  // The layout's keys of an action that another action has taken.
  const heldBack = (map, base, id) => combosOf(base, id).filter(c => map[c] && map[c] !== id);
  // The action takes the key, and with swap the action that had it gets the keys this one had.
  const takeKey = (map, combo, id, swapped) => {
    const next = { ...map }, had = combosOf(next, id), other = next[combo];
    for (const k of had) delete next[k];
    next[combo] = id;
    if (swapped && other) for (const k of had) next[k] = other;
    return next;
  };
  // A reset that takes the layout's keys back. With swap the action that held them gets the keys this one had instead.
  const restore = (map, base, id, swapped) => {
    const holder = map[heldBack(map, base, id)[0]], had = combosOf(map, id), next = { ...map };
    for (const k of had) delete next[k];
    for (const k of combosOf(base, id)) next[k] = id;
    if (swapped && holder) for (const k of had) if (!next[k]) next[k] = holder;
    return next;
  };
  const clash = (map, id, combo, reset = false) => {
    const name = esc(nameOf(CC.actions[map[combo]]));
    return { id, combo, reset, choice: true, msg: combosOf(map, id).length
      ? `${esc(label(combo))} gehört schon zu „${name}“. Beim Tauschen bekommt „${name}“ deine bisherige Taste. Esc lässt alles, wie es ist.`
      : `${esc(label(combo))} gehört schon zu „${name}“. Ersetzen holt die Taste hierher, Esc lässt alles, wie es ist.` };
  };

  // A row of the window or the tile. A clash offers the buttons only where the row asks for them.
  const row = (id, map, base, rec, pending) => {
    const a = CC.actions[id], own = combosOf(map, id), main = own.find(c => label(c) === DEFAULT_LABEL[id]) || own[0];
    const changed = combosOf(base, id).sort().join() !== own.sort().join();
    const recording = rec === id;
    return `<div class="kb-row ${recording ? 'rec' : ''} ${changed ? 'own' : ''}" data-kb="${id}">
      <span class="kb-name">${a.g ? g(a.g) : '<span class="ph"></span>'}${esc(nameOf(a))}</span>
      <button class="kb-key" data-kb-rec="${id}" aria-label="Taste für ${esc(nameOf(a))} ändern">${recording ? '<span class="muted">Taste drücken …</span>' : main ? `<span class="kbd">${esc(label(main))}</span>${own.length > 1 ? `<span class="muted">+${own.length - 1}</span>` : ''}` : '<span class="muted">keine</span>'}</button>
      <span class="kb-acts">${main ? `<button class="btn icon" data-kb-clear="${id}" data-tip="Taste löschen" aria-label="Taste löschen">${g('IconTrash')}</button>` : ''}${changed ? `<button class="btn icon" data-kb-reset="${id}" data-tip="Wie vorher" aria-label="Wie vorher">${g('IconUndo')}</button>` : ''}</span>
      ${pending?.id === id ? `<div class="kb-warn"><span>${pending.msg}</span>${pending.choice ? `<span class="kb-choice">${own.length ? `<button class="btn" id="kb-swap">${g('IconSync')}Tauschen</button>` : ''}<button class="btn" id="kb-replace">${g('IconCheck')}Ersetzen</button></span>` : ''}</div>` : ''}</div>`;
  };

  function keysWindow() {
    let map = { ...CC.keymap }, rec = null, pending = null, query = '';
    const w = win({
      title: 'Tastenkürzel', wide: true, cls: 'kb-win',
      body: `<div class="well">${frow('Befehl suchen', '<input class="field m" id="kb-q" placeholder="etwa Kopieren" autocomplete="off">')}</div>
        <div class="well kb-well"><div class="kb-list" id="kb-list"></div>
          <div class="eyebrow kb-fixedhead">Fest belegt</div>${fixedGrid()}</div>`,
      foot: [{ start: 'Klick auf eine Taste und drück die neue Kombination' }, { label: 'Alles zurücksetzen', g: 'IconRefresh', run: () => { map = { ...DEFAULT_MAP }; draw(); return false; } },
        cancel, { label: 'Übernehmen', g: 'IconCheck', primary: true, run: () => {
          apply(map);
          const own = JSON.stringify(map) !== JSON.stringify(DEFAULT_MAP);
          saved = own ? { map } : null;
          store(saved);
          CC.render();
          CC.toast(own ? 'Deine Tastenkürzel gelten jetzt überall' : 'Die Tastenkürzel sind wieder wie ausgeliefert');
        } }],
      onClose: () => document.removeEventListener('keydown', key, true),
    });
    const draw = () => {
      $('#kb-list', w).innerHTML = groups().map(([menuName, list]) => {
        const found = hits(list, map, query);
        return found.length ? `<section class="kb-group"><div class="eyebrow">${esc(menuName)}</div>${found.map(id => row(id, map, DEFAULT_MAP, rec, pending)).join('')}</section>` : '';
      }).join('') || `<div class="empty">${g('IconSearch')}<div>Kein Befehl heißt so</div></div>`;
    };
    const stop = () => { rec = null; pending = null; draw(); };
    // While a row listens, every key belongs to it; Escape alone gives up.
    function key(e) {
      if (!rec) return;
      e.preventDefault();
      e.stopPropagation();
      if (MODIFIERS.includes(e.key)) return;
      if (e.key === 'Escape' && !e.ctrlKey && !e.altKey && !e.shiftKey) return stop();
      const combo = comboOf(e);
      if (['Tab', 'Enter', 'F10'].includes(combo)) { pending = { id: rec, msg: `${esc(label(combo))} ist fest belegt. Nimm eine andere Taste.` }; return draw(); }
      if (/^[\p{L}\d]$/u.test(combo)) { pending = { id: rec, msg: 'Einzelne Buchstaben und Ziffern braucht die Schnellsuche. Nimm Strg, Alt oder eine F-Taste dazu.' }; return draw(); }
      const other = map[combo];
      if (other && other !== rec && pending?.combo !== combo) {
        pending = clash(map, rec, combo);
        return draw();
      }
      for (const c of combosOf(map, rec)) delete map[c];
      map[combo] = rec;
      stop();
    }
    // Redrawing the list takes the focus away, so the keys are caught for the whole page while the window is open.
    document.addEventListener('keydown', key, true);
    w.addEventListener('click', e => {
      const r = e.target.closest('[data-kb-rec]'), c = e.target.closest('[data-kb-clear]'), z = e.target.closest('[data-kb-reset]');
      if (e.target.closest('#kb-swap, #kb-replace')) {
        const swapped = !!e.target.closest('#kb-swap');
        map = pending.reset ? restore(map, DEFAULT_MAP, pending.id, swapped) : takeKey(map, pending.combo, pending.id, swapped);
        return stop();
      }
      if (r) { rec = rec === r.dataset.kbRec ? null : r.dataset.kbRec; pending = null; draw(); if (rec) $(`[data-kb-rec="${rec}"]`, w)?.focus(); return; }
      if (c) { for (const k of combosOf(map, c.dataset.kbClear)) delete map[k]; return stop(); }
      if (z) {
        // A reset whose key another action has taken asks first, as a clash does.
        const id = z.dataset.kbReset, held = heldBack(map, DEFAULT_MAP, id)[0];
        if (held) { rec = null; pending = clash(map, id, held, true); return draw(); }
        for (const k of combosOf(map, id)) delete map[k];
        for (const k of combosOf(DEFAULT_MAP, id)) if (!map[k]) map[k] = id;
        return stop();
      }
    });
    $('#kb-q', w).addEventListener('input', e => { query = e.target.value; draw(); });
    draw();
  }
  CC.action('shortcuts', { ...CC.actions.shortcuts, run: keysWindow });

  // The settings tile: every key, changed at once, and a reset that drops the own keys.
  const tab = { rec: null, pending: null, query: '' };
  const onTab = () => CC.view === 'settings' && CC.settingsTab_ === 'keys';
  const BROWSER = ['Ctrl+n', 'Ctrl+Shift+n', 'Ctrl+t', 'Ctrl+Shift+t', 'Ctrl+w', 'Ctrl+Shift+w', 'Ctrl+Tab', 'Ctrl+Shift+Tab', 'Ctrl+PageUp', 'Ctrl+PageDown', 'Ctrl+Shift+p'];
  const ownOf = (base, map) => Object.fromEntries([...new Set([...Object.keys(base), ...Object.keys(map)])].filter(c => base[c] !== map[c]).map(c => [c, map[c] || '']));
  function commit(map) {
    apply(map);
    saved = Object.keys(ownOf(DEFAULT_MAP, map)).length ? { map } : null;
    store(saved);
    tab.rec = null;
    tab.pending = null;
    CC.render();
  }
  function tabKey(e) {
    if (!tab.rec || !onTab()) return;
    e.preventDefault();
    e.stopPropagation();
    if (MODIFIERS.includes(e.key)) return;
    if (e.key === 'Escape' && !e.ctrlKey && !e.altKey && !e.shiftKey) { tab.rec = null; tab.pending = null; return CC.render(); }
    const id = tab.rec, scope = CC.actions[id]?.scope, bare = comboOf(e), combo = scope ? `${scope}:${bare}` : bare;
    const map = { ...CC.keymap }, key = esc(label(combo));
    const say = (msg) => { tab.pending = { id, msg }; CC.render(); };
    if (['Tab', 'Enter', 'F10'].includes(bare)) return say(`${key} ist fest belegt. Nimm eine andere Taste.`);
    if (/^[\p{L}\d]$/u.test(bare)) return say('Einzelne Buchstaben und Ziffern braucht die Schnellsuche. Nimm Strg, Alt oder eine F-Taste dazu.');
    if (BROWSER.includes(bare)) return say(`${key} behält der Browser für seine Tabs und Fenster. Nimm hier bitte eine andere, in der Desktop-App klappt sie.`);
    const other = map[combo];
    if (other && other !== id && tab.pending?.combo !== combo) { tab.pending = clash(map, id, combo); return CC.render(); }
    for (const c of combosOf(map, id)) delete map[c];
    map[combo] = id;
    commit(map);
  }
  // On the window, so it runs before the shell's own keys on the document.
  addEventListener('keydown', tabKey, true);

  CC.settingsTab({ id: 'keys', label: 'Tastenkürzel', g: 'IconKeyboard', order: 44, render(host) {
    const base = DEFAULT_MAP, map = CC.keymap;
    const cards = groups().map(([menuName, list]) => {
      const found = hits(list, map, tab.query);
      return found.length ? CC.card(menuName, found.map(id => row(id, map, base, tab.rec, tab.pending)).join('')) : '';
    }).join('') || CC.card('Tastenkürzel', `<div class="empty">${g('IconSearch')}<div>Kein Befehl heißt so</div></div>`);
    host.innerHTML = CC.card('Bedienung',
      frow('Vim-Tasten im Editor', sw(CC.prefs.editorVim, 'id="kt-vim"'), 'Der Editor arbeitet dann wie Vim, mit Normal- und Einfügemodus. Mit :w speicherst du, mit :q schließt du die Datei.')
      + frow('Eigene Tasten', `<button class="btn" id="kt-reset" ${Object.keys(ownOf(base, map)).length ? '' : 'disabled'}>${g('IconRefresh')}Zurücksetzen</button>`, 'Nimmt deine eigenen Tasten zurück.')
      + frow('Befehl suchen', `<input class="field m" id="kt-q" placeholder="etwa Kopieren" autocomplete="off" value="${esc(tab.query)}">`))
      + cards + CC.card('Fest belegt', fixedGrid());
    $('#kt-vim', host).addEventListener('change', e => CC.setPref('editorVim', swVal(e.target)));
    // Every change draws the tile anew, so the search field gets its focus and caret back.
    $('#kt-q', host).addEventListener('input', e => {
      tab.query = e.target.value;
      CC.render();
      const q = $('#kt-q');
      q?.focus();
      q?.setSelectionRange(tab.query.length, tab.query.length);
    });
    host.addEventListener('click', e => {
      const r = e.target.closest('[data-kb-rec]'), c = e.target.closest('[data-kb-clear]'), z = e.target.closest('[data-kb-reset]');
      const m = { ...CC.keymap };
      if (e.target.closest('#kt-reset')) return commit({ ...base });
      if (e.target.closest('#kb-swap, #kb-replace')) {
        const { id, combo, reset } = tab.pending, swapped = !!e.target.closest('#kb-swap');
        return commit(reset ? restore(m, base, id, swapped) : takeKey(m, combo, id, swapped));
      }
      if (r) {
        tab.rec = tab.rec === r.dataset.kbRec ? null : r.dataset.kbRec;
        tab.pending = null;
        CC.render();
        if (tab.rec) $(`[data-kb-rec="${tab.rec}"]`)?.focus();
        return;
      }
      if (c) { for (const k of combosOf(m, c.dataset.kbClear)) delete m[k]; return commit(m); }
      if (z) {
        const id = z.dataset.kbReset, held = heldBack(m, base, id)[0];
        if (held) { tab.rec = null; tab.pending = clash(m, id, held, true); return CC.render(); }
        for (const k of combosOf(m, id)) delete m[k];
        for (const k of combosOf(base, id)) if (!m[k]) m[k] = id;
        return commit(m);
      }
    });
  } });

  CC.css(`
.kb-win { height: min(48rem, calc(100vh - 3rem)); }
.kb-well { flex: 1; min-height: 0; overflow: auto; }
.kb-list { display: flex; flex-direction: column; gap: 1rem; }
.kb-group { display: flex; flex-direction: column; gap: 1px; }
.kb-row { display: grid; grid-template-columns: minmax(0, 1fr) 11rem 4rem; align-items: center; gap: .6rem; min-height: 2.2rem; padding: 0 .3rem 0 .5rem; border-radius: var(--radius-control); }
.kb-row:hover { background: var(--carbon-surface3); }
.kb-row.rec { background: color-mix(in srgb, var(--accent) 14%, var(--carbon-surface2)); }
.kb-name { display: flex; align-items: center; gap: .6rem; min-width: 0; font-size: 14px; }
.kb-name svg { width: .95rem; height: .95rem; color: var(--accent-glyph); flex: none; }
.kb-name .ph { width: .95rem; flex: none; }
.kb-key { justify-self: end; display: inline-flex; align-items: center; gap: .35rem; height: 1.8rem; padding: 0 .55rem; border-radius: var(--radius-control); font-size: 13px; }
.kb-key:hover, .kb-row.rec .kb-key { background: var(--carbon-surface2); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--accent) 55%, transparent); }
.kb-row.own .kb-key .kbd { background: color-mix(in srgb, var(--accent) 22%, var(--carbon-surface3)); }
.kb-acts { display: flex; justify-content: flex-end; gap: 2px; }
.kb-acts .btn { width: 1.8rem; height: 1.8rem; background: transparent; }
.kb-warn { grid-column: 1 / -1; display: flex; flex-wrap: wrap; align-items: center; gap: .4rem .8rem; margin: 0 0 .4rem; padding: .35rem .6rem; border-radius: var(--radius-control); background: color-mix(in srgb, var(--status-warn-solid, #FF832B) 16%, transparent); font-size: 12.5px; }
.kb-choice { display: flex; gap: .4rem; margin-left: auto; }
.kb-fixedhead { margin-top: 1.2rem; }
@media (max-width: 760px) { .kb-row { grid-template-columns: minmax(0, 1fr) auto auto; } }
`);
})();
