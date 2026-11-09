// Shared pieces for the settings and admin tabs: lists, windows with steps, the folder picker and the colour picker.
(() => {
  const T0 = Date.now();
  const now = () => NOW + (Date.now() - T0);

  function ago(t) {
    const d = now() - t, min = 60000, h = 60 * min;
    if (d < 0) {
      const days = Math.ceil(-d / DAY);
      return days <= 1 ? 'morgen' : `noch ${days} Tage`;
    }
    if (d < 2 * min) return 'gerade eben';
    if (d < h) return `vor ${Math.round(d / min)} Min.`;
    if (d < DAY) return `vor ${Math.round(d / h)} Std.`;
    if (d < 2 * DAY) return 'gestern';
    return `vor ${Math.round(d / DAY)} Tagen`;
  }

  // A pref change renders the whole page, so the scroll position and the focused control are carried over by hand.
  function keep(fn) {
    const scroller = () => $('#settings-body') || $('#page');
    const y = scroller()?.scrollTop;
    const a = document.activeElement;
    const key = a?.closest('[data-pref], [data-k-focus]');
    const sel = key ? (key.dataset.pref ? `[data-pref="${key.dataset.pref}"]` : `[data-k-focus="${key.dataset.kFocus}"]`) : null;
    const pressed = a?.dataset?.v;
    fn();
    const p2 = scroller();
    if (p2 && y != null) p2.scrollTop = y;
    if (sel) {
      const el = $(sel);
      const target = pressed && el ? el.querySelector(`[data-v="${pressed}"]`) : el;
      target?.focus({ preventScroll: true });
    }
  }
  const setPref = (k, v) => keep(() => CC.setPref(k, v));
  const redraw = () => keep(() => CC.render());

  // Switches and selectors carrying data-pref save themselves.
  function bindPrefs(host) {
    host.addEventListener('change', e => {
      const el = e.target.closest('[data-pref]');
      if (!el) return;
      const k = el.dataset.pref;
      const v = el.classList.contains('sw') ? swVal(el) : el.classList.contains('seg') ? segVal(el) : el.classList.contains('a-dd') ? el.dataset.val : el.value;
      setPref(k, el.dataset.num ? Number(v) : v);
    });
  }

  const copy = (text, what = 'Kopiert') => {
    try { navigator.clipboard?.writeText(text).catch(() => {}); } catch {}
    CC.toast(what);
  };
  const copyField = (value, attr = '', grow = false) => `<span class="a-ctl ${grow ? 'grow' : ''}"><input class="field w mono" readonly value="${esc(value)}" ${attr}><button class="btn icon" data-copy="${esc(value)}" data-tip="Kopieren">${g('IconCopy')}</button></span>`;
  document.addEventListener('click', e => {
    const c = e.target.closest('[data-copy]');
    if (c) { e.stopPropagation(); copy(c.dataset.copy); }
  });

  const pill = (text, tone = '', attr = '') => `<span class="pill ${tone}" ${attr}>${esc(text)}</span>`;
  const bareBtn = (glyph, tip, attr) => `<button class="btn icon a-bare" data-tip="${esc(tip)}" aria-label="${esc(tip)}" ${attr}>${g(glyph)}</button>`;

  // A list row: glyph tile, the name, badges on the right, and to their left buttons that come in on hover.
  const row = ({ tile, name, tip, badges = [], acts = [], attr = '', mono = false }) => `<div class="a-row" tabindex="0" ${attr}>
      <span class="tile">${tile}</span>
      <span class="a-name ${mono ? 'mono' : ''}"><span class="t">${esc(name)}</span>${tip ? info(tip) : ''}</span>
      <span class="a-badges">${badges.join('')}</span>
      <span class="a-acts">${acts.map(a => a || '<span class="a-slot"></span>').join('')}</span>
    </div>`;
  const list = (rows, empty) => rows.length ? `<div class="a-list">${rows.join('')}</div>` : `<div class="empty">${g(empty[0])}<div>${esc(empty[1])}</div></div>`;
  const foot = (...btns) => `<div class="a-foot">${btns.join('')}</div>`;
  const btn = (label, glyph, attr = '', cls = '') => `<button class="btn ${cls}" ${attr}>${glyph ? g(glyph) : ''}${esc(label)}</button>`;

  // A text control that writes into an object on input, without a save button.
  const field = (value, attr = '', cls = 'w') => `<input class="field ${cls}" value="${esc(value)}" ${attr}>`;
  const pwField = (attr = '', cls = 'w') => `<span class="a-pw"><input type="password" class="field ${cls}" autocomplete="new-password" ${attr}><button class="a-eye" type="button" data-tip="Anzeigen" aria-label="Anzeigen">${g('IconEye')}</button></span>`;
  document.addEventListener('click', e => {
    const eye = e.target.closest('.a-eye');
    if (!eye) return;
    const i = eye.previousElementSibling, show = i.type === 'password';
    i.type = show ? 'text' : 'password';
    eye.innerHTML = g(show ? 'IconEyeOff' : 'IconEye');
    eye.dataset.tip = show ? 'Verbergen' : 'Anzeigen';
  });

  // A dropdown is a field-shaped trigger that opens the shared menu under it. An option may carry a flag instead of a glyph.
  const mark = (o) => (o[3] ? `<span class="fi fi-${o[3]}"></span>` : o[2] ? g(o[2]) : '');
  const dropdown = (opts, cur, attr = '', cls = 'm') => {
    const o = opts.find(x => x[0] === cur) || opts[0];
    return `<button class="field ${cls} a-dd" data-dd="${esc(JSON.stringify(opts))}" data-val="${esc(o[0])}" ${attr}><span>${mark(o)}${esc(o[1])}</span>${g('IconExpand')}</button>`;
  };
  document.addEventListener('click', e => {
    const dd = e.target.closest('.a-dd');
    if (!dd) return;
    e.stopPropagation();
    const opts = JSON.parse(dd.dataset.dd), r = dd.getBoundingClientRect();
    menu(r.left, r.bottom + 6, opts.map((o) => ({ label: o[1], g: o[2], radio: true, checked: () => o[0] === dd.dataset.val, run: () => {
      dd.dataset.val = o[0];
      dd.querySelector('span').innerHTML = mark(o) + esc(o[1]);
      dd.dispatchEvent(new Event('change', { bubbles: true }));
      dd.focus();
    } })));
  });
  const ddVal = (el) => el.dataset.val;

  const shake = (el) => { if (!el) return; el.classList.remove('glim-shake'); void el.offsetWidth; el.classList.add('glim-shake'); };
  const fail = (text, el) => { CC.toast(text, 'warn'); shake(el); return false; };

  // A window whose body and foot can change while it is open, for steps and verdicts.
  function stepWin({ title, wide, cls = '' }) {
    let close, locked = false;
    const el = win({ title, wide, cls, body: '', foot: [], onOpen: (_, c) => { close = c; } });
    const body = $('.win-body', el), ft = $('.win-foot', el);
    el.addEventListener('mousedown', e => { if (locked && e.target === el) e.stopImmediatePropagation(); }, true);
    const esc_ = (e) => { if (locked && e.key === 'Escape' && el.isConnected) e.stopImmediatePropagation(); };
    addEventListener('keydown', esc_, true);
    const w = {
      el,
      close: () => { removeEventListener('keydown', esc_, true); close(); },
      lock: (on) => { locked = on; },
      set(html, buttons) {
        body.innerHTML = html;
        // A start of '' keeps its place, so the buttons stay where they are while a verdict comes and goes. tone colours a
        // button that carries a verdict.
        ft.innerHTML = buttons.map((f, k) => f.start !== undefined ? `<span class="start">${f.start}</span>` : `<button class="btn ${f.primary ? 'primary' : ''} ${f.tone ? 'tone-' + f.tone : ''}" ${f.id ? `id="${f.id}"` : ''} data-sf="${k}">${f.g ? g(f.g) : ''}${esc(f.label)}</button>`).join('');
        $$('[data-sf]', ft).forEach(b => b.addEventListener('click', () => { const f = buttons[+b.dataset.sf]; if (!f.run || f.run(el, b) !== false) w.close(); }));
        wireControls(el);
        setTimeout(() => (body.querySelector('input:not([readonly])') || ft.querySelector('.btn:last-child'))?.focus(), 0);
      },
    };
    return w;
  }

  function confirmWin({ title, text, verb, glyph = 'IconCheck', run }) {
    win({ title, body: `<div class="well a-text">${text}</div>`, foot: [cancel, { label: verb, g: glyph, primary: true, run }] });
  }

  // The folder picker: the server's tree, folders only unless files with one of the given endings are wanted.
  function pickFolder({ title = 'Ordner wählen', start = '/mnt/user', exts = null } = {}) {
    return new Promise(resolve => {
      let chosen = start, done = false;
      const open = new Set(['/mnt']);
      FS.split(start).forEach((_, k, a) => open.add('/' + a.slice(0, k + 1).join('/')));
      const w = stepWin({ title, cls: 'a-picker' });
      const tree = () => {
        const rows = [];
        const walk = (n, path, depth) => {
          for (const k of FS.kids(n)) {
            const p = FS.join(path, k.name);
            if (!k.dir && !(exts && exts.includes(FS.ext(k.name)))) continue;
            const hasKids = k.dir && FS.kids(k).some(c => c.dir || (exts && exts.includes(FS.ext(c.name))));
            const isOpen = open.has(p);
            const glyph = !k.dir ? 'IconFile' : p === '/boot' ? 'IconFlash' : p === '/remotes' ? 'IconCloud' : FS.FREE[p] ? 'IconZFS' : isOpen ? 'IconFolderOpen' : 'IconFolder';
            rows.push(`<div class="nav-row tree-row ${chosen === p ? 'cur' : ''}" style="padding-left:${.3 + depth * .85}rem" data-pick="${esc(p)}" data-isdir="${k.dir ? 1 : ''}">
              ${hasKids ? `<button class="tree-tog ${isOpen ? 'open' : ''}" data-ptog="${esc(p)}" aria-label="${isOpen ? 'Zuklappen' : 'Aufklappen'}">${g('IconExpand')}</button>` : '<span class="tree-tog"></span>'}
              ${g(glyph)}<span>${esc(FS.LABEL[p] || k.name)}</span></div>`);
            if (k.dir && isOpen) walk(k, p, depth + 1);
          }
        };
        walk(ROOT, '/', 0);
        return rows.join('');
      };
      const choose = (el) => {
        const p = $('#a-pick-path', el).value.trim();
        const n = FS.nodeAt(p);
        if (!n || (exts ? n.dir : !n.dir)) return fail(exts ? 'Erst eine Datei wählen' : 'Diesen Ordner gibt es nicht', $('#a-pick-path', el));
        done = true;
        resolve(p);
      };
      w.set(`${frow(exts ? 'Datei' : 'Ordner', `<input class="field w mono" id="a-pick-path" value="${esc(chosen)}">`)}<div class="well a-tree" id="a-tree">${tree()}</div>`,
        [cancel, { label: 'Auswählen', g: 'IconCheck', primary: true, run: choose }]);
      const el = w.el;
      el.addEventListener('click', e => {
        const tog = e.target.closest('[data-ptog]');
        if (tog) { const p = tog.dataset.ptog; open.has(p) ? open.delete(p) : open.add(p); $('#a-tree', el).innerHTML = tree(); return; }
        const r = e.target.closest('[data-pick]');
        if (r) { chosen = r.dataset.pick; $('#a-pick-path', el).value = chosen; $('#a-tree', el).innerHTML = tree(); }
      });
      el.addEventListener('dblclick', e => { const r = e.target.closest('[data-pick]'); if (r && (exts ? !r.dataset.isdir : true) && choose(el) !== false) w.close(); });
      new MutationObserver((_, o) => { if (!el.isConnected) { o.disconnect(); if (!done) resolve(null); } }).observe(document.body, { childList: true });
    });
  }
  // A path field with the picker button beside it.
  const pathField = (value, id, opts = {}) => `<span class="a-ctl"><input class="field w mono" id="${id}" value="${esc(value)}" ${opts.attr || ''}><button class="btn icon" type="button" data-pickfor="${id}" data-exts="${opts.exts ? esc(opts.exts.join(',')) : ''}" data-tip="${opts.exts ? 'Datei wählen' : 'Ordner wählen'}">${g('IconFolderOpen')}</button></span>`;
  document.addEventListener('click', async e => {
    const b = e.target.closest('[data-pickfor]');
    if (!b) return;
    e.stopPropagation();
    const input = document.getElementById(b.dataset.pickfor);
    const exts = b.dataset.exts ? b.dataset.exts.split(',') : null;
    const here = FS.inArchive(Files.T().path) ? '/mnt/user' : Files.T().path;
    const p = await pickFolder({ start: FS.nodeAt(input.value) ? input.value : here, exts, title: exts ? 'Datei wählen' : 'Ordner wählen' });
    if (p) { input.value = p; input.dispatchEvent(new Event('input', { bubbles: true })); input.dispatchEvent(new Event('change', { bubbles: true })); }
  });

  // Five more taps on the option already chosen open a hidden one, like the shape picker's leaf.
  const taps = {};
  const tapCount = (key, v, top) => {
    const t = taps[key] || (taps[key] = { v: null, n: 0 });
    if (v === top && t.v === top) t.n++; else t.n = 0;
    t.v = v;
    return t.n;
  };

  // The colour picker floats beside the swatch it belongs to: a saturation and value square, a hue bar and the hex.
  const hexToHsv = (hex) => {
    const n = parseInt(hex.slice(1), 16), r = (n >> 16 & 255) / 255, gr = (n >> 8 & 255) / 255, b = (n & 255) / 255;
    const mx = Math.max(r, gr, b), mn = Math.min(r, gr, b), d = mx - mn;
    let h = 0;
    if (d) h = mx === r ? ((gr - b) / d) % 6 : mx === gr ? (b - r) / d + 2 : (r - gr) / d + 4;
    return [(h * 60 + 360) % 360, mx ? d / mx : 0, mx];
  };
  const hsvToHex = (h, s, v) => {
    const f = (n) => { const k = (n + h / 60) % 6; return v - v * s * Math.max(0, Math.min(k, 4 - k, 1)); };
    return '#' + [f(5), f(3), f(1)].map(x => Math.round(x * 255).toString(16).padStart(2, '0')).join('').toUpperCase();
  };
  function colorPicker(anchor, hex, onLive, onDone) {
    $('.a-cpop')?.remove();
    let [h, s, v] = hexToHsv(hex);
    const pop = document.createElement('div');
    pop.className = 'a-cpop glim-card';
    pop.innerHTML = `<div class="a-sv"><i></i></div><div class="a-hue"><i></i></div>${frow('Hex', '<input class="field s mono" maxlength="7" spellcheck="false">')}`;
    document.body.appendChild(pop);
    const r = anchor.getBoundingClientRect();
    pop.style.left = Math.min(r.left, innerWidth - pop.offsetWidth - 12) + 'px';
    pop.style.top = (r.bottom + pop.offsetHeight + 16 > innerHeight ? r.top - pop.offsetHeight - 8 : r.bottom + 8) + 'px';
    const sv = $('.a-sv', pop), hue = $('.a-hue', pop), hx = $('input', pop);
    const paint = (fromField) => {
      const cur = hsvToHex(h, s, v);
      sv.style.background = `linear-gradient(to top, #000, transparent), linear-gradient(to right, #fff, hsl(${h} 100% 50%))`;
      $('i', sv).style.cssText = `left:${s * 100}%;top:${(1 - v) * 100}%;background:${cur}`;
      $('i', hue).style.left = `${h / 3.6}%`;
      if (!fromField) hx.value = cur;
      onLive(cur);
    };
    const drag = (el, fn) => el.addEventListener('pointerdown', e => {
      el.setPointerCapture(e.pointerId);
      const move = (ev) => { const b = el.getBoundingClientRect(); fn(Math.max(0, Math.min(1, (ev.clientX - b.left) / b.width)), Math.max(0, Math.min(1, (ev.clientY - b.top) / b.height))); paint(); };
      move(e);
      el.onpointermove = move;
      el.onpointerup = () => { el.onpointermove = null; onDone(hsvToHex(h, s, v)); };
    });
    drag(sv, (x, y) => { s = x; v = 1 - y; });
    drag(hue, (x) => { h = Math.min(359.9, x * 360); });
    hx.addEventListener('input', () => { if (/^#[0-9a-f]{6}$/i.test(hx.value)) { [h, s, v] = hexToHsv(hx.value); paint(true); onDone(hx.value.toUpperCase()); } });
    const away = (e) => {
      if (e.type === 'keydown' && e.key !== 'Escape') return;
      if (e.type === 'mousedown' && (pop.contains(e.target) || anchor.contains(e.target))) return;
      if (e.type === 'keydown') e.preventDefault();
      pop.remove();
      removeEventListener('mousedown', away, true); removeEventListener('keydown', away, true); removeEventListener('resize', away);
    };
    setTimeout(() => { addEventListener('mousedown', away, true); addEventListener('keydown', away, true); addEventListener('resize', away); }, 0);
    paint();
  }

  // Motion is a token set on the root, and the accent's ink follows its brightness.
  const ink = (hex) => {
    const n = parseInt(String(hex).slice(1), 16);
    if (Number.isNaN(n)) return '#161616';
    const lin = (c) => { c /= 255; return c <= .04045 ? c / 12.92 : ((c + .055) / 1.055) ** 2.4; };
    const L = .2126 * lin(n >> 16 & 255) + .7152 * lin(n >> 8 & 255) + .0722 * lin(n & 255);
    return L > .18 ? '#161616' : '#ffffff';
  };
  const baseApply = CC.applyPrefs;
  CC.applyPrefs = () => {
    baseApply();
    const r = document.documentElement;
    r.dataset.motion = CC.prefs.motion;
    r.style.setProperty('--accent-contrast', ink(CC.prefs.accent));
  };

  CC.admin = { now, ago, keep, setPref, redraw, bindPrefs, copy, copyField, pill, bareBtn, row, list, foot, btn, field, pwField, dropdown, ddVal, shake, fail, stepWin, confirmWin, pickFolder, pathField, tapCount, colorPicker, ink };

  CC.css(`
.a-list { display: flex; flex-direction: column; gap: 2px; margin: 0 -.5rem; }
.a-row { display: grid; grid-template-columns: 2.25rem minmax(0, 1fr) auto auto; align-items: center; gap: .6rem; min-height: 2.6rem; padding: 0 .5rem; border-radius: var(--radius-control); cursor: pointer; outline-offset: -2px; }
.a-row:not([data-open], [data-uopen], [data-gopen]) { cursor: default; }
.a-row:hover, .a-row:focus-within { background: var(--carbon-hover); }
.a-row:hover .tile, .a-row:focus-visible .tile { background: var(--accent); }
.a-row:hover .tile svg, .a-row:focus-visible .tile svg { color: var(--accent-contrast); }
.a-row:hover .tile i, .a-row:focus-visible .tile i { color: var(--accent-contrast); }
.a-row .tile i { font-style: normal; font-size: 12px; font-weight: 700; color: var(--carbon-text-sub); }
.a-row .tile svg { color: var(--accent-glyph); }
.a-name { display: flex; align-items: center; gap: .4rem; min-width: 0; white-space: nowrap; }
.a-name .t { overflow: hidden; text-overflow: ellipsis; }
/* A row's values stay at the right edge whether or not it has the pointer, and its buttons come in to their left. */
.a-badges { order: 2; display: flex; gap: .35rem; justify-content: flex-end; align-items: center; flex-wrap: wrap; }
.a-acts { order: 1; display: flex; gap: .15rem; opacity: 0; transition: opacity .1s; }
.a-row:hover .a-acts, .a-row:focus-within .a-acts { opacity: 1; }
@media (hover: none) { .a-acts { opacity: 1; } }
.a-slot { width: var(--btn-h); flex: none; }
.btn.a-bare { background: transparent; box-shadow: none; }
.btn.a-bare:hover, .btn.a-bare:focus-visible { background: transparent; }
.btn.a-bare:hover svg, .btn.a-bare:focus-visible svg { color: var(--accent-glyph); }
.a-foot { display: flex; justify-content: flex-end; flex-wrap: wrap; gap: .5rem; padding-top: .5rem; }
.a-ctl { display: inline-flex; gap: .4rem; align-items: center; flex: none; }
.a-ctl.grow { flex: 1; min-width: 0; }
.a-ctl.grow .field { flex: 1; width: auto; }
.a-text { line-height: 1.55; }
.a-text p { margin: 0; }
.a-text p + p { margin-top: .6rem; }
.well .pill { background: var(--carbon-surface3); }
.well .field { background: var(--carbon-surface3); }
.mono.pill { font-size: 12px; }
.card-body > .frow + .frow { margin-top: .1rem; }
.a-pw { position: relative; display: inline-flex; flex: none; }
.a-pw .field { padding-right: 2.3rem; }
.a-eye { position: absolute; right: .35rem; top: 50%; translate: 0 -50%; width: 1.6rem; height: 1.6rem; display: grid; place-items: center; color: var(--carbon-text-muted); border-radius: var(--radius-control); }
.a-eye svg { width: .95rem; height: .95rem; }
.a-eye:hover { color: var(--carbon-text); }
.a-dd { display: inline-flex; align-items: center; justify-content: space-between; gap: .5rem; text-align: left; cursor: pointer; }
.a-dd span { display: inline-flex; align-items: center; gap: .45rem; overflow: hidden; white-space: nowrap; }
.a-dd svg { width: .8rem; height: .8rem; flex: none; color: var(--carbon-text-sub); }
.a-dd:hover { background: var(--carbon-hover-raised); }
.card-body .a-dd:not(:hover) { background: var(--carbon-surface2); }
.card-body .field:not(.a-dd) { background: var(--carbon-surface2); }
.card-body .field:not(.a-dd):focus { background: var(--carbon-surface3); }
.card-body .well .field:not(.a-dd) { background: var(--carbon-surface3); }
.a-tree { max-height: 22rem; overflow: auto; gap: 2px; padding: .5rem; }
.a-picker .nav-row:hover { background: var(--carbon-surface3); }
.a-picker .nav-row.cur { background: var(--accent); }
.a-cpop { position: fixed; z-index: 75; width: 16rem; padding: .8rem; display: flex; flex-direction: column; gap: .7rem; }
.a-sv { position: relative; height: 9rem; border-radius: var(--radius-control); cursor: crosshair; touch-action: none; }
.a-hue { position: relative; height: .9rem; border-radius: var(--radius-pill); background: linear-gradient(to right, #f00, #ff0, #0f0, #0ff, #00f, #f0f, #f00); cursor: pointer; touch-action: none; }
.a-sv i, .a-hue i { position: absolute; width: 1rem; height: 1rem; border-radius: 50%; translate: -50% -50%; box-shadow: 0 0 0 2px #fff, 0 0 0 3px rgba(0,0,0,.4); pointer-events: none; }
.a-hue i { top: 50%; background: transparent; }
.btn.ok { background: color-mix(in srgb, var(--carbon-surface2), #6fdc8c 40%); }
.btn.bad { background: color-mix(in srgb, var(--carbon-surface2), #ff8389 40%); }
.win .well .seg, .card-body .well .seg { background: var(--carbon-surface3); }
.win .well .seg button:not([aria-pressed="true"]):hover, .card-body .well .seg button:not([aria-pressed="true"]):hover { background: var(--carbon-hover-raised); }
@media (max-width: 760px) {
  .settings-body .frow, .win .frow { flex-wrap: wrap; }
  .settings-body .frow > .seg, .win .frow > .seg { flex: 1 1 100%; overflow-x: auto; scrollbar-width: none; }
  .settings-body .frow > .seg button, .win .frow > .seg button { flex: 1; justify-content: center; padding: 0 .5rem; }
  .settings-body .frow > .a-swatches { flex: 1 1 100%; justify-content: flex-start; }
  .settings-body .field.w, .win .field.w { width: 100%; }
  .settings-body .a-ctl, .win .a-ctl { width: 100%; }
  .settings-body .a-ctl .field, .win .a-ctl .field { flex: 1; width: auto; }
  .a-row { grid-template-columns: 2.25rem minmax(0, 1fr) auto; }
  .a-badges { display: none; }
}
`);
})();
