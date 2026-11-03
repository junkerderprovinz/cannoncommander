// The list follows changes other programs make on the server: new files appear on their own and glow for a moment.
// While a window, a menu or a selection is open the refresh waits, so nothing moves under the pointer.
(() => {
  const { sw, frow } = CC.ui;
  const fresh = new Map();
  let pending = false, seen = [];

  const on = () => CC.prefs.liveRefresh !== false;
  const panels = () => (CC.prefs.dual ? [0, 1] : [0]);
  const shows = (dir) => panels().some(i => {
    const t = Files.T(i);
    return t.path === dir || ((t.flat || (t.filter && t.deep !== false)) && dir.startsWith(t.path + '/'));
  });
  const busy = () => winStack.length || $('#menu, .qdrop, .cpick, .a-cpop') || panels().some(i => Files.T(i).marked.size)
    || document.activeElement?.matches('input, textarea');

  // The cursor stays on the file it was on at the last drawing, even when a new file lands above it.
  function refresh() {
    pending = false;
    panels().forEach((i, n) => { const k = Files.entries(i).findIndex(x => x.name === seen[n]); if (k >= 0) Files.T(i).cursor = k; });
    CC.render();
  }
  CC.live = {
    changed(dir, names) {
      names.forEach(n => fresh.set(FS.join(dir, n), Date.now()));
      if (!on() || CC.view !== 'files' || !shows(dir)) return;
      if (busy()) pending = true; else refresh();
    },
  };
  setInterval(() => { if (pending && CC.view === 'files' && !busy()) refresh(); }, 700);

  const render = CC.views.files.render;
  CC.views.files.render = (host) => {
    render(host);
    seen = panels().map(i => Files.entries(i)[Files.T(i).cursor]?.name);
    const now = Date.now();
    for (const [p, t] of fresh) if (now - t > 2500) fresh.delete(p);
    if (!fresh.size) return;
    $$('.panel[data-panel] [data-idx]', host).forEach(row => {
      const i = +row.closest('.panel').dataset.panel, k = Files.entries(i)[+row.dataset.idx];
      if (k && fresh.has(FS.join(k.at || Files.T(i).path, k.name))) row.classList.add('fresh');
    });
  };

  // Only in the prototype: a download client writes into downloads, a part file grows and then gets its real name.
  const STEPS = ['Cosmos Laundromat (2015)', 'Spring (2019)', 'Agent 327 (2017)'];
  let demo = null, step = 0;
  function tick() {
    const dir = '/mnt/user/downloads', n = FS.nodeAt(dir), film = STEPS[Math.floor(step / 3) % STEPS.length], phase = step % 3;
    const part = `${film}.mkv.part`, done = `${film}.mkv`;
    if (phase === 0) n.children.push({ name: part, size: 300 * MB, time: Date.now(), owner: 'nobody', group: 'users', mode: '0664' });
    if (phase === 1) { const k = FS.kids(n).find(x => x.name === part); if (k) { k.size = 900 * MB; k.time = Date.now(); } }
    if (phase === 2) { const k = FS.kids(n).find(x => x.name === part); if (k) { k.name = FS.kids(n).some(x => x.name === done) ? Files.freeName(n, done) : done; k.size = 1.6 * GB; k.time = Date.now(); } }
    step += 1;
    CC.live.changed(dir, [phase === 2 ? done : part]);
  }
  CC.live.demo = (run) => { clearInterval(demo); demo = run ? setInterval(tick, 4000) : null; if (run) tick(); };

  const tab = CC.settingsTabs.find(t => t.id === 'files');
  const draw = tab.render;
  tab.render = (host) => {
    draw(host);
    host.insertAdjacentHTML('beforeend', CC.card('Aktualisieren',
      frow('Liste von selbst aktualisieren', sw(on(), 'data-pref="liveRefresh"'), 'Ändern andere Programme Dateien im offenen Ordner, etwa ein Download-Client, erscheinen sie ohne Neu laden und leuchten kurz auf. Ist ein Fenster offen oder etwas markiert, wartet die Liste, bis du fertig bist.')
      + frow('Download-Client vorführen', sw(!!demo, 'id="f-live-demo"'), 'Nur im Prototyp: alle vier Sekunden schreibt ein gespielter Download-Client in downloads.')));
    $('#f-live-demo', host).addEventListener('change', e => CC.live.demo(swVal(e.currentTarget)));
  };

  CC.css(`
.row.fresh { animation: cc-fresh 2.2s ease-out; }
@keyframes cc-fresh { 0%, 35% { background: color-mix(in srgb, var(--accent) 26%, transparent); } }
@media (prefers-reduced-motion: reduce) { .row.fresh { animation: none; box-shadow: inset 3px 0 0 var(--accent); } }
`);
})();
