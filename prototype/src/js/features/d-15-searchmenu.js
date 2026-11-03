// A click into the empty search field opens its menu: the saved searches and every colour and tag in use.
// While you type, the menu steps aside so the hits stay visible.
(() => {
  const close = () => $$('.qdrop').forEach(d => d.remove());

  function show(input) {
    close();
    if (input.value) return;
    const rows = CC.smart.rows(), chips = Labels.chips();
    input.closest('.qfilter').insertAdjacentHTML('beforeend', `<div class="qdrop glim-card" role="menu">
      <div class="eyebrow">Gespeicherte Suchen<button class="btn icon" data-run="search" data-tip="Neue Suche mit allen Feldern (Alt+F7)" aria-label="Neue Suche">${g('IconAdd')}</button></div>
      ${rows || '<div class="qd-none muted">Tipp ein Muster wie *.mkv und speichere es mit dem Knopf im Feld.</div>'}
      ${chips ? `<div class="eyebrow">Markierungen</div><div class="qd-chips">${chips}</div>` : ''}
      <div class="qd-hint muted"><b>*.mkv</b> findet alle MKV-Dateien, <b>#steuer</b> alles mit diesem Schlagwort</div>
    </div>`);
  }

  document.addEventListener('focusin', e => { if (e.target.matches('.qfilter input')) show(e.target); });
  document.addEventListener('focusout', e => { if (e.target.matches('.qfilter input')) close(); });
  document.addEventListener('input', e => { if (e.target.isConnected && e.target.matches('.qfilter input')) show(e.target); });
  // A click in the menu must not take the focus from the field, or the menu would close before the click lands.
  document.addEventListener('mousedown', e => { if (e.target.closest('.qdrop')) e.preventDefault(); });
  document.addEventListener('click', e => {
    if (e.target.closest('[data-qfsave]')) return CC.smart.saveFilter();
    if (e.target.closest('.qdrop [data-smart], .qdrop [data-labelsearch], .qdrop [data-run]')) close();
  });

  CC.css(`
.qdrop { position: absolute; top: calc(100% + .45rem); right: 0; width: 21rem; max-height: min(28rem, calc(100vh - 12rem)); overflow: auto; padding: .7rem .35rem .45rem; display: flex; flex-direction: column; gap: .1rem; z-index: 5; cursor: default; }
.qdrop .eyebrow { padding-top: .2rem; }
.qdrop .eyebrow:not(:first-child) { margin-top: .5rem; }
.qrow { height: 2rem; padding: 0 .6rem; border-radius: var(--radius-control); display: flex; align-items: center; gap: .6rem; text-align: left; white-space: nowrap; flex: none; }
.qrow svg { width: .95rem; height: .95rem; color: var(--accent-glyph); flex: none; }
.qrow span:first-of-type { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; }
.qrow .glim-num { font-size: 12px; color: var(--carbon-text-muted); }
.qrow:hover, .qrow:focus-visible, .qrow.cur { background: var(--carbon-hover); outline: none; }
.qd-chips { display: flex; flex-wrap: wrap; gap: .35rem; padding: 0 .4rem; }
.qchip { height: 1.75rem; padding: 0 .65rem; border-radius: var(--radius-pill); display: inline-flex; align-items: center; gap: .4rem; background: var(--carbon-surface3); color: var(--carbon-text-sub); font-size: 13px; }
.qchip svg { width: .8rem; height: .8rem; color: var(--accent-glyph); }
.qchip .glim-num { font-size: 11px; color: var(--carbon-text-muted); }
.qchip:hover { background: var(--carbon-hover-raised); color: var(--carbon-text); }
.qd-none, .qd-hint { font-size: 12px; padding: .25rem .6rem; white-space: normal; line-height: 1.45; }
.qd-hint { margin-top: .55rem; padding-top: .55rem; border-top: 1px solid var(--carbon-border, color-mix(in srgb, var(--carbon-text) 10%, transparent)); }
.qd-hint b { font-family: var(--font-mono, monospace); font-weight: 500; color: var(--carbon-text-sub); }
`);
})();
