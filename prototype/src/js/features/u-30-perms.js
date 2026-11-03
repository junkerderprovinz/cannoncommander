// Rechte wie Unraid setzen: nobody:users with 0777 and 0666 as Docker Safe New Perms does, leaving appdata and the system alone.
(() => {
  const { frow } = CC.ui;
  const bk = CC.bk;

  // Containers and VMs keep their own owners in these folders; resetting them breaks databases and running guests.
  const KEEP = { appdata: 'Container-Daten', system: 'Docker und VMs', domains: 'VM-Abbilder' };
  function kept(path) {
    const p = FS.split(path);
    if (p[0] === 'boot') return 'USB-Stick';
    return p[0] === 'mnt' && p.length >= 3 ? KEEP[p[2]] || null : null;
  }

  // Walks the folder like the real script: every kept folder is skipped whole, everything else is counted.
  function survey(path) {
    const skipped = [];
    let files = 0, dirs = 0;
    const walk = (n, p) => {
      for (const k of FS.kids(n)) {
        const q = FS.join(p, k.name), why = kept(q);
        if (why) { skipped.push({ path: q, why, n: k }); continue; }
        if (k.dir && !k.link) { dirs++; walk(k, q); } else files++;
      }
    };
    walk(FS.nodeAt(path), path);
    return { skipped, files, dirs };
  }

  function apply(path) {
    let n = 0;
    const set = (k) => { Object.assign(k, { owner: 'nobody', group: 'users', mode: k.dir && !k.link ? '0777' : '0666' }); n++; };
    const walk = (node, p) => {
      for (const k of FS.kids(node)) {
        const q = FS.join(p, k.name);
        if (kept(q)) continue;
        set(k);
        if (k.dir && !k.link) walk(k, q);
      }
    };
    const top = FS.nodeAt(path);
    if (path !== '/') set(top);
    walk(top, path);
    return n;
  }

  CC.action('fixPerms', { label: 'Rechte wie Unraid setzen', g: 'IconShieldOn', run: () => {
    if (CC.view !== 'files') CC.go('files');
    const sel = Files.sel();
    const path = sel.length === 1 && sel[0].dir && !sel[0].link ? FS.join(bk.here(), sel[0].name) : bk.here();
    const own = kept(path), arch = FS.inArchive(path);
    const { skipped, files, dirs } = own || arch ? { skipped: [], files: 0, dirs: 0 } : survey(path);
    const name = FS.LABEL[path] || FS.split(path).pop() || 'Server';
    win({
      title: 'Rechte wie Unraid setzen', wide: true, cls: 'bk u-win',
      body: own || arch ? `<div class="well u-warnwell"><div class="u-alert">${g('IconShieldOn')}<div><b>Hier setzt CannonCommander keine Rechte</b><p>${arch ? 'Im Archiv lässt sich nichts ändern.' : `${esc(path)} gehört zu ${esc(own)}. Dort brauchen Container, VMs oder der Stick ihre eigenen Besitzer und Rechte.`}</p></div></div></div>`
        : `<div class="well bk-form"><div class="eyebrow">Ordner</div>
            ${frow('Ordner', `<span class="bk-val bk-pathv"><span class="bk-path mono" title="${esc(path)}"><bdi>${esc(path)}</bdi></span></span>`)}
            ${frow('Besitzer', '<span class="bk-val mono">nobody:users (99:100)</span>', 'So gehören Dateien auf Unraid den Freigaben, Docker-Containern und dem Netz gemeinsam.')}
            ${frow('Rechte', '<span class="bk-val mono">Ordner 0777, Dateien 0666</span>', 'Alle dürfen lesen und schreiben, Ordner lassen sich öffnen. Ausführbar ist danach keine Datei mehr.')}
            ${frow('Betrifft', `<span class="bk-val glim-num">${bk.plural(files, 'Datei', 'Dateien')} in ${bk.plural(dirs + 1, 'Ordner', 'Ordnern')}</span>`)}</div>
          <div class="well u-warnwell"><div class="u-alert">${g('IconWarning')}<div><b>Das lässt sich nicht rückgängig machen</b><p>Alles in ${esc(name)} bekommt diese Rechte, auch Dateien, die du bewusst anders eingestellt hast. ${skipped.length ? 'Diese Ordner bleiben, wie sie sind:' : 'Hier liegt nichts, das ausgelassen werden muss.'}</p></div></div>
            ${skipped.length ? `<table class="tbl u-tbl">${skipped.map(s => `<tr><td class="mono">${esc(s.path)}</td><td><span class="pill">${esc(s.why)}</span></td><td class="glim-num r">${fmtSize(FS.totalSize(s.n)) || '0 B'}</td></tr>`).join('')}</table>` : ''}</div>`,
      foot: own || arch ? [CC.ui.closeBtn] : [{ start: 'Läuft als Aufgabe im Hintergrund' }, CC.ui.cancel, { label: 'Rechte setzen', g: 'IconShieldOn', primary: true, run: () => start() }],
    });
    function start() {
      Jobs.start({ kind: 'perms', label: `Rechte setzen: ${name}`, total: 0, speed: files > 200 ? 0.6 : 1.4, done: () => {
        const n = apply(path);
        bk.log('Rechte gesetzt', `nobody:users, 0777/0666 für ${bk.plural(n, 'Objekt', 'Objekte')} in ${path}`);
        CC.toast(`Rechte gesetzt für ${bk.plural(n, 'Objekt', 'Objekte')}${skipped.length ? `, ${bk.plural(skipped.length, 'Ordner', 'Ordner')} ausgelassen` : ''}`);
        if (CC.view === 'files') CC.render();
      } });
      CC.toast(`Rechte setzen in ${name} läuft`, 'ok', { label: 'Aufgaben', run: () => CC.run('jobs') });
    }
  } });
  const tools = MENUS.find(m => m[0] === 'Werkzeuge')[1];
  tools.splice(tools.indexOf('diskUsage') + 1, 0, 'fixPerms');
})();
