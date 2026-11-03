// Ordner bündeln, after the unbalanced plugin: how a user share folder spreads over the disks, gather it onto one
// or empty one disk onto the others. Whole subfolders stay together when they move.
(() => {
  const { seg, segVal, frow, cancel } = CC.ui;
  const bk = CC.bk, U = CC.unraid;
  const free = (d) => FS.FREE['/mnt/' + d];

  function survey(path) {
    const w = U.where(path), files = U.filesOf(FS.nodeAt(path), '').map(([n, p]) => ({ n, rel: w.rel + p, unit: FS.split(p)[0] }));
    const on = Object.fromEntries(U.DEVS.map(d => [d, { size: 0, count: 0 }]));
    files.forEach(f => { on[f.n.dev].size += f.n.size || 0; on[f.n.dev].count++; });
    return { files, on, array: files.filter(f => f.n.dev !== 'cache') };
  }

  function gatherPlan(s, to) {
    const moves = s.array.filter(f => f.n.dev !== to).map(f => ({ ...f, from: f.n.dev, to }));
    const size = moves.reduce((a, m) => a + (m.n.size || 0), 0);
    return { moves, size, short: Math.max(0, size - free(to)) };
  }

  // Largest subfolder first, each onto the disk with the most room left at that point.
  function scatterPlan(s, from) {
    const units = {};
    s.array.filter(f => f.n.dev === from).forEach(f => { (units[f.unit] ||= []).push(f); });
    const room = Object.fromEntries(U.DISKS.filter(d => d !== from).map(d => [d, free(d)]));
    const moves = [], short = [];
    Object.values(units).map(list => ({ list, size: list.reduce((a, f) => a + (f.n.size || 0), 0) })).sort((a, b) => b.size - a.size).forEach(u => {
      const to = Object.keys(room).sort((a, b) => room[b] - room[a])[0];
      if (!to || room[to] < u.size) { short.push(u); return; }
      room[to] -= u.size;
      u.list.forEach(f => moves.push({ ...f, from, to }));
    });
    return { moves, size: moves.reduce((a, m) => a + (m.n.size || 0), 0), short: short.reduce((a, u) => a + u.size, 0) };
  }

  function apply(moves) {
    const touched = new Set();
    for (const m of moves) {
      if (FS.nodeAt(`/mnt/${m.to}/${m.rel}`)) continue;
      U.cut(m.from, m.rel, m.n);
      U.put(m.to, m.rel, m.n);
      m.n.dev = m.to;
      FS.FREE['/mnt/' + m.from] += m.n.size || 0;
      FS.FREE['/mnt/' + m.to] -= m.n.size || 0;
      touched.add(m.from);
    }
    // unbalanced leaves no empty folders behind on the disk it emptied.
    const prune = (dev, rel) => {
      const n = FS.nodeAt(`/mnt/${dev}/${rel}`);
      if (!n?.dir) return;
      [...n.children].filter(k => k.dir && !k.link).forEach(k => prune(dev, `${rel}/${k.name}`));
      if (!n.children.length) U.cut(dev, rel);
    };
    const top = moves[0] && FS.split(moves[0].rel)[0];
    touched.forEach(dev => prune(dev, top));
  }

  CC.action('gather', { label: 'Ordner bündeln', g: 'IconMerge', run: () => {
    const sel = Files.sel(), here = CC.view === 'files' ? bk.here() : '/mnt/user';
    const start = sel.length === 1 && sel[0].dir && !sel[0].link ? FS.join(here, sel[0].name) : here, at = U.where(start);
    let path = at?.share ? '/mnt/user/' + at.rel : '/mnt/user/filme', mode = 'gather', target = null, source = null, s = null, plan = null;
    const w = win({
      title: 'Ordner bündeln', wide: true, cls: 'bk u-win u-gather',
      body: `<div class="well bk-form">
          ${frow('Ordner', bk.pathField('g-dir', path), 'Ein Ordner unter /mnt/user. Sein Inhalt kann über mehrere Platten verteilt liegen, im User-Share sieht man das nicht.')}
          ${frow('Aufgabe', seg([['gather', 'Auf eine Platte holen'], ['scatter', 'Platte leeren']], mode, 'id="g-mode"'), 'Holen bringt alles auf eine Platte, damit beim Abspielen nur sie aufwacht. Leeren verteilt den Teil auf einer Platte auf die anderen, etwa bevor du sie tauschst.')}
          <div id="g-pick"></div></div>
        <div class="well"><div class="bk-tools"><span class="eyebrow" id="g-sum">Verteilung</span></div><div class="u-dist" id="g-bars"></div></div>
        <p class="u-plan" id="g-plan"></p>`,
      foot: [{ start: 'Läuft als Aufgabe im Hintergrund' }, cancel, { label: 'Starten', g: 'IconMerge', primary: true, run: () => go() }],
    });
    bk.wirePicks(w);
    const draw = () => {
      const p = U.where(path);
      if (!p || p.vol !== 'user' || !p.share || !bk.isDir(path)) {
        $('#g-pick', w).innerHTML = '';
        $('#g-bars', w).innerHTML = `<div class="empty">${g('IconFolder')}<div>Wähle einen Ordner unter /mnt/user</div></div>`;
        $('#g-plan', w).textContent = '';
        return bk.setPrimary(w, 'Starten', 'IconMerge', true);
      }
      s = survey(path);
      const used = U.DISKS.filter(d => s.on[d].count);
      if (mode === 'gather') {
        const cost = Object.fromEntries(U.DISKS.map(d => [d, gatherPlan(s, d)]));
        if (!target) target = U.DISKS.filter(d => !cost[d].short).sort((a, b) => cost[a].size - cost[b].size)[0] || U.DISKS[0];
        plan = cost[target];
        $('#g-pick', w).innerHTML = frow('Zielplatte', seg(U.DISKS.map(d => [d, `${U.devName(d)}, ${fmtSize(cost[d].size) || '0 B'} kopieren`]), target, 'id="g-target"'), 'Vorgeschlagen ist die Platte, die am wenigsten kopieren muss und genug Platz hat.');
      } else {
        if (!used.includes(source)) source = used.sort((a, b) => s.on[b].size - s.on[a].size)[0] || null;
        plan = source ? scatterPlan(s, source) : { moves: [], size: 0, short: 0 };
        $('#g-pick', w).innerHTML = used.length ? frow('Platte', seg(used.map(d => [d, U.devName(d)]), source, 'id="g-source"'), 'Nur Platten, auf denen von diesem Ordner etwas liegt.') : '';
      }
      CC.ui.wireControls($('#g-pick', w));
      const after = Object.fromEntries(U.DEVS.map(d => [d, s.on[d].size]));
      plan.moves.forEach(m => { after[m.from] -= m.n.size || 0; after[m.to] += m.n.size || 0; });
      // Sizes like 0.9 GB leave float dust behind once subtracted.
      for (const d in after) after[d] = Math.max(0, Math.round(after[d]));
      const total = s.files.reduce((a, f) => a + (f.n.size || 0), 0) || 1;
      const devs = U.DEVS.filter(d => d !== 'cache' || s.on.cache.count);
      const mark = mode === 'gather' ? target : source;
      $('#g-sum', w).textContent = `${bk.plural(s.files.length, 'Datei', 'Dateien')}, ${fmtSize(total) || '0 B'}`;
      $('#g-bars', w).innerHTML = `<span></span><span></span><span class="u-dh r">Jetzt</span><span class="u-dh r">Danach</span><span class="u-dh r">Frei</span>`
        + devs.map(d => `<span class="u-dn ${d === mark ? 'on' : ''}">${g(d === 'cache' ? 'IconBolt' : 'IconDatabase')}${esc(U.devName(d))}${U.spin[d] === false ? g('IconMoon') : ''}</span>
          <span class="bar"><i style="width:${(s.on[d].size / total * 100).toFixed(1)}%"></i></span>
          <span class="pill glim-num">${fmtSize(s.on[d].size) || '0 B'}</span>
          <span class="pill glim-num ${after[d] !== Math.round(s.on[d].size) ? 'state' : ''}">${fmtSize(after[d]) || '0 B'}</span>
          <span class="glim-num muted r">${fmtSize(free(d))}</span>`).join('');
      const cache = s.on.cache.count ? ` ${bk.plural(s.on.cache.count, 'Datei liegt', 'Dateien liegen')} noch auf dem Cache, die bringt der Mover.` : '';
      let text, ok = plan.moves.length > 0;
      if (mode === 'gather') {
        const from = U.names([...new Set(plan.moves.map(m => m.from))]);
        text = plan.short ? `Auf ${U.devName(target)} fehlen ${fmtSize(plan.short)} Platz.` : !plan.moves.length ? `Der Ordner liegt schon ganz auf ${U.devName(target)}.`
          : `Verschiebt ${fmtSize(plan.size)} in ${bk.plural(plan.moves.length, 'Datei', 'Dateien')} von ${from} auf ${U.devName(target)}.`;
        ok = ok && !plan.short;
        bk.setPrimary(w, `Auf ${U.devName(target)} holen`, 'IconMerge', !ok);
      } else {
        const to = {};
        plan.moves.forEach(m => { to[m.to] = (to[m.to] || 0) + (m.n.size || 0); });
        text = !source ? 'Von diesem Ordner liegt nichts auf dem Array.' : !plan.moves.length ? `Auf den anderen Platten ist zu wenig Platz für ${fmtSize(plan.short)}.`
          : `Verteilt ${fmtSize(plan.size)} von ${U.devName(source)} auf ${Object.entries(to).map(([d, b]) => `${U.devName(d)} (${fmtSize(b)})`).join(' und ')}. Ganze Unterordner bleiben zusammen.${plan.short ? ` ${fmtSize(plan.short)} passen nirgends hin und bleiben.` : ''}`;
        bk.setPrimary(w, source ? `${U.devName(source)} leeren` : 'Platte leeren', 'IconMerge', !ok);
      }
      $('#g-plan', w).textContent = text + cache;
    };
    function go() {
      if (!plan?.moves.length) return false;
      const moves = plan.moves, name = FS.split(path).pop();
      U.wake(moves.flatMap(m => [m.from, m.to]));
      const label = mode === 'gather' ? `Bündeln: ${name} auf ${U.devName(target)}` : `Leeren: ${name} von ${U.devName(source)}`;
      Jobs.start({ kind: 'move', label, total: plan.size, speed: plan.size > 10 * GB ? 0.5 : 1, done: () => {
        apply(moves);
        bk.log('verschoben', `${bk.plural(moves.length, 'Datei', 'Dateien')} aus ${path} zwischen den Platten`);
        CC.toast(label.replace('Bündeln:', 'Gebündelt:').replace('Leeren:', 'Geleert:'));
        if (CC.view === 'files') CC.render();
      } });
      CC.toast(`${label} läuft`, 'ok', { label: 'Aufgaben', run: () => CC.run('jobs') });
    }
    w.addEventListener('change', e => {
      if (e.target.closest('#g-mode')) mode = segVal($('#g-mode', w));
      if (e.target.closest('#g-target')) target = segVal($('#g-target', w));
      if (e.target.closest('#g-source')) source = segVal($('#g-source', w));
      draw();
    });
    w.addEventListener('input', e => { if (e.target.id === 'g-dir') { path = e.target.value.trim(); target = null; source = null; draw(); } });
    draw();
  } });
  const tools = MENUS.find(m => m[0] === 'Werkzeuge')[1];
  tools.splice(tools.indexOf('diskUsage') + 1, 0, 'gather');

  CC.css(`
.u-dist { display: grid; grid-template-columns: 9rem minmax(0, 1fr) 5.2rem 5.2rem 5rem; gap: .45rem .7rem; align-items: center; }
.u-dist .empty { grid-column: 1 / -1; }
.u-dh { font-size: 12px; color: var(--carbon-text-muted); }
.u-dist .r { text-align: right; justify-self: end; font-size: 12px; }
.u-dn { display: flex; align-items: center; gap: .45rem; font-size: 13px; }
.u-dn svg { width: 1rem; height: 1rem; color: var(--carbon-text-sub); flex: none; }
.u-dn svg + svg { width: .8rem; height: .8rem; color: var(--carbon-text-muted); }
.u-dn.on { font-weight: 600; }
.u-dn.on svg:first-child { color: var(--accent-glyph); }
.u-dist .pill { justify-self: end; }
@media (max-width: 760px) { .u-dist { grid-template-columns: 6.5rem minmax(0, 1fr) 4.6rem 4.6rem; } .u-dist > :nth-child(5n) { display: none; } }
`);
})();
