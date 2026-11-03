// The shared core every module builds on: glyphs, helpers, the registries and the window kit.
const g = (name) => {
  const d = GLYPHS[name] || GLYPHS.IconFile;
  // Some glyphs leave their paths unfilled and rely on the svg's own fill, as GlimStone's components set it.
  return `<svg viewBox="${d.viewBox}" fill="currentColor" aria-hidden="true">${d.svg}</svg>`;
};
const $ = (s, r = document) => r.querySelector(s);
const $$ = (s, r = document) => [...r.querySelectorAll(s)];
const esc = (s) => String(s ?? '').replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
const KB = 1024, MB = KB * 1024, GB = MB * 1024, TB = GB * 1024;
function fmtSize(b) {
  if (b == null) return '';
  for (const [n, v] of [['TB', TB], ['GB', GB], ['MB', MB], ['KB', KB]]) {
    if (b >= v) return (b / v).toLocaleString('de-AT', { maximumFractionDigits: b / v < 10 ? 1 : 0 }) + ' ' + n;
  }
  return b + ' B';
}
const fmtDate = (t) => new Date(t).toLocaleDateString('de-AT', { day: '2-digit', month: '2-digit', year: 'numeric' });
const fmtTime = (t) => new Date(t).toLocaleString('de-AT', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' });
function fmtAgo(t) {
  const days = Math.floor((NOW - t) / DAY);
  if (days <= 0) return 'heute';
  if (days === 1) return 'gestern';
  if (days < 30) return `vor ${days} Tagen`;
  if (days < 365) return `vor ${Math.round(days / 30)} Mon.`;
  return `vor ${Math.round(days / 365)} J.`;
}
// The list shows dates the way the Dateiliste settings ask for; every other place keeps the plain date.
const fmtListDate = (t) => ({ long: fmtTime, relative: fmtAgo }[CC.prefs.dateFormat] || fmtDate)(t);
const NOW = Date.UTC(2026, 10, 2, 9, 30);
const DAY = 86400000;

// A seeded random source keeps the made-up data the same on every load.
let seed = 7;
const rnd = () => (seed = (seed * 16807) % 2147483647) / 2147483647;

const CC = {
  views: {},
  actions: {},
  settingsTabs: [],
  prefs: {
    theme: 'dark', accent: '#FCC419', shape: 'round', motion: 'subtle', quietToasts: false,
    labelsButtons: 'textGlyph', labelsSidebar: 'textGlyph', labelsTabs: 'textGlyph', labelsBottombar: 'textGlyph',
    rainbow: false, reactive: false, rotate: false, tabOrder: [],
    columns: ['name', 'size', 'modified'], colWidths: {},
    density: 'medium', showHidden: false, showExt: true, syncBrowse: false, folderSizes: true, foldersFirst: true, dblclick: 'open', fkeys: false, side: true, dual: true,
    quickview: false, confirmDelete: true, trash: true, trashDays: 30, language: 'de', dateFormat: 'short', startFolder: '/mnt/user/downloads',
  },
  view: 'files',
  user: { name: 'jdp', full: 'Junker der Provinz', role: 'Admin' },
  signedIn: true,
};

CC.css = (text) => { const s = document.createElement('style'); s.textContent = text; document.head.appendChild(s); };

// An action is anything the menu bar, the context menu, a key or a button can run.
// { id, label, g, key, menu, order, group, ctx, needs: 'sel' | 'file' | 'dual' | 'writable', run, when, checked }
CC.action = (id, def) => { CC.actions[id] = { id, ...def }; };
CC.run = (id, ...args) => {
  const a = CC.actions[id];
  if (!a) return CC.toast(`Noch nicht gebaut: ${id}`, 'warn');
  const why = CC.blocked(a);
  if (why) return CC.toast(why, 'warn');
  return a.run(...args);
};
CC.blocked = (a) => {
  const need = [].concat(a.needs || []);
  if (need.includes('sel') && !Files.sel().length) return 'Erst eine Datei oder einen Ordner wählen';
  if (need.length && !['copy', 'move', 'delete'].includes(a.id) && Files.sel().some(k => k.at && k.at !== Files.T().path)) return 'Das geht nur im eigenen Ordner: mit Enter zur Datei springen';
  if (need.includes('file') && !Files.sel().some(k => !k.dir)) return 'Erst eine Datei wählen';
  if (need.includes('dual') && !CC.prefs.dual) return 'Dafür beide Fenster einschalten (Strg+2)';
  if (need.includes('writable') && FS.inArchive(Files.T().path)) return 'Im Archiv geht nur Ansehen und Entpacken';
  return null;
};

// A view fills the main area: 'files' is the file manager, others are full pages.
CC.registerView = (id, def) => { CC.views[id] = def; };
CC.go = (id, arg) => { CC.view = id; CC.viewArg = arg; closeMenu(); CC.render(); };

// A settings tab: { id, label, g, order, render(host) }.
CC.settingsTab = (def) => { CC.settingsTabs.push(def); CC.settingsTabs.sort((a, b) => a.order - b.order); };

CC.setPref = (k, v) => {
  CC.prefs[k] = v;
  try { localStorage.setItem('cannoncommander-prefs', JSON.stringify(CC.prefs)); } catch {}
  CC.applyPrefs();
  CC.render();
};
CC.applyPrefs = () => {
  const r = document.documentElement;
  r.dataset.theme = CC.prefs.theme === 'system' ? (matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark') : CC.prefs.theme;
  r.style.setProperty('--accent', CC.prefs.accent);
  r.dataset.shape = CC.prefs.shape;
  r.dataset.density = CC.prefs.density;
  r.dataset.lButtons = CC.prefs.labelsButtons;
  r.dataset.lSidebar = CC.prefs.labelsSidebar;
  r.dataset.lTabs = CC.prefs.labelsTabs;
  r.dataset.lBottombar = CC.prefs.labelsBottombar;
  r.toggleAttribute('data-rainbow', CC.prefs.rainbow);
  r.toggleAttribute('data-reactive', CC.prefs.reactive);
  r.toggleAttribute('data-rotate', CC.prefs.rainbow && CC.prefs.rotate);
};
// A stored single colour mode or label mode maps onto the split prefs.
try {
  const saved = JSON.parse(localStorage.getItem('cannoncommander-prefs') || '{}');
  if (saved.colorMode) { saved.rainbow = saved.colorMode === 'rainbow'; saved.reactive = saved.colorMode === 'reactive'; delete saved.colorMode; }
  if (saved.labels) { for (const a of ['Buttons', 'Sidebar', 'Tabs', 'Bottombar']) saved['labels' + a] ??= saved.labels; delete saved.labels; }
  Object.assign(CC.prefs, saved);
} catch {}

// Windows: one look for every dialog, the answer buttons at the end of the foot row.
let winStack = [];
const WAY_OUT = /^(Abbrechen|Schließen|Fertig)$/;
function win({ title, body, foot = [], wide = false, full = false, onOpen, onClose, cls = '' }) {
  closeMenu();
  // Every window can be left from its foot row: one without Abbrechen or Schließen gets one before its other buttons.
  if (foot.length && !foot.some(f => f.label && WAY_OUT.test(f.label))) {
    const at = foot.findIndex(f => !f.start);
    foot = [...foot];
    foot.splice(at < 0 ? foot.length : at, 0, foot.some(f => f.primary) ? cancel : closeBtn);
  }
  const scrim = document.createElement('div');
  scrim.className = 'scrim glim-modal-backdrop';
  scrim.innerHTML = `<div class="win glim-card ${wide ? 'wide' : ''} ${full ? 'full' : ''} ${cls}" role="dialog" aria-label="${esc(title)}"><span class="notch">${esc(title)}</span>
    <div class="win-body">${body}</div><div class="win-foot">${foot.map((f, k) => f.start ? `<span class="start">${f.start}</span>` : `<button class="btn ${f.primary ? 'primary' : ''}" data-f="${k}">${f.g ? g(f.g) : ''}${esc(f.label)}</button>`).join('')}</div></div>`;
  document.body.appendChild(scrim);
  const close = () => { scrim.remove(); winStack = winStack.filter(w => w.close !== close); onClose && onClose(); $('.panel.active .list')?.focus(); };
  winStack.push({ close, el: scrim });
  // A click beside an open dropdown only closes the dropdown, not the window under it.
  scrim.addEventListener('mousedown', e => { if (e.target === scrim && !$('#menu')) close(); });
  scrim.querySelectorAll('[data-f]').forEach(b => b.addEventListener('click', () => { const f = foot[+b.dataset.f]; if (!f.run || f.run(scrim) !== false) close(); }));
  wireControls(scrim);
  onOpen && onOpen(scrim, close);
  setTimeout(() => (scrim.querySelector('input:not([type=file]), textarea') || scrim.querySelector('.win-foot .btn:last-child'))?.focus(), 0);
  return scrim;
}
CC.win = win;
CC.closeWin = () => winStack.at(-1)?.close();
const cancel = { label: 'Abbrechen', g: 'IconClose' };
const closeBtn = { label: 'Schließen', g: 'IconClose' };

// Small controls that every window and settings card uses.
const sw = (on, attr = '') => `<button class="sw" role="switch" aria-checked="${!!on}" ${attr}></button>`;
const info = (t) => `<span class="info" data-tip="${esc(t)}">${g('IconInfo')}</span>`;
const seg = (opts, cur, attr = '') => `<div class="seg" ${attr}>${opts.map(o => {
  const [v, l, gl] = Array.isArray(o) ? o : [o, o];
  return `<button aria-pressed="${v === cur}" data-v="${esc(v)}">${gl ? g(gl) : ''}${esc(l)}</button>`;
}).join('')}</div>`;
const frow = (label, control, tip) => `<div class="frow"><span class="lab">${label}${tip ? ' ' + info(tip) : ''}</span>${control}</div>`;
const segVal = (el) => el.querySelector('[aria-pressed="true"]')?.dataset.v;
const swVal = (el) => el.getAttribute('aria-checked') === 'true';
function wireControls(root) {
  root.querySelectorAll('.sw:not([data-wired])').forEach(s => { s.dataset.wired = 1; s.addEventListener('click', () => { s.setAttribute('aria-checked', String(!swVal(s))); s.dispatchEvent(new Event('change', { bubbles: true })); }); });
  root.querySelectorAll('.seg:not([data-wired])').forEach(sg => { sg.dataset.wired = 1; sg.addEventListener('click', e => { const b = e.target.closest('button'); if (!b) return; sg.querySelectorAll('button').forEach(x => x.setAttribute('aria-pressed', String(x === b))); sg.dispatchEvent(new Event('change', { bubbles: true })); }); });
}
Object.assign(CC, { ui: { sw, info, seg, frow, segVal, swVal, wireControls, cancel, closeBtn } });

function toast(text, kind = 'ok', act) {
  // Quiet notifications drop routine confirmations; failures and anything that offers a step still show.
  if (CC.prefs.quietToasts && kind === 'ok' && !act) return;
  const el = document.createElement('div');
  el.className = `toast glim-card ${kind}`;
  el.innerHTML = `<span class="dot"></span><span>${esc(text)}</span>${act ? `<button class="btn">${esc(act.label)}</button>` : ''}`;
  if (act) el.querySelector('.btn').onclick = () => { act.run(); el.remove(); };
  $('#toasts').appendChild(el);
  let left = 4000, started = Date.now(), timer;
  const arm = () => { started = Date.now(); timer = setTimeout(() => el.remove(), left); };
  el.addEventListener('mouseenter', () => { clearTimeout(timer); left -= Date.now() - started; });
  el.addEventListener('mouseleave', arm);
  arm();
}
CC.toast = toast;

// Menus: items are action ids, '-' for a separator, {head} for a heading, a plain {label, g, run}
// or {label, g, sub: [...]}, which opens its items to the side.
const resolveItems = (items) => items.map(it => typeof it === 'string' && it !== '-' ? CC.actions[it] : it).filter(Boolean)
  .filter((it, k, a) => it !== '-' || (k > 0 && k < a.length - 1 && a[k - 1] !== '-' && !a[k - 1].head));
const menuHTML = (list) => list.map((it, k) => {
  if (it === '-') return '<div class="sep"></div>';
  if (it.head) return `<div class="head">${esc(it.head)}${it.run ? `<button class="hedit" data-k="${k}" data-tip="${esc(it.tip)}" aria-label="${esc(it.tip)}">${g(it.g)}</button>` : ''}</div>`;
  const off = it.id && CC.actions[it.id] && (CC.blocked(it) || (it.when && !it.when()));
  // A switch shows a checkbox where its glyph would stand, as the column menu does; a choice among several keeps its glyph and ticks the current one at the end.
  const on = it.checked && it.checked();
  const lead = it.checked && !it.radio ? `<span class="cbox ${on ? 'on' : ''}" role="presentation">${g('IconCheck')}</span>` : it.color ? `<i class="cdot big" style="--c:${it.color}"></i>` : it.g ? g(it.g) : '<span class="ph"></span>';
  const role = it.checked ? `role="${it.radio ? 'menuitemradio' : 'menuitemcheckbox'}" aria-checked="${!!on}"` : it.sub ? 'role="menuitem" aria-haspopup="menu" data-sub' : 'role="menuitem"';
  const end = it.sub ? `<span class="chev">${g('IconExpand')}</span>` : it.radio && on ? `<span class="tick">${g('IconCheck')}</span>` : it.key ? `<span class="kbd">${esc(it.key)}</span>` : it.count != null ? `<span class="cnt glim-num">${it.count}</span>` : '';
  return `<button data-k="${k}" class="${it.radio && on ? 'on' : ''}" ${off ? 'disabled' : ''} ${role}>${lead}<span>${esc(typeof it.label === 'function' ? it.label() : it.label)}</span>${end}</button>`;
}).join('');

function menu(x, y, items, { above = false, anchor = null } = {}) {
  closeMenu();
  $$('.bubble').forEach(b => b.remove());
  const m = document.createElement('div');
  m.className = 'menu glim-card';
  m.id = 'menu';
  m.setAttribute('role', 'menu');
  const resolved = resolveItems(items);
  m.innerHTML = menuHTML(resolved);
  document.body.appendChild(m);
  const r = m.getBoundingClientRect();
  m.style.left = Math.max(8, Math.min(x, innerWidth - r.width - 8)) + 'px';
  m.style.top = (above ? Math.max(8, y - r.height - 8) : Math.min(y, innerHeight - r.height - 8)) + 'px';

  // The side menu lives on body, not inside the menu, so the menu's scrolling cannot clip it.
  let sub = null;
  const closeSub = () => { sub?.remove(); sub = null; $$('[data-sub]', m).forEach(b => b.classList.remove('open')); };
  const openSub = (host, focus) => {
    closeSub();
    const list = resolveItems(resolved[+host.dataset.k].sub);
    sub = document.createElement('div');
    sub.className = 'menu glim-card';
    sub.id = 'submenu';
    sub.setAttribute('role', 'menu');
    sub.innerHTML = menuHTML(list);
    document.body.appendChild(sub);
    host.classList.add('open');
    const hr = host.getBoundingClientRect(), sr = sub.getBoundingClientRect();
    sub.style.left = (hr.right + sr.width + 6 > innerWidth ? hr.left - sr.width - 6 : hr.right + 6) + 'px';
    sub.style.top = Math.max(8, Math.min(hr.top - 6, innerHeight - sr.height - 8)) + 'px';
    sub.addEventListener('click', e => {
      const b = e.target.closest('[data-k]');
      if (!b || b.disabled) return;
      const it = list[+b.dataset.k], box = m.getBoundingClientRect(), k = +b.dataset.k, hk = host.dataset.k;
      closeMenu();
      it.id && CC.actions[it.id] ? CC.run(it.id) : it.run && it.run();
      if (it.checked && !it.id && !it.radio) {
        const again = menu(box.left, box.top, items, { anchor });
        again.style.left = box.left + 'px';
        again.style.top = box.top + 'px';
        again._openSub(again.querySelector(`[data-k="${hk}"]`));
        $(`#submenu [data-k="${k}"]`)?.focus();
      }
    });
    sub.addEventListener('keydown', e => {
      const bs = $$('button:not([disabled])', sub), i = bs.indexOf(document.activeElement);
      if (e.key === 'ArrowDown') { e.preventDefault(); bs[(i + 1) % bs.length]?.focus(); }
      if (e.key === 'ArrowUp') { e.preventDefault(); bs[(i - 1 + bs.length) % bs.length]?.focus(); }
      if (e.key === 'ArrowLeft') { e.preventDefault(); closeSub(); host.focus(); }
    });
    if (focus) sub.querySelector('button:not([disabled])')?.focus();
  };
  m._openSub = openSub;
  m.addEventListener('mouseover', e => {
    const b = e.target.closest('button[data-k]');
    if (!b) return;
    if (b.hasAttribute('data-sub')) { if (!b.classList.contains('open')) openSub(b); }
    else if (sub) closeSub();
  });
  m.addEventListener('click', e => {
    const b = e.target.closest('[data-k]');
    if (!b || b.disabled) return;
    const it = resolved[+b.dataset.k];
    if (it.sub) return openSub(b, true);
    const box = m.getBoundingClientRect(), k = +b.dataset.k;
    closeMenu();
    it.id && CC.actions[it.id] ? CC.run(it.id) : it.run && it.run();
    // A checkbox item leaves the menu open where it was, so several boxes can be ticked in a row.
    if (it.checked && !it.id && !it.radio) {
      const again = menu(box.left, box.top, items, { anchor });
      again.style.left = box.left + 'px';
      again.style.top = box.top + 'px';
      again.querySelector(`[data-k="${k}"]`)?.focus();
    }
  });
  m.addEventListener('keydown', e => {
    const bs = $$('button:not([disabled])', m), i = bs.indexOf(document.activeElement);
    if (e.key === 'ArrowDown') { e.preventDefault(); bs[(i + 1) % bs.length]?.focus(); }
    if (e.key === 'ArrowUp') { e.preventDefault(); bs[(i - 1 + bs.length) % bs.length]?.focus(); }
    if (e.key === 'ArrowRight' && document.activeElement?.hasAttribute('data-sub')) { e.preventDefault(); openSub(document.activeElement, true); return; }
    if (e.key === 'ArrowRight' && anchor) { e.preventDefault(); Shell.stepMenu(1); }
    if (e.key === 'ArrowLeft' && anchor) { e.preventDefault(); Shell.stepMenu(-1); }
  });
  m._anchor = anchor;
  m.querySelector('button:not([disabled])')?.focus();
  return m;
}
function closeMenu() { $('#submenu')?.remove(); const m = $('#menu'); if (m) { m._anchor?.classList.remove('open'); m.remove(); } }
CC.menu = menu;

let bubble;
document.addEventListener('mouseover', e => {
  if (matchMedia('(hover: none)').matches) return;
  const tr = e.target.closest('[data-tip]');
  if (!tr) { bubble?.remove(); bubble = null; return; }
  if (bubble && bubble._for === tr) return;
  bubble?.remove();
  bubble = document.createElement('div');
  bubble.className = 'bubble';
  bubble._for = tr;
  bubble.textContent = tr.dataset.tip;
  document.body.appendChild(bubble);
  const r = tr.getBoundingClientRect(), b = bubble.getBoundingClientRect();
  bubble.style.left = Math.max(8, Math.min(r.left + r.width / 2 - b.width / 2, innerWidth - b.width - 8)) + 'px';
  bubble.style.top = (r.bottom + b.height + 12 > innerHeight ? r.top - b.height - 8 : r.bottom + 8) + 'px';
});
