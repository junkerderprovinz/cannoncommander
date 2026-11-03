// Konflikte: when names already exist at the target, each one is decided here before anything is written.
(() => {
  const { plural } = CC.ck;
  const CHOICES = [['replace', 'Ersetzen', 'IconKeepLeft'], ['skip', 'Überspringen', 'IconKeepRight'], ['both', 'Beide behalten', 'IconKeepBoth']];

  const pick = (rule, k, old) => rule === 'newer' ? (k.time > old.time ? 'replace' : 'skip') : rule;

  CC.resolveConflicts = (items, dest, rule = 'ask') => new Promise((resolve) => {
    const target = FS.nodeAt(dest);
    const rows = items.map(k => ({ k, old: FS.kids(target).find(c => c.name === k.name) }));
    // A rule chosen before the copy started answers every name without asking.
    if (rule !== 'ask') return resolve(Object.fromEntries(rows.map(({ k, old }) => [k.name, pick(rule, k, old)])));
    const choice = Object.fromEntries(rows.map(({ k }) => [k.name, 'both']));
    let answered = false;
    const side = (n, other, label) => {
      const size = FS.totalSize(n), newer = n.time > other.time + 60000;
      return `<div class="cf-side"><span class="cf-lab">${label}</span><span class="pill glim-num">${n.dir ? `${plural(FS.kids(n).length, 'Objekt', 'Objekte')}, ` : ''}${fmtSize(size) || '0 B'}</span><span class="pill glim-num">${fmtTime(n.time)}</span>${newer ? '<span class="pill state">neuer</span>' : ''}</div>`;
    };
    const w = win({
      title: 'Konflikte', wide: true,
      body: `<div class="well cf-all">${frow(`Für alle übernehmen`, seg([...CHOICES, ['newer', 'Nur Neuere ersetzen', 'IconUpdate']], '', 'id="cf-all"'), 'Setzt jede Zeile unten auf dieselbe Antwort. Nur Neuere ersetzen ersetzt eine Datei nur, wenn die neue ein jüngeres Änderungsdatum hat, und überspringt sonst.')}</div>
        <div class="well"><div class="eyebrow">${plural(rows.length, 'Name gibt', 'Namen gibt')} es in ${esc(FS.split(dest).pop() || 'Server')} schon</div>
          <div class="cf-list">${rows.map(({ k, old }, i) => `<div class="cf-row" data-i="${i}">
            <div class="cf-name"><span class="tile">${g(k.dir ? 'IconFolder' : FS.glyphOf(k))}</span><b>${esc(k.name)}</b></div>
            <div class="cf-sides">${side(k, old, 'Neu')}${side(old, k, 'Vorhanden')}</div>
            ${seg(CHOICES, choice[k.name], `data-row="${i}"`)}
          </div>`).join('')}</div></div>`,
      foot: [{ start: `Ziel: ${esc(dest)}` }, cancel, { label: 'Weiter', g: 'IconForward', primary: true, run: () => { answered = true; resolve(choice); } }],
      onClose: () => { if (!answered) resolve(null); },
    });
    w.addEventListener('change', (e) => {
      const s = e.target.closest('.seg');
      if (!s) return;
      if (s.id === 'cf-all') {
        const v = segVal(s);
        rows.forEach(({ k, old }, i) => {
          const c = pick(v, k, old);
          choice[k.name] = c;
          $$(`[data-row="${i}"] button`, w).forEach(b => b.setAttribute('aria-pressed', String(b.dataset.v === c)));
        });
        return;
      }
      choice[rows[+s.dataset.row].k.name] = segVal(s);
      $$('#cf-all button', w).forEach(b => b.setAttribute('aria-pressed', 'false'));
    });
  });

  CC.css(`
.cf-all .frow { flex-wrap: wrap; row-gap: .5rem; }
.cf-list { display: flex; flex-direction: column; max-height: 24rem; overflow: auto; margin: 0 -.3rem; }
.cf-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; grid-template-areas: "name seg" "sides seg"; gap: .35rem 1rem; align-items: center; padding: .6rem .3rem; border-radius: var(--radius-control); }
.cf-row + .cf-row { box-shadow: inset 0 1px 0 color-mix(in srgb, var(--carbon-text) 7%, transparent); }
.cf-name { grid-area: name; display: flex; align-items: center; gap: .6rem; min-width: 0; }
.cf-name b { font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cf-name .tile { background: var(--carbon-surface3); flex: none; }
.cf-name .tile svg { color: var(--carbon-text-sub); }
.cf-sides { grid-area: sides; display: flex; flex-direction: column; gap: .25rem; padding-left: 2.35rem; }
.cf-side { display: flex; align-items: center; gap: .35rem; flex-wrap: wrap; }
.cf-side .pill { background: var(--carbon-surface3); justify-self: auto; }
.cf-side .pill.state { background: var(--carbon-hover-raised); font-weight: 600; }
.cf-lab { width: 5.2rem; font-size: 12px; color: var(--carbon-text-muted); }
.cf-row .seg { grid-area: seg; background: var(--carbon-surface3); }
.cf-row .seg button:hover { background: var(--carbon-hover-raised); }
@media (max-width: 900px) {
  .cf-row { grid-template-columns: 1fr; grid-template-areas: "name" "sides" "seg"; }
  .cf-row .seg { justify-self: start; margin-left: 2.35rem; max-width: calc(100% - 2.35rem); overflow-x: auto; }
  .cf-sides { padding-left: 2.35rem; }
  .cf-lab { width: auto; min-width: 4.5rem; }
  .cf-all .seg { overflow-x: auto; max-width: 100%; }
}
`);
})();
