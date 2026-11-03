// Cache or array at a glance: the share's storage and mover in the panel foot, a badge on files still on the cache,
// and a view that shows only those.
(() => {
  const U = CC.unraid;
  const STORE = { cache: 'Cache', array: 'Array' };

  function storeBadge(path) {
    const w = U.where(path);
    if (!w || !w.share) return '';
    if (w.vol !== 'user') {
      const asleep = U.spin[w.vol] === false;
      return `<span class="pill u-store" data-tip="Du bist direkt auf ${esc(U.devName(w.vol))}. Kopiere oder verschiebe nicht zwischen hier und /mnt/user, sonst kann Unraid Dateien auf 0 Byte kürzen.${asleep ? ' Die Platte schläft, die Liste kommt aus dem Index.' : ''}">${g(asleep ? 'IconMoon' : 'IconDatabase')}Disk-Share</span>`;
    }
    const s = U.shareOf(w.share), first = STORE[s.primary], second = s.secondary && s.disks.length ? STORE[s.secondary] : '';
    const label = !second ? `nur ${first}` : s.mover === 'up' ? 'Array → Cache' : 'Cache → Array';
    const disks = U.names(s.disks);
    const waiting = U.filesOf(FS.nodeAt(path)).filter(([k]) => k.dev === 'cache');
    const tip = `Share ${w.share}: Primärspeicher ${first}${second ? `, Sekundärspeicher ${second}` : ', kein Sekundärspeicher'}. `
      + (!second ? (s.primary === 'cache' ? 'Alles bleibt auf dem Cache.' : `Neue Dateien landen direkt auf ${disks}.`)
        : s.mover === 'up' ? `Neue Dateien landen auf dem Cache, der Mover holt Reste vom Array (${disks}) dorthin.`
          : `Neue Dateien landen auf dem Cache, der Mover schiebt sie um 3:40 Uhr auf ${disks}.`)
      + (second && s.mover === 'down' && waiting.length ? ` Hier liegen noch ${waiting.length === 1 ? 'eine Datei' : `${waiting.length} Dateien`} mit ${fmtSize(waiting.reduce((a, [k]) => a + (k.size || 0), 0)) || '0 B'} auf dem Cache.` : '');
    return `<span class="pill u-store" data-tip="${esc(tip)}">${g(s.primary === 'cache' ? 'IconBolt' : 'IconDatabase')}${esc(label)}</span>`;
  }

  // Only where the mover will still carry a file to the array does "on the cache" say something.
  const waits = (share, k) => U.shareOf(share).mover === 'down' && U.filesOf(k).some(([f]) => f.dev === 'cache');

  const baseFooter = footer;
  footer = (i, list) => {
    const t = Files.T(i), filter = t.cacheOnly ? '<button class="pill state u-filter" data-run="cacheOnly" data-tip="Filter ausschalten">Nur Cache' + g('IconClose') + '</button>' : '';
    return baseFooter(i, list).replace('<span class="grow"></span>', `${storeBadge(t.path)}${filter}<span class="grow"></span>`);
  };

  const baseName = nameCell;
  nameCell = (i, k) => {
    const html = baseName(i, k), w = U.where(k.at || Files.T(i).path);
    if (w?.vol !== 'user' || !waits(w.share || k.name, k)) return html;
    return html.slice(0, -'</span>'.length) + `<span class="pill u-cache" data-tip="${k.dir ? 'Darin liegt noch etwas auf dem Cache' : 'Liegt noch auf dem Cache, der Mover bringt die Datei aufs Array'}">Cache</span></span>`;
  };

  const baseEntries = Files.entries;
  Files.entries = (i = Files.active) => {
    const list = baseEntries(i);
    return Files.T(i).cacheOnly ? list.filter(k => k.up || U.onCache(k)) : list;
  };

  CC.action('cacheOnly', { label: 'Nur auf dem Cache', g: 'IconFilter', checked: () => !!Files.T().cacheOnly, run: () => {
    const t = Files.T();
    t.cacheOnly = !t.cacheOnly;
    t.cursor = 0;
    CC.render();
    if (t.cacheOnly) CC.toast(`${Files.entries().length} auf dem Cache`);
  } });
  const view = MENUS.find(m => m[0] === 'Ansicht')[1];
  view.splice(view.indexOf('syncBrowse') + 1, 0, 'cacheOnly');

  CC.css(`
.lfoot .u-store, .lfoot .u-filter { gap: .3rem; margin-left: .6rem; cursor: default; }
.lfoot .u-store svg, .lfoot .u-filter svg { width: .85rem; height: .85rem; }
.lfoot .u-filter { cursor: pointer; }
.lfoot .u-filter:hover { background: var(--carbon-hover-raised); }
.name .u-cache { height: 1.1rem; padding: 0 .4rem; font-size: 11px; margin-left: .4rem; background: color-mix(in srgb, var(--carbon-surface2), var(--accent) 14%); }
`);
})();
