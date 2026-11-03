// Copy and move on Unraid: the disk share against user share trap, hard links and where a file lives.
(() => {
  const { seg, segVal, frow, cancel } = CC.ui;
  const bk = CC.bk, U = CC.unraid;
  const verbOf = (kind) => kind === 'copy' ? 'Kopieren' : 'Verschieben';

  // Every file of the selection as the real node, with its path below the source folder.
  const picked = (items, from) => items.flatMap(k => { const n = bk.nodeIn(from, k.name); return n ? U.filesOf(n) : []; });

  // Through /mnt/diskN and /mnt/user the same file can sit on both ends of a copy; Unraid then truncates it to 0 bytes.
  function sameFiles(items, dest, from) {
    const a = U.where(from), b = U.where(dest);
    if (!a || !b || (a.vol === 'user') === (b.vol === 'user')) return [];
    return picked(items, from).filter(([n, rel]) => FS.nodeAt(FS.join(dest, rel)) === n);
  }

  // A copy always gets its own data; a move does too once it crosses a share or a pool.
  function brokenLinks(kind, items, dest, from) {
    const w = U.where(dest);
    return picked(items, from).map(([n, rel]) => ({ n, rel, others: U.linksOf(n) })).filter(x => x.others.length
      && (kind === 'copy' || !w || U.landsOn(dest, kind, x.n) !== x.n.dev || (w.vol === 'user' && w.share !== x.n.share)));
  }

  function linkable(items, from, dest) {
    const a = U.where(from), b = U.where(dest);
    if (!a || !b || from === dest || !bk.isDir(dest)) return false;
    const files = picked(items, from);
    return files.length > 0 && files.every(([n]) => b.vol === 'user' ? b.share && U.allowed(n.dev, b.share) : b.vol === n.dev);
  }

  function makeLinks(items, dest, from) {
    if (!linkable(items, from, dest)) return CC.toast('Ein harter Link geht nur auf derselben Platte oder im selben Pool', 'warn');
    const target = FS.nodeAt(dest), share = U.where(dest).share;
    const twin = (n) => { n.ino ??= U.newIno(); const c = bk.clone(n); delete c.uid; c.share = share; return c; };
    const mirror = (n) => n.dir && !n.link ? { ...bk.clone({ ...n, children: [] }), children: n.children.map(mirror) } : twin(n);
    let made = 0;
    for (const k of items) {
      const n = bk.nodeIn(from, k.name);
      if (!n) continue;
      const c = mirror(n);
      if (bk.nodeIn(dest, c.name)) c.name = Files.freeName(target, c.name);
      target.children.push(c);
      made += U.filesOf(c).length;
    }
    bk.log('verknüpft', `${bk.plural(made, 'harter Link', 'harte Links')} in ${dest}`);
    CC.render();
    CC.toast(`${bk.plural(made, 'harter Link', 'harte Links')} angelegt, belegt keinen neuen Platz`);
  }

  const fileRows = (list, cell) => `<table class="tbl u-tbl">${list.slice(0, 6).map(x => `<tr>${cell(x)}</tr>`).join('')}${list.length > 6 ? `<tr><td colspan="3">und ${list.length - 6} weitere</td></tr>` : ''}</table>`;
  const alert = (title, text) => `<div class="u-alert">${g('IconWarning')}<div><b>${esc(title)}</b><p>${text}</p></div></div>`;

  function sameFileWindow(kind, items, dest, from, rule, same) {
    const a = U.where(from), b = U.where(dest), verb = verbOf(kind);
    const counts = {};
    same.forEach(([n]) => { counts[n.dev] = (counts[n.dev] || 0) + 1; });
    const src = a.vol === 'user' ? Object.keys(counts).sort((x, y) => counts[y] - counts[x])[0] : a.vol;
    const targets = U.DISKS.filter(d => d !== src).sort((x, y) => FS.FREE['/mnt/' + y] - FS.FREE['/mnt/' + x]);
    const diskFrom = `/mnt/${src}${a.rel ? '/' + a.rel : ''}`, shareFrom = `/mnt/user${a.rel ? '/' + a.rel : ''}`, shareTo = `/mnt/user${b.rel ? '/' + b.rel : ''}`;
    const w = win({
      title: 'Dieselbe Datei', wide: true, cls: 'bk u-win',
      body: `<div class="well u-warnwell">${alert('Quelle und Ziel sind dieselbe Datei',
          `${esc(a.vol === 'user' ? dest : from)} ist ein Disk-Share, ${esc(a.vol === 'user' ? from : dest)} der User-Share darüber. Beide zeigen auf dieselben Dateien. ${verb} von einem zum anderen kürzt sie auf 0 Byte, die Daten sind danach weg.`)}
          ${fileRows(same, ([n, rel]) => `<td class="mono">${esc(rel)}</td><td>${esc(U.devName(n.dev))}</td><td class="glim-num r">${fmtSize(n.size)}</td>`)}</div>
        <div class="well bk-form"><div class="eyebrow">Sicherer Weg</div>
          ${frow('Weg', seg([['disk', 'Disk zu Disk'], ['share', 'Share zu Share']], 'disk', 'id="u-way"'), 'Sicher ist es, wenn beide Seiten Disk-Shares sind oder beide User-Shares. Nur das Mischen ist gefährlich.')}
          <div id="u-diskrow">${frow('Zielplatte', seg(targets.map(d => [d, `${U.devName(d)}, ${fmtSize(FS.FREE['/mnt/' + d])} frei`]), targets[0], 'id="u-disk"'), `Was auf ${U.devName(src)} liegt, kommt auf diese Platte. Im User-Share bleibt es am selben Ort sichtbar.`)}</div>
          <p class="u-plan" id="u-plan"></p></div>`,
      foot: [{ start: 'Läuft als Aufgabe im Hintergrund' }, cancel, { label: `Sicher ${verb.toLowerCase()}`, g: 'IconShieldOn', primary: true, run: () => go() }],
    });
    const plan = () => {
      const way = segVal($('#u-way', w)), disk = segVal($('#u-disk', w));
      $('#u-diskrow', w).hidden = way !== 'disk';
      const nothing = way === 'share' && shareFrom === shareTo;
      $('#u-plan', w).textContent = way === 'disk'
        ? `${verb} von ${diskFrom} nach /mnt/${disk}${b.rel ? '/' + b.rel : ''}.`
        : nothing ? 'Über die User-Shares sind Quelle und Ziel derselbe Ordner. Die Dateien liegen also schon dort, es gibt nichts zu tun.'
          : `${verb} von ${shareFrom} nach ${shareTo}.`;
      bk.primary(w).disabled = nothing;
    };
    function go() {
      const way = segVal($('#u-way', w));
      if (way === 'share') return run(kind, items, shareTo, shareFrom, rule);
      const to = `/mnt/${segVal($('#u-disk', w))}${b.rel ? '/' + b.rel : ''}`;
      const names = new Set(items.map(k => k.name));
      const found = FS.kids(FS.nodeAt(diskFrom)).filter(k => names.has(k.name)).map(k => ({ ...k }));
      if (!found.length) { CC.toast(`Auf ${U.devName(src)} liegt davon nichts`, 'warn'); return false; }
      bk.mkdirs(to);
      run(kind, found, to, diskFrom, rule);
    }
    w.addEventListener('change', plan);
    plan();
  }

  function linkWindow(kind, items, dest, from, broken) {
    const verb = verbOf(kind), extra = broken.reduce((s, x) => s + x.n.size, 0), canLink = linkable(items, from, dest);
    return new Promise(resolve => {
      let answer = null;
      win({
        title: 'Harte Links', wide: true, cls: 'bk u-win',
        body: `<div class="well u-warnwell">${alert(kind === 'copy' ? 'Die Kopie ist kein harter Link' : 'Die harten Links gehen verloren',
            kind === 'copy' ? `Eine Kopie ist eine neue Datei und belegt ${fmtSize(extra)} noch einmal. Die anderen Pfade bleiben mit dem Original verknüpft.`
              : `Nach ${esc(dest)} geht es über eine Share- oder Pool-Grenze, dabei entsteht eine neue Datei. Die anderen Pfade behalten ihre Daten, danach liegen ${fmtSize(extra)} doppelt.`)}
            ${fileRows(broken, x => `<td class="mono">${esc(x.rel)}</td><td class="u-also">auch ${x.others.map(p => `<span class="mono">${esc(p)}</span>`).join(', ')}</td><td class="glim-num r">${fmtSize(x.n.size)}</td>`)}</div>
          ${canLink ? '' : '<p class="u-plan">Ein harter Link geht hier nicht, weil das Ziel auf einer anderen Platte oder in einem anderen Pool liegt.</p>'}`,
        foot: [cancel, ...(canLink ? [{ label: 'Als harten Link anlegen', g: 'IconLink', run: () => { answer = 'link'; } }] : []), { label: `Trotzdem ${verb.toLowerCase()}`, g: kind === 'copy' ? 'IconCopy' : 'IconMove', primary: true, run: () => { answer = 'go'; } }],
        onClose: () => resolve(answer),
      });
    });
  }

  const transfer = Files.transfer;
  function run(kind, items, dest, from, rule) {
    U.wake(picked(items, from).flatMap(([n]) => [n.dev, U.landsOn(dest, kind, n)]));
    return transfer(kind, items, dest, from, rule);
  }
  Files.transfer = async (kind, items, dest, from = Files.T().path, rule = 'ask') => {
    if (kind === 'link') return makeLinks(items, dest, from);
    const same = bk.isDir(dest) && dest !== from ? sameFiles(items, dest, from) : [];
    if (same.length) return sameFileWindow(kind, items, dest, from, rule, same);
    const broken = bk.isDir(dest) && dest !== from ? brokenLinks(kind, items, dest, from) : [];
    if (broken.length) {
      const answer = await linkWindow(kind, items, dest, from, broken);
      if (answer === 'link') return makeLinks(items, dest, from);
      if (answer !== 'go') return;
    }
    return run(kind, items, dest, from, rule);
  };

  CC.transferWays.push({ id: 'link', label: 'Als harten Link anlegen', tip: 'Ein harter Link ist ein zweiter Name für dieselbe Datei und belegt keinen Platz. Das geht nur, wenn Quelle und Ziel auf derselben Platte oder im selben Pool liegen.',
    fits: (kind, items, from, dest) => kind === 'copy' && linkable(items, from, dest) });

  // Eigenschaften shows the disk a file sits on and the other paths of its hard links.
  const props = CC.actions.props;
  const openProps = props.run;
  props.run = (...args) => {
    const dir = bk.here(), nodes = bk.selNodes(), out = openProps(...args);
    const w = $$('.scrim .bk-props').at(-1);
    const one = nodes.length === 1 ? nodes[0] : !nodes.length ? FS.nodeAt(dir) : null;
    if (!w || !one || !U.where(dir)) return out;
    const files = U.filesOf(one), devs = {};
    files.forEach(([k]) => { if (k.dev) devs[k.dev] = (devs[k.dev] || 0) + (k.size || 0); });
    const links = U.linksOf(one);
    const place = Object.keys(devs).length ? Object.entries(devs).map(([d, s]) => `<span class="pill ${d === 'cache' ? 'state' : ''} glim-num">${esc(U.devName(d))}${files.length > 1 ? `, ${fmtSize(s) || '0 B'}` : ''}</span>`).join('') : '<span class="muted">nichts</span>';
    const well = document.createElement('div');
    well.className = 'well bk-form u-where';
    well.innerHTML = `<div class="eyebrow">Speicherort</div>
      ${frow('Liegt auf', `<span class="bk-val u-pills">${place}</span>`, 'Die Platte oder der Pool, auf dem die Daten wirklich liegen. Der User-Share unter /mnt/user zeigt alle Platten zusammen.')}
      ${files.length === 1 && !one.dir ? frow('Harte Links', links.length ? `<span class="bk-val u-links">${links.map(p => `<span class="bk-pathv"><span class="bk-path mono" title="${esc(p)}"><bdi>${esc(p)}</bdi></span>${bk.copyBtn(p, 'Pfad kopieren')}</span>`).join('')}</span>` : '<span class="bk-val">keine, nur dieser Pfad</span>',
        'Dieselbe Datei liegt auch unter diesen Pfaden. Sie belegt den Platz nur einmal. Kopieren oder Verschieben über eine Share-Grenze macht daraus eigene Dateien.') : ''}`;
    $('.bk-top', w).after(well);
    return out;
  };

  CC.css(`
.u-win .win-body { gap: .8rem; }
.u-warnwell { box-shadow: inset 0 0 0 1px color-mix(in srgb, #FF832B 55%, transparent); }
.u-alert { display: grid; grid-template-columns: 1.4rem minmax(0, 1fr); gap: .7rem; align-items: start; }
.u-alert > svg { width: 1.4rem; height: 1.4rem; color: #FF832B; }
.u-alert b { font-size: 15px; }
.u-alert p { margin: .25rem 0 0; color: var(--carbon-text-sub); line-height: 1.45; }
.u-tbl td { font-size: 13px; }
.u-tbl td.mono { font-size: 12.5px; }
.u-also { color: var(--carbon-text-muted); font-size: 12px; }
.u-plan { margin: 0; color: var(--carbon-text-sub); font-size: 13px; padding: 0 .1rem; }
.u-pills { display: flex; gap: .3rem; justify-content: flex-end; flex-wrap: wrap; }
.u-links { display: flex; flex-direction: column; align-items: flex-end; gap: .1rem; max-width: 70%; }
.u-links .bk-pathv { max-width: 100%; }
`);
})();
