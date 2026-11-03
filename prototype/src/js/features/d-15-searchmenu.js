// A click into the empty search field opens its menu: the saved searches and a few examples of what the field understands.
// While you type, the menu steps aside so the hits stay visible.
(() => {
  const EXAMPLES = [
    ['*.mkv', 'alle MKV-Dateien, auch in Unterordnern'],
    ['#steuer', 'alles mit dem Schlagwort steuer'],
    ['bericht', 'Namen, in denen bericht vorkommt'],
    ['IMG_????.jpg', 'ein ? steht für genau ein Zeichen'],
  ];
  const close = () => $$('.qdrop').forEach(d => d.remove());

  function show(input) {
    close();
    if (input.value) return;
    const rows = CC.smart.rows();
    input.closest('.qfilter').insertAdjacentHTML('beforeend', `<div class="qdrop glim-card" role="menu">
      <div class="eyebrow">Gespeicherte Suchen<button class="btn icon" data-run="search" data-tip="Neue Suche mit allen Feldern (Alt+F7)" aria-label="Neue Suche">${g('IconAdd')}</button></div>
      ${rows || '<div class="qd-none muted">Tipp ein Muster und speichere es mit dem Knopf im Feld.</div>'}
      <div class="eyebrow">Beispiele</div>
      ${EXAMPLES.map(([q, say]) => `<button class="qrow qex" data-example="${esc(q)}"><b>${esc(q)}</b><span class="muted">${esc(say)}</span></button>`).join('')}
    </div>`);
  }

  document.addEventListener('focusin', e => { if (e.target.matches('.qfilter input')) show(e.target); });
  document.addEventListener('focusout', e => { if (e.target.matches('.qfilter input')) close(); });
  document.addEventListener('input', e => { if (e.target.isConnected && e.target.matches('.qfilter input')) show(e.target); });
  // A click in the menu must not take the focus from the field, or the menu would close before the click lands.
  document.addEventListener('mousedown', e => { if (e.target.closest('.qdrop')) e.preventDefault(); });
  document.addEventListener('click', e => {
    if (e.target.closest('[data-qfsave]')) return CC.smart.saveFilter();
    const ex = e.target.closest('[data-example]');
    if (ex) {
      Object.assign(Files.T(), { filter: ex.dataset.example, cursor: 0 });
      CC.render();
      return bindFilter($(`#qf-${Files.active}`), ex.dataset.example.length);
    }
    if (e.target.closest('.qdrop [data-smart], .qdrop [data-run]')) close();
  });

  CC.css(`
.qdrop { position: absolute; top: calc(100% + .45rem); right: 0; width: 22rem; max-height: min(28rem, calc(100vh - 12rem)); overflow: auto; padding: .7rem .35rem .45rem; display: flex; flex-direction: column; gap: .1rem; z-index: 5; cursor: default; }
.qdrop .eyebrow { padding-top: .2rem; }
.qdrop .eyebrow:not(:first-child) { margin-top: .6rem; }
.qrow { height: 2rem; padding: 0 .6rem; border-radius: var(--radius-control); display: flex; align-items: center; gap: .6rem; text-align: left; white-space: nowrap; flex: none; }
.qrow svg { width: .95rem; height: .95rem; color: var(--accent-glyph); flex: none; }
.qrow span:first-of-type { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; }
.qrow .glim-num { font-size: 12px; color: var(--carbon-text-muted); }
.qrow:hover, .qrow:focus-visible, .qrow.cur { background: var(--carbon-hover); outline: none; }
.qex b { font-family: var(--font-mono, monospace); font-weight: 500; font-size: 12px; min-width: 6.5rem; color: var(--carbon-text); }
.qex span { font-size: 12px; }
.qd-none { font-size: 12px; padding: .25rem .6rem; white-space: normal; line-height: 1.45; }
`);
})();
