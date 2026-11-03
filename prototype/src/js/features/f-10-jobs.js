// Jobs that behave well on a server: a speed limit, waiting while Unraid checks parity, and a message when a long one ends.
(() => {
  const { sw, seg, frow } = CC.ui;
  const BIG = 2 * GB;
  const CAPS = [[0, 'Ohne'], [25, '25 MB/s'], [50, '50 MB/s'], [100, '100 MB/s'], [200, '200 MB/s']];
  const mbs = (n) => `${n} MB/s`;
  const took = (s) => s < 60 ? `${Math.round(s)} s` : s < 3600 ? `${Math.round(s / 60)} Min.` : `${(s / 3600).toFixed(1).replace('.', ',')} Std.`;

  // The prototype has no array, so a parity check is played: it runs for a while and then lets the waiting jobs go.
  const Parity = { running: false, pct: 0, timer: null };
  const resume = () => { Jobs.timer ||= setInterval(Jobs.tick, TICK); Jobs.tick(false); };
  Parity.start = () => {
    if (Parity.running) return;
    Object.assign(Parity, { running: true, pct: 0 });
    Parity.timer = setInterval(() => {
      Parity.pct = Math.min(100, Parity.pct + 2);
      if (Parity.pct >= 100) Parity.stop(true);
      else if ($('#tree')) renderSide();
    }, 1000);
    if ($('#tree')) renderSide();
    resume();
  };
  Parity.stop = (finished) => {
    clearInterval(Parity.timer);
    Object.assign(Parity, { running: false, timer: null });
    if (finished) CC.toast('Die Paritätsprüfung ist fertig, wartende Aufgaben laufen jetzt weiter');
    if ($('#tree')) renderSide();
    if (CC.view === 'settings') CC.render();
    resume();
  };
  CC.parity = Parity;

  const waits = (j) => CC.prefs.parityWait !== false && Parity.running && ['copy', 'move'].includes(j.kind) && (j.parity || j.total >= BIG);
  Jobs.held = waits;
  // Each job keeps to its own limit, and the limit for all of them is shared out between the jobs that move.
  Jobs.limit = (j, moving) => {
    const own = j.cap ? j.cap * MB : Infinity, all = CC.prefs.speedCap ? CC.prefs.speedCap * MB / Math.max(1, moving) : Infinity;
    return Math.min(own, all);
  };

  let holdNext = false;
  const start = Jobs.start;
  Jobs.start = (o) => { const j = start(o); if (holdNext) { j.parity = true; holdNext = false; Jobs.tick(false); } return j; };

  function askParity(total) {
    return new Promise(resolve => {
      let answer = null;
      win({
        title: 'Paritätsprüfung läuft', cls: 'xwin',
        body: `<div class="xdel"><span class="xbin">${g('IconShieldOn')}</span><div><b>Jetzt kopieren oder danach?</b><span class="glim-num">Unraid prüft gerade die Parität, ${Parity.pct} % sind geschafft. Eine Kopie von ${fmtSize(total) || '0 B'} bremst die Prüfung und läuft selbst langsamer.</span></div></div>`,
        foot: [{ label: 'Danach', g: 'IconQueued', run: () => { answer = 'later'; } }, { label: 'Jetzt', g: 'IconPlay', primary: true, run: () => { answer = 'now'; } }],
        onClose: () => resolve(answer),
      });
    });
  }
  const transfer = Files.transfer;
  Files.transfer = async (kind, items, dest, from, rule) => {
    if (['copy', 'move'].includes(kind) && Parity.running && CC.prefs.parityWait !== false) {
      const total = items.reduce((a, k) => a + FS.totalSize(k), 0);
      if (total < BIG) {
        const answer = await askParity(total);
        if (!answer) return;
        holdNext = answer === 'later';
      } else CC.toast('Die Aufgabe wartet, bis die Paritätsprüfung fertig ist', 'warn');
    }
    try { return await transfer(kind, items, dest, from, rule); } finally { holdNext = false; }
  };

  // The jobs window names the reason a job waits and offers a speed limit per job.
  const line = jobLine;
  jobLine = (j, place) => {
    let html = line(j, place);
    if (waits(j) && j.state === 'wartet') html = html.replace(/wartet, Platz \d+/, 'wartet auf die Paritätsprüfung');
    const cap = j.cap || CC.prefs.speedCap;
    if (cap && j.state === 'läuft') html = html.replace(/(<div class="sub glim-num">)([^<]*)/, (m, a, b) => `${a}${b} · höchstens ${mbs(cap)}`);
    if (['läuft', 'wartet', 'pausiert'].includes(j.state)) html = html.replace('<span class="job-acts">', `<span class="job-acts"><button class="btn icon ${j.cap ? 'on' : ''}" data-jact="limit" data-tip="Tempo begrenzen" aria-label="Tempo begrenzen">${g('IconSliders')}</button>`);
    return html;
  };
  document.addEventListener('click', e => {
    const b = e.target.closest('[data-jact="limit"]');
    if (!b) return;
    e.stopImmediatePropagation();
    const j = Jobs.list.find(x => String(x.id) === b.closest('[data-job]').dataset.job);
    const r = b.getBoundingClientRect();
    menu(r.left, r.bottom + 6, [{ head: 'Tempo dieser Aufgabe' }, ...CAPS.map(([v, l]) => ({ label: l, g: v ? 'IconSliders' : 'IconBolt', radio: true, checked: () => (j.cap || 0) === v, run: () => { j.cap = v; Jobs.tick(false); } }))]);
  }, true);

  Files.sideRows.push(() => Parity.running ? `<div class="nav-row" data-parity data-tip="Läuft auf dem Server, große Aufgaben warten so lange">${g('IconShieldOn')}<span>Paritätsprüfung</span><span class="meta glim-num">${Parity.pct} %</span></div>` : '');
  document.addEventListener('click', e => {
    const row = e.target.closest('[data-parity]');
    if (!row) return;
    const r = row.getBoundingClientRect();
    menu(r.left + 12, r.bottom, [{ label: 'Vorführung beenden', g: 'IconStop', run: () => Parity.stop(false) }]);
  });

  // A finished job is reported once, if it ran long enough to have been forgotten.
  const told = new Set();
  const say = (title, body) => {
    if (CC.prefs.notifyDone && 'Notification' in window && Notification.permission === 'granted') new Notification(title, { body, tag: 'cannoncommander-job' });
    if (CC.prefs.notifyUnraid) DATA.activity.unshift({ t: Date.now(), who: 'Unraid', what: 'benachrichtigt', obj: body });
  };
  Jobs.listeners.push(() => {
    for (const j of Jobs.list) {
      if (told.has(j.id) || !['fertig', 'fehlgeschlagen'].includes(j.state)) continue;
      told.add(j.id);
      const secs = (Date.now() - j.started) / 1000 * LAPSE;
      if ((!CC.prefs.notifyDone && !CC.prefs.notifyUnraid) || secs < (CC.prefs.notifyAfter ?? 60)) continue;
      say(j.state === 'fertig' ? 'Aufgabe fertig' : 'Aufgabe fehlgeschlagen', `${j.label}, nach ${took(secs)}`);
    }
  });

  async function allowNotes() {
    if (!('Notification' in window)) { CC.toast('Dieser Browser kann keine Benachrichtigungen zeigen', 'warn'); return false; }
    if (Notification.permission === 'default') await Notification.requestPermission();
    if (Notification.permission === 'granted') return true;
    CC.toast('Der Browser lässt keine Benachrichtigungen zu. Erlauben kannst du sie in den Seiteneinstellungen des Browsers', 'warn');
    return false;
  }

  const general = CC.settingsTabs.find(t => t.id === 'general');
  const render = general.render;
  general.render = (host) => {
    render(host);
    const p = CC.prefs;
    const card = $$('.card', host).find(c => $('.notch', c)?.textContent === 'Aufgaben');
    if (!card) return;
    $('.card-body', card).insertAdjacentHTML('beforeend',
      frow('Tempo für alle zusammen', seg(CAPS, p.speedCap || 0, 'data-pref="speedCap" data-num="1"'), 'Alle laufenden Aufgaben teilen sich dieses Tempo, damit Streams und Downloads flüssig bleiben. Eine einzelne Aufgabe bremst du im Aufgabenfenster.')
      + frow('Während der Paritätsprüfung warten', sw(p.parityWait !== false, 'data-pref="parityWait"'), 'Große Kopien ab 2 GB warten, bis Unraid mit der Prüfung oder einem Rebuild fertig ist. Bei kleinen fragt CannonCommander vorher. Beides teilt sich sonst dieselben Platten.')
      + frow('Paritätsprüfung vorführen', sw(Parity.running, 'id="f-parity"'), 'Nur im Prototyp: spielt eine Paritätsprüfung ab, damit du siehst, wie Aufgaben darauf warten.')
      + frow('Im Browser melden, wenn fertig', sw(p.notifyDone, 'data-pref="notifyDone"'), 'Endet eine lange Aufgabe oder schlägt sie fehl, zeigt der Browser eine Benachrichtigung, auch wenn der Tab im Hintergrund liegt.')
      + frow('Auch an Unraid melden', sw(p.notifyUnraid, 'data-pref="notifyUnraid"'), 'Schickt die Meldung zusätzlich an die Unraid-Benachrichtigungen. Über deren Agenten landet sie auf dem Handy, etwa per Pushover, Discord oder Telegram.')
      + (p.notifyDone || p.notifyUnraid ? frow('Melden ab', seg([[0, 'Immer'], [60, '1 Min.'], [600, '10 Min.'], [3600, '1 Std.']], p.notifyAfter ?? 60, 'data-pref="notifyAfter" data-num="1"'), 'Kurze Aufgaben meldet CannonCommander nicht, die siehst du ja noch laufen.')
        + frow('Ausprobieren', `<button class="btn" id="f-notify-test">${g('IconSendTest')}Test senden</button>`) : ''));
    $('#f-parity', host).addEventListener('change', e => (swVal(e.currentTarget) ? Parity.start() : Parity.stop(false)));
    $('[data-pref="notifyDone"]', host).addEventListener('change', async e => {
      if (swVal(e.currentTarget) && !(await allowNotes())) CC.admin.setPref('notifyDone', false);
    });
    $('#f-notify-test', host)?.addEventListener('click', async () => {
      if (p.notifyDone && !(await allowNotes())) return;
      say('CannonCommander', 'So sieht die Meldung aus, wenn eine Aufgabe fertig ist');
      CC.toast(p.notifyUnraid ? 'Testmeldung an den Browser und an Unraid geschickt' : 'Testmeldung geschickt');
    });
  };

  CC.css(`.job-acts .btn.on svg { color: var(--accent-glyph); }`);
})();
