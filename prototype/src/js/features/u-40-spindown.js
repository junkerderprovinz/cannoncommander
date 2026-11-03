// Sleeping disks: a moon in the folder tree, and a word before something wakes one. Listing a folder comes from the index.
(() => {
  const U = CC.unraid, bk = CC.bk;

  const baseSide = renderSide;
  renderSide = () => {
    baseSide();
    for (const d of U.DISKS) {
      const row = U.spin[d] === false && $(`#tree [data-go="/mnt/${d}"]`);
      if (!row) continue;
      const moon = document.createElement('i');
      moon.className = 'u-moon';
      moon.dataset.tip = `${U.devName(d)} schläft. Ordner zeigt CastleCommander aus dem Index, erst eine Datei zu öffnen weckt sie.`;
      moon.innerHTML = g('IconMoon');
      row.querySelector('.meta').before(moon);
    }
  };

  // Reading a file needs the disk it is on; the selection's disks wake before the viewer or editor opens.
  for (const id of ['view', 'edit', 'quickLook', 'download', 'checksum']) {
    const a = CC.actions[id];
    if (!a) continue;
    const open = a.run;
    a.run = (...args) => {
      if (CC.view === 'files') U.wake(Files.sel().flatMap(k => { const n = bk.nodeIn(bk.here(), k.name); return n ? U.filesOf(n).map(([f]) => U.devOf(f, bk.here())) : []; }));
      return open(...args);
    };
  }

  CC.action('spinDown', { label: 'Platten schlafen legen', g: 'IconSleep', when: () => U.DISKS.some(d => U.spin[d]), run: () => {
    const awake = U.DISKS.filter(d => U.spin[d]);
    awake.forEach(d => { U.spin[d] = false; });
    CC.render();
    CC.toast(`${U.names(awake)} ${awake.length === 1 ? 'schläft' : 'schlafen'} jetzt`);
  } });
  const tools = MENUS.find(m => m[0] === 'Werkzeuge')[1];
  tools.splice(tools.indexOf('mounts') + 1, 0, 'spinDown');

  CC.css(`
.nav-row .u-moon { display: inline-grid; place-items: center; margin-left: auto; color: var(--carbon-text-muted); }
.nav-row .u-moon svg { width: .85rem; height: .85rem; }
.nav-row .u-moon + .meta { margin-left: .35rem; }
.nav-row.cur .u-moon { color: var(--accent-contrast); }
`);
})();
