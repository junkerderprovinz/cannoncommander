// Before a copy into a user share, ask the question Unraid cannot: will the disk it is about to pick hold the biggest file?
// Unraid chooses the disk when a file is created and cannot know its final size, so a file larger than the free space
// stops halfway with "disk full" even while other disks of the share have room.
(() => {
  const bk = CC.bk, U = CC.unraid;
  FS.FREE['/mnt/disk1'] = 48 * GB;

  // How each share picks its disk and how much room it leaves free; in Unraid both sit in the share settings.
  const RULES = {
    filme: { alloc: 'fill', min: 30 * GB },
    backups: { alloc: 'most', min: 20 * GB },
    isos: { alloc: 'high', min: 10 * GB },
  };
  const ruleOf = (share) => RULES[share] || { alloc: 'high', min: 10 * GB };
  const ALLOC = { fill: 'Auffüllen', most: 'Meister freier Platz', high: 'Hochwasser' };
  const free = (dev) => FS.FREE['/mnt/' + dev] || 0;

  // The disk Unraid would write to: among the share's disks with more than the minimum free, by its allocation method.
  function pick(share) {
    const s = U.shareOf(share), r = ruleOf(share);
    const disks = s.disks.filter(d => free(d) > r.min);
    if (!disks.length) return null;
    if (r.alloc === 'fill') return disks[0];
    if (r.alloc === 'most') return disks.reduce((a, d) => (free(d) > free(a) ? d : a));
    // High water: the first disk above a mark that starts at half the largest disk and halves while nothing is above it.
    let mark = Math.max(...s.disks.map(d => FS.TOTAL['/mnt/' + d] || 0)) / 2;
    while (mark > r.min) { const d = disks.find(x => free(x) >= mark); if (d) return d; mark /= 2; }
    return disks[0];
  }

  function check(items, from, dest, kind) {
    const w = U.where(dest);
    if (!w || w.vol !== 'user' || !w.share) return null;
    const files = items.flatMap(k => { const n = bk.nodeIn(from, k.name); return n ? U.filesOf(n) : []; });
    if (!files.length) return null;
    const src = U.where(from);
    if (kind === 'move' && src?.vol === 'user' && src.share === w.share) return null;
    const s = U.shareOf(w.share), r = ruleOf(w.share);
    const total = files.reduce((a, [n]) => a + (n.size || 0), 0);
    const [big, bigRel] = files.reduce((a, f) => ((f[0].size || 0) > (a[0].size || 0) ? f : a));
    if (s.primary === 'cache' && free('cache') - total > 0) return null;
    const room = s.disks.reduce((a, d) => a + Math.max(0, free(d) - r.min), 0);
    const disk = pick(w.share);
    const roomy = s.disks.filter(d => free(d) >= big.size).sort((a, b) => free(b) - free(a))[0];
    if (!disk) return { why: `Keine Platte von ${w.share} hat mehr als den Mindestplatz von ${fmtSize(r.min)} frei. Unraid würde die Kopie gar nicht erst anfangen.`, roomy, w };
    if (total > room) return { why: `Zusammen sind es ${fmtSize(total)}, auf den Platten von ${w.share} ist aber nur noch ${fmtSize(room)} über dem Mindestplatz frei.`, roomy, w };
    if (big.size > free(disk)) {
      return { why: `Unraid würde ${U.devName(disk)} nehmen, weil dort mehr als der Mindestplatz von ${fmtSize(r.min)} frei ist (Zuteilung ${ALLOC[r.alloc]}). Die größte Datei, ${FS.split(bigRel).pop()} mit ${fmtSize(big.size)}, passt aber nicht auf die ${fmtSize(free(disk))}, die dort frei sind. Die Kopie würde mittendrin abbrechen.`, roomy, w, tip: true };
    }
    return null;
  }

  const panel = (res) => `<div class="u-alert u-space">${g('IconWarning')}<div><b>Das passt so nicht</b><p>${esc(res.why)}</p>
    ${res.tip ? `<p class="muted">Stellst du den Mindestplatz des Shares mindestens auf die größte Datei, überspringt Unraid zu volle Platten von selbst.</p>` : ''}
    ${res.roomy ? `<button class="btn" data-space-disk="${res.roomy}">${g('IconDatabase')}Direkt auf ${esc(U.devName(res.roomy))} legen, ${fmtSize(free(res.roomy))} frei</button>` : ''}</div></div>`;

  const open = transferWindow;
  transferWindow = (kind) => {
    open(kind);
    const w = $$('.scrim .xwin').at(-1), input = w && $('#dest', w);
    if (!input) return;
    const sel = Files.sel(), from = Files.T().path;
    const slot = document.createElement('div');
    $('.well', w).before(slot);
    const show = () => {
      const res = check(sel, from, input.value.trim().replace(/\/$/, ''), kind);
      slot.innerHTML = res ? panel(res) : '';
      $('.win-foot .btn.primary', w).lastChild.textContent = res ? `Trotzdem ${kind === 'copy' ? 'kopieren' : 'verschieben'}` : (kind === 'copy' ? 'Kopieren' : 'Verschieben');
    };
    slot.addEventListener('click', e => {
      const b = e.target.closest('[data-space-disk]');
      if (!b) return;
      const at = U.where(input.value.trim().replace(/\/$/, ''));
      const to = `/mnt/${b.dataset.spaceDisk}/${at.rel}`;
      bk.mkdirs(to);
      input.value = to;
      input.dispatchEvent(new Event('input', { bubbles: true }));
    });
    input.addEventListener('input', show);
    show();
  };

  CC.css(`
.u-space { padding: .85rem 1rem; border-radius: var(--radius-control); background: color-mix(in srgb, #FF832B 10%, var(--carbon-surface2)); box-shadow: inset 0 0 0 1px color-mix(in srgb, #FF832B 45%, transparent); }
.u-space p { margin: .3rem 0 0; font-size: 13px; line-height: 1.45; }
.u-space .btn { margin-top: .6rem; }
`);
})();
