// Auswahlmodus: a long press on a row starts it, every row gets a box to tick, and a bar at the bottom carries the actions.
(() => {
  const HOLD = 500;
  const Sel = { on: false, path: null };
  let press = null, swallow = false;

  function start(panel, idx) {
    Files.active = panel;
    const t = Files.T(), k = Files.entries()[idx];
    Object.assign(Sel, { on: true, path: t.path });
    if (k && !k.up) t.marked.add(k.name);
    if (k) t.cursor = idx;
    CC.render();
    navigator.vibrate?.(12);
  }
  function stop() {
    Sel.on = false;
    Files.T().marked = new Set();
    CC.render();
  }
  function toggle(panel, idx) {
    Files.active = panel;
    const t = Files.T(), k = Files.entries()[idx];
    if (!k || k.up) return;
    t.marked.has(k.name) ? t.marked.delete(k.name) : t.marked.add(k.name);
    t.cursor = idx;
    CC.render();
  }

  const rowAt = (e) => CC.view === 'files' && e.target.closest?.('.panel[data-panel] .list .row');
  document.addEventListener('pointerdown', e => {
    swallow = false;
    const row = e.pointerType === 'touch' && rowAt(e);
    if (!row) return;
    const panel = +row.closest('.panel').dataset.panel, idx = +row.dataset.idx;
    clearTimeout(press?.timer);
    press = { x: e.clientX, y: e.clientY, timer: setTimeout(() => {
      press = null;
      swallow = true;
      Sel.on ? toggle(panel, idx) : start(panel, idx);
    }, HOLD) };
  });
  const drop = () => { if (press) { clearTimeout(press.timer); press = null; } };
  document.addEventListener('pointermove', e => { if (press && Math.hypot(e.clientX - press.x, e.clientY - press.y) > 10) drop(); });
  document.addEventListener('pointerup', drop);
  document.addEventListener('pointercancel', drop);

  // The browser answers a long press with its own context menu, a drag or a click; the selection has taken it over.
  document.addEventListener('contextmenu', e => { if (press || swallow) { e.preventDefault(); e.stopImmediatePropagation(); } }, true);
  document.addEventListener('dragstart', e => { if (press || Sel.on) { e.preventDefault(); e.stopImmediatePropagation(); } }, true);
  document.addEventListener('click', e => {
    if (swallow) { swallow = false; if (rowAt(e)) { e.preventDefault(); e.stopImmediatePropagation(); return; } }
    const row = Sel.on && rowAt(e);
    if (!row) return;
    e.preventDefault();
    e.stopImmediatePropagation();
    toggle(+row.closest('.panel').dataset.panel, +row.dataset.idx);
  }, true);
  document.addEventListener('keydown', e => { if (Sel.on && e.key === 'Escape' && !winStack.length && !$('#menu')) { e.preventDefault(); e.stopImmediatePropagation(); stop(); } }, true);

  const ACTS = [['copy', 'Kopieren', 'IconCopy'], ['move', 'Verschieben', 'IconMove'], ['share', 'Teilen', 'IconLink'], ['delete', 'Löschen', 'IconTrash']];
  function decorate() {
    const app = $('#app');
    // Going to another folder ends the selection, as a phone's gallery does.
    if (Sel.on && Files.T().path !== Sel.path) Sel.on = false;
    app.classList.toggle('touch-sel', Sel.on);
    if (!Sel.on) return;
    const panel = $('.panel.active'), t = Files.T();
    $$('.list .row', panel).forEach(r => {
      const k = Files.entries()[+r.dataset.idx];
      if (!k || k.up || t.marked.has(k.name)) return;
      const box = '<span class="cbox"></span>';
      const glyph = $('.name .glyph', r);
      if (glyph) glyph.innerHTML = box; else r.insertAdjacentHTML('afterbegin', box);
    });
    const n = t.marked.size;
    app.insertAdjacentHTML('beforeend', `<div class="touchbar glim-card" role="toolbar" aria-label="Auswahl">
      <span class="tb-count glim-num">${n ? `${n} markiert` : 'Tippe auf Zeilen'}</span>
      ${ACTS.map(([id, l, gl]) => `<button data-run="${id}" ${n ? '' : 'disabled'}>${g(gl)}<span>${l}</span></button>`).join('')}
      <button data-tsel-done class="tb-done">${g('IconCheck')}<span>Fertig</span></button></div>`);
  }
  const render = CC.views.files.render;
  CC.views.files.render = (host) => { render(host); decorate(); };
  document.addEventListener('click', e => { if (e.target.closest('[data-tsel-done]')) stop(); });

  CC.action('touchSelect', { label: 'Auswahlmodus', g: 'IconSelectAll', checked: () => Sel.on, run: () => {
    if (Sel.on) return stop();
    if (CC.view !== 'files') CC.go('files');
    start(Files.active, Files.T().cursor);
  } });
  const edit = MENUS.find(([m]) => m === 'Bearbeiten')[1];
  edit.splice(edit.indexOf('selectAll'), 0, 'touchSelect');

  CC.css(`
.touch-sel .row { user-select: none; -webkit-user-select: none; }
.touch-sel .row .glyph .cbox { box-shadow: inset 0 0 0 1.5px var(--carbon-text-muted); background: transparent; }
.touch-sel :is(.gcard, .citem) > .cbox:not(.on) { box-shadow: inset 0 0 0 1.5px var(--carbon-text-muted); background: var(--carbon-surface); }
.touchbar { position: fixed; z-index: 45; left: 50%; translate: -50% 0; bottom: 1rem; width: min(36rem, calc(100vw - 1.5rem)); display: flex; align-items: stretch; gap: 3px; padding: 3px; }
.touchbar .tb-count { display: flex; align-items: center; padding: 0 .7rem; font-size: 13px; font-weight: 600; white-space: nowrap; color: var(--carbon-text); }
.touchbar button { flex: 1; min-width: 0; min-height: 3rem; border-radius: var(--radius-control); display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 2px; font-size: 11px; color: var(--carbon-text-sub); }
.touchbar button svg { width: 1.15rem; height: 1.15rem; color: var(--accent-glyph); }
.touchbar button:hover:not(:disabled) { background: var(--carbon-hover); color: var(--carbon-text); }
.touchbar button:disabled { opacity: .4; }
.touchbar .tb-done { background: var(--accent); color: var(--accent-contrast); }
.touchbar .tb-done svg { color: var(--accent-contrast); }
@media (max-width: 760px) {
  .touchbar { bottom: .75rem; flex-wrap: wrap; }
  .touchbar .tb-count { width: 100%; padding: .3rem .6rem .1rem; font-size: 12px; }
  .app.touch-sel .bottombar { visibility: hidden; }
  .app.touch-sel { padding-bottom: 7.5rem; }
}
`);
})();
