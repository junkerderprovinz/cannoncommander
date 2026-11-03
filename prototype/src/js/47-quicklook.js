// Quick look: the space bar shows the file under the cursor large, without leaving the list, as on a Mac.
// The arrows walk through the folder while it is open; I adds the file's details beside the picture.
(() => {
  let el = null;

  const close = () => { el?.remove(); el = null; $('.panel.active .list')?.focus(); };
  const step = (d) => {
    const t = Files.T(), n = Files.entries().length;
    t.cursor = Math.max(0, Math.min(n - 1, t.cursor + d));
    CC.render();
    $('.panel.active .row.cursor')?.scrollIntoView({ block: 'nearest' });
    show();
  };
  const openIt = () => { const k = Files.cursorEntry(); close(); Files.open(Files.active, k); };
  // Only the details column moves; a full re-render would restart the player underneath.
  const toggleInfo = () => {
    CC.prefs.quickInfo = !CC.prefs.quickInfo;
    try { localStorage.setItem('cannoncommander-prefs', JSON.stringify(CC.prefs)); } catch {}
    $('.ql', el).classList.toggle('with-info', CC.prefs.quickInfo);
  };

  function show() {
    const t = Files.T(), list = Files.entries(), k = list[t.cursor];
    if (!k) return close();
    if (!el) {
      el = document.createElement('div');
      el.className = 'ql-scrim';
      el.innerHTML = `<div class="ql glim-card" role="dialog" aria-label="Schnellansicht"><span class="notch"><span class="ql-name"></span><span class="ql-count glim-num"></span></span><div class="ql-body"></div></div>`;
      document.body.appendChild(el);
      el.addEventListener('mousedown', e => { if (e.target === el) close(); });
    }
    $('.ql', el).classList.toggle('with-info', !!CC.prefs.quickInfo);
    $('.ql-name', el).textContent = k.name;
    $('.ql-count', el).textContent = `${t.cursor + 1} von ${list.length}`;
    const body = $('.ql-body', el);
    body.innerHTML = '';
    CC.renderPreview(k, body, k.at || t.path, { autoplay: true });
  }

  // While the preview is up it owns the keyboard; everything else would act on the list behind it.
  document.addEventListener('keydown', e => {
    if (!el) return;
    const steps = { ArrowLeft: -1, ArrowUp: -1, ArrowRight: 1, ArrowDown: 1 };
    const act = e.key === ' ' || e.key === 'Escape' ? close : steps[e.key] ? () => step(steps[e.key]) : e.key === 'Enter' ? openIt
      : e.key.toLowerCase() === 'i' ? toggleInfo : null;
    if (!act) return;
    e.preventDefault();
    e.stopImmediatePropagation();
    act();
  }, true);

  CC.action('quickLook', { label: 'Schnellansicht', g: 'IconEye', key: 'Leertaste', when: () => !!Files.cursorEntry(), run: show });

  CC.css(`
.ql-scrim { position: fixed; inset: 0; z-index: 58; display: grid; place-items: center; background: rgb(0 0 0 / .5); }
.ql { position: relative; width: min(56rem, calc(100vw - 2rem)); max-height: calc(100vh - 4rem); padding: 1.4rem .9rem .9rem; display: flex; flex-direction: column; }
.ql .notch { gap: .6rem; }
.ql-count { font-weight: 500; letter-spacing: 0; opacity: .7; }
.ql-body { flex: none; max-height: calc(100vh - 7rem); overflow: auto; display: grid; grid-template-columns: minmax(0, 1fr); gap: .75rem; align-items: start; }
.ql.with-info .ql-body { grid-template-columns: minmax(0, 1fr) 17rem; }
.ql:not(.with-info) .cv-details { display: none; }
.ql-body > .cv-compact { min-height: 24rem; max-height: calc(100vh - 8rem); }
@media (max-width: 760px) { .ql.with-info .ql-body { grid-template-columns: minmax(0, 1fr); } }
`);
})();
