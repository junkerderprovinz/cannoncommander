// Hochladen, Herunterladen and Freigeben. Uploads are simulated in chunks; finished files land in the tree.
(() => {
  const { real, plural, rng, viewKind } = CC.ck;
  const nodesOf = (folder) => Files.sel().map(k => real(folder, k.name) || k);

  const Up = { items: [], job: null, timer: null, win: null, paused: false };
  let upId = 0;

  function addFiles(files, path) {
    const dir = FS.nodeAt(path);
    if (!dir || !dir.dir || FS.inArchive(path)) return CC.toast('In diesen Ordner lässt sich nichts hochladen', 'warn');
    const list = [...files].filter(f => f.size || f.name);
    if (!list.length) return;
    const taken = (name) => FS.kids(dir).some(k => k.name === name);
    const clashes = list.filter(f => taken(f.name)).map(f => ({ name: f.name, size: f.size, time: f.lastModified || Date.now() }));
    const go = (decide) => {
      if (!decide) return;
      const fresh = list.filter(f => !taken(f.name) || (decide[f.name] || 'replace') !== 'skip');
      if (!fresh.length) return;
      ensureJob();
      for (const f of fresh) Up.items.push({ id: ++upId, batch: Up.job.id, file: f, name: f.name, size: f.size, path, loaded: 0, state: 'wartet', choice: taken(f.name) ? decide[f.name] || 'replace' : 'new' });
      paintUp();
    };
    if (clashes.length && CC.resolveConflicts) CC.resolveConflicts(clashes, path).then(go);
    else go({});
  }

  function ensureJob() {
    if (!Up.job || !['läuft', 'pausiert'].includes(Up.job.state)) {
      Up.job = { id: Date.now() + Math.random(), kind: 'upload', label: '', total: 0, pct: 0, state: 'läuft', started: Date.now() };
      Jobs.list.push(Up.job);
    }
    if (!Up.timer) Up.timer = setInterval(tick, 200);
  }
  function tick() {
    const job = Up.job, items = Up.items.filter(i => i.batch === job.id);
    if (job.state === 'abgebrochen') items.forEach(i => { if (i.state !== 'fertig') i.state = 'abgebrochen'; });
    Up.paused = job.state === 'pausiert';
    if (!Up.paused) {
      const running = items.filter(i => i.state === 'läuft');
      for (const i of items) if (running.length < 2 && i.state === 'wartet') { i.state = 'läuft'; running.push(i); }
      for (const i of running) {
        i.loaded = Math.min(i.size, i.loaded + Math.max(i.size / 22, 2.5 * MB) * (.6 + Math.random() * .8));
        if (i.loaded >= i.size) land(i);
      }
    }
    const live = items.filter(i => i.state !== 'abgebrochen');
    const total = live.reduce((a, i) => a + i.size, 0), done = live.reduce((a, i) => a + i.loaded, 0);
    const path = items[0]?.path || '/';
    Object.assign(job, { total, pct: total ? done / total * 100 : 100, label: `Hochladen: ${items.length === 1 ? items[0].name : plural(items.length, 'Datei', 'Dateien')} nach ${FS.split(path).pop() || 'Server'}` });
    if (!items.some(i => ['wartet', 'läuft', 'pausiert'].includes(i.state))) {
      clearInterval(Up.timer);
      Up.timer = null;
      if (job.state !== 'abgebrochen') {
        job.state = 'fertig';
        job.pct = 100;
        const n = items.filter(i => i.state === 'fertig').length;
        if (n) CC.toast(`${plural(n, 'Datei', 'Dateien')} hochgeladen nach ${path}`, 'ok', Up.win?.isConnected ? null : { label: 'Hingehen', run: () => Files.go(path) });
      }
    }
    Jobs.changed();
    paintUp();
  }
  // A finished file becomes a node; small text files and pictures keep their real content for the viewer.
  function land(i) {
    i.state = 'fertig';
    const dir = FS.nodeAt(i.path);
    let name = i.name;
    if (i.choice === 'both') name = Files.freeName(dir, name);
    else dir.children = dir.children.filter(k => k.name !== name);
    const node = { name, size: i.size, time: Date.now(), owner: 'nobody', group: 'users', mode: '0664' };
    const kind = viewKind(node);
    if (i.file instanceof File) {
      if (['text', 'code', 'md'].includes(kind) && i.size < 512 * KB) i.file.text().then(t => { node.text = t; });
      if (kind === 'img') node.url = URL.createObjectURL(i.file);
    }
    dir.children.push(node);
    DATA.activity.unshift({ t: Date.now(), who: CC.user.name, what: 'hochgeladen', obj: `${name} nach ${i.path}` });
    if (CC.view === 'files') CC.render();
  }

  const stateOf = (i) => i.state === 'läuft' ? `${Math.floor(i.loaded / i.size * 100)} %` : i.state === 'pausiert' ? 'angehalten' : i.state;
  function paintUp() {
    const w = Up.win;
    if (!w?.isConnected) return;
    const items = Up.items.slice().reverse();
    const live = items.filter(i => i.state !== 'abgebrochen');
    const total = live.reduce((a, i) => a + i.size, 0), done = live.reduce((a, i) => a + i.loaded, 0);
    const nDone = live.filter(i => i.state === 'fertig').length;
    const running = items.some(i => ['wartet', 'läuft', 'pausiert'].includes(i.state));
    $('#cu-queue', w).innerHTML = `<div class="eyebrow">Warteschlange ${info('Große Dateien gehen in Stücken von 10 MB hoch. Reißt die Verbindung ab, setzt der Upload beim letzten fertigen Stück wieder an, auch nach einem Neuladen der Seite.')}
        ${running ? `<button class="btn icon" id="cu-all" data-tip="${Up.paused ? 'Alle fortsetzen' : 'Alle anhalten'}" aria-label="${Up.paused ? 'Alle fortsetzen' : 'Alle anhalten'}">${g(Up.paused ? 'IconPlay' : 'IconPause')}</button>` : ''}
        ${items.some(i => ['fertig', 'abgebrochen'].includes(i.state)) ? `<button class="btn icon" id="cu-clear" data-tip="Fertige entfernen" aria-label="Fertige entfernen">${g('IconEmpty')}</button>` : ''}</div>
      ${items.length ? `<div class="cu-total">${frow(`${nDone} von ${plural(live.length, 'Datei', 'Dateien')} fertig`, `<span class="pill glim-num">${fmtSize(done) || '0 B'} von ${fmtSize(total) || '0 B'}</span>`)}<div class="bar"><i style="width:${total ? done / total * 100 : 0}%"></i></div></div>
        <div class="cu-list">${items.map(i => `<div class="cu-row ${i.state === 'fertig' ? 'done' : ''}" data-id="${i.id}">
          <span class="tile">${g(FS.glyphOf({ name: i.name }))}</span><span class="t">${esc(i.name)}</span>
          ${i.state === 'fertig' ? `<span class="cu-ok"><i></i>fertig</span>` : i.state === 'abgebrochen' ? '<span class="pill">abgebrochen</span>' : `<span class="bar"><i style="width:${i.loaded / i.size * 100}%"></i></span><span class="pill glim-num">${stateOf(i)}</span>`}
          <span class="cu-acts">${['läuft', 'wartet', 'pausiert'].includes(i.state) ? `<button class="btn icon" data-up="pause" data-tip="${i.state === 'pausiert' ? 'Fortsetzen' : 'Anhalten'}" aria-label="${i.state === 'pausiert' ? 'Fortsetzen' : 'Anhalten'}">${g(i.state === 'pausiert' ? 'IconPlay' : 'IconPause')}</button><button class="btn icon" data-up="cancel" data-tip="Abbrechen" aria-label="Abbrechen">${g('IconStop')}</button>` : ''}</span>
        </div>`).join('')}</div>`
      : `<div class="cu-empty muted">Noch nichts in der Warteschlange</div>`}`;
  }

  function uploadWindow(path) {
    if (Up.win?.isConnected) return Up.win;
    const w = Up.win = win({
      title: 'Hochladen', wide: true,
      body: `<button class="well cu-drop" id="cu-drop">${g('IconUpload')}<span class="t">Dateien hierher ziehen</span><span class="muted">oder klicken, um sie auszuwählen</span></button>
        <input type="file" id="cu-in" multiple hidden><input type="file" id="cu-din" webkitdirectory hidden>
        <div class="well">${frow('Ziel', `<span class="pill mono cu-path">${esc(path)}</span>`, 'Hochgeladene Dateien landen in dem Ordner, der im aktiven Fenster offen ist. Gleichnamige Dateien fragt CannonCommander vorher nach.')}</div>
        <div class="well" id="cu-queue"></div>`,
      foot: [{ start: 'Läuft weiter, auch wenn du das Fenster schließt' }, closeBtn, { label: 'Ordner wählen', g: 'IconFolder', run: (s) => { $('#cu-din', s).click(); return false; } }, { label: 'Dateien wählen', g: 'IconFile', primary: true, run: (s) => { $('#cu-in', s).click(); return false; } }],
      onClose: () => { Up.win = null; },
    });
    w.dataset.path = path;
    const drop = $('#cu-drop', w);
    drop.onclick = () => $('#cu-in', w).click();
    ['cu-in', 'cu-din'].forEach(id => $('#' + id, w).addEventListener('change', (e) => { addFiles(e.target.files, w.dataset.path); e.target.value = ''; }));
    drop.addEventListener('dragover', (e) => { e.preventDefault(); e.stopPropagation(); drop.classList.add('over'); });
    drop.addEventListener('dragleave', () => drop.classList.remove('over'));
    drop.addEventListener('drop', (e) => { e.preventDefault(); e.stopPropagation(); drop.classList.remove('over'); addFiles(e.dataTransfer.files, w.dataset.path); });
    w.addEventListener('click', (e) => {
      if (e.target.closest('#cu-all')) { if (Up.job) Jobs.pause(Up.job); Up.paused = Up.job?.state === 'pausiert'; paintUp(); }
      if (e.target.closest('#cu-clear')) { Up.items = Up.items.filter(i => !['fertig', 'abgebrochen'].includes(i.state)); paintUp(); }
      const b = e.target.closest('[data-up]');
      if (!b) return;
      const i = Up.items.find(x => x.id === +b.closest('[data-id]').dataset.id);
      if (b.dataset.up === 'pause') i.state = i.state === 'pausiert' ? 'wartet' : 'pausiert';
      else i.state = 'abgebrochen';
      paintUp();
    });
    paintUp();
    return w;
  }

  CC.action('upload', { label: 'Hochladen', g: 'IconUpload', needs: 'writable', run: () => uploadWindow(Files.T().path) });
  CC.uploadFiles = (files, path) => { uploadWindow(path); addFiles(files, path); };

  CC.action('download', { label: 'Herunterladen', g: 'IconDownload', needs: 'sel', run: () => download(nodesOf(Files.T().path), Files.T().path) });
  function download(items, folder) {
    if (items.length === 1 && !items[0].dir) return CC.toast(`${items[0].name} wird heruntergeladen, ${fmtSize(items[0].size)}`);
    const total = items.reduce((a, k) => a + FS.totalSize(k), 0);
    const base = (items.length === 1 ? items[0].name : FS.split(folder).pop() || 'dateien').replace(/::$/, '');
    const name = (fmt) => `${base}.${fmt === 'zip' ? 'zip' : 'tar.gz'}`;
    win({
      title: 'Herunterladen',
      body: `<div class="well"><div class="eyebrow">${plural(items.length, 'Objekt', 'Objekte')}, ${fmtSize(total)}</div>
          <table class="tbl">${items.slice(0, 6).map(k => `<tr><td>${esc(k.name)}</td><td class="glim-num r">${fmtSize(FS.totalSize(k)) || '0 B'}</td></tr>`).join('')}${items.length > 6 ? `<tr><td colspan="2">und ${items.length - 6} weitere</td></tr>` : ''}</table></div>
        <div class="well"><div class="eyebrow">Archiv</div>
          ${frow('Format', seg([['zip', 'ZIP'], ['tgz', 'TAR.GZ']], 'zip', 'id="dl-fmt"'), 'ZIP öffnet jedes Betriebssystem ohne Zusatzprogramm. TAR.GZ behält Rechte und Besitzer, das passt für Linux.')}
          ${frow('Dateiname', `<input class="field m" id="dl-name" value="${esc(name('zip'))}">`)}
          ${frow('Speichert in', '<span class="pill">Download-Ordner deines Browsers</span>', 'Der Server packt beim Senden, auf dem Server bleibt keine Datei zurück. Wohin der Browser speichert, stellst du in seinen Einstellungen ein.')}</div>`,
      foot: [cancel, { label: 'Herunterladen', g: 'IconDownload', primary: true, run: (w) => {
        const file = $('#dl-name', w).value.trim() || name(segVal($('#dl-fmt', w)));
        Jobs.start({ kind: 'pack', label: `Verpacken für den Download: ${file}`, total, speed: total > 10 * GB ? .5 : 1.4, done: () => CC.toast(`${file} liegt im Download-Ordner deines Browsers`) });
        CC.toast(`${file} wird gepackt und heruntergeladen`, 'ok', { label: 'Aufgaben', run: () => CC.run('jobs') });
      } }],
      onOpen: (w) => $('#dl-fmt', w).addEventListener('change', (e) => { $('#dl-name', w).value = name(segVal(e.currentTarget)); }),
    });
  }
  CC.ck.download = download;

  // Freigeben: one public link per file or folder, or one for a collection, which the other side sees as a folder.
  const HOST = 'https://dateien.example.org/s/';
  const EXPIRY = [['1', '1 Tag'], ['7', '7 Tage'], ['30', '30 Tage'], ['0', 'Unbegrenzt']];
  const LIMITS = [['0', 'Ohne'], ['1', '1'], ['5', '5'], ['10', '10'], ['25', '25']];
  const SLUG = /^[a-z0-9][a-z0-9-]{2,39}$/;
  let rolls = 0;
  const slugFor = (name) => {
    const r = rng(name + Date.now() + ++rolls);
    const id = Array.from({ length: 4 }, () => 'abcdefghjkmnpqrstuvwxyz23456789'[Math.floor(r() * 31)]).join('');
    return `${id}-${name.toLowerCase().replace(/\.[^.]+$/, '').normalize('NFD').replace(/[^\w]+/g, '-').replace(/^-|-$/g, '').slice(0, 18) || 'link'}`;
  };
  const secret = (id, ph) => `<span class="cs-secret"><input class="field w" type="password" id="${id}" placeholder="${ph}" autocomplete="new-password"><button class="cs-eye" type="button" data-eye data-tip="Anzeigen" aria-label="Passwort anzeigen">${g('IconEye')}</button></span>`;
  document.addEventListener('click', (e) => {
    const b = e.target.closest('[data-eye]');
    if (!b) return;
    const i = b.previousElementSibling, show = i.type === 'password';
    i.type = show ? 'text' : 'password';
    b.innerHTML = g(show ? 'IconEyeOff' : 'IconEye');
    b.dataset.tip = show ? 'Verbergen' : 'Anzeigen';
  });
  CC.ck.secret = secret;

  const taken = (slug) => DATA.shares.some(s => s.slug === slug);
  const freshSlug = (name) => { let s = slugFor(name); while (taken(s)) s = slugFor(name); return s; };

  // The options every new link has, for a single file as for a collection.
  const linkOptions = (name, dir) => `<div class="well"><div class="eyebrow">Neuer Link</div>
      ${frow('Adresse', `<span class="cs-slugf"><span class="muted mono cs-host">${esc(HOST.replace('https://', ''))}</span><input class="field mono" id="cs-slug" value="${esc(freshSlug(name))}" autocomplete="off" spellcheck="false" aria-label="Ende der Adresse"><button class="btn icon" id="cs-dice" type="button" data-tip="Link neu würfeln" aria-label="Link neu würfeln">${g('IconRetry')}</button></span>`, 'Das Ende der Adresse kannst du selbst schreiben, etwa urlaub-2026. Erlaubt sind Kleinbuchstaben, Ziffern und Bindestriche. Der Würfel macht eine neue Adresse, die sich schwer erraten lässt.')}
      ${frow('Nachricht an die Gegenseite', '<textarea class="field w" id="cs-msg" rows="2" maxlength="2000" placeholder="etwa: Hier sind die Fotos vom Wochenende"></textarea>', 'Steht auf der Seite des Links über den Dateien, etwa ein Gruß oder ein Hinweis. Darf leer bleiben.')}
      ${frow('Gültig', seg(EXPIRY, '7', 'id="cs-exp"'), 'Danach führt der Link ins Leere. Verlängern kannst du ihn unter Freigaben.')}
      ${frow('Download-Limit', seg(LIMITS, '0', 'id="cs-limit"'), 'Nach so vielen Downloads führt der Link ins Leere. Ansehen im Browser zählt nicht mit.')}
      ${frow('Mit Passwort', sw(false, 'id="cs-pw"'))}
      <div id="cs-pwrow" hidden>${frow('Passwort', secret('cs-pass', 'Passwort für den Link'))}</div>
      ${dir ? frow('Hochladen erlauben', sw(false, 'id="cs-up"'), 'Wer den Link hat, kann Dateien in diesen Ordner legen. Ansehen und löschen kann er dort nichts.') : ''}
      ${frow('Nachricht beim Öffnen', sw(false, 'id="cs-notify"'), 'Sobald jemand den Link öffnet, bekommst du eine Nachricht, hier in CannonCommander und über die Benachrichtigungen von Unraid.')}
    </div>`;
  function wireOptions(w, name) {
    $('#cs-pw', w).addEventListener('change', (e) => { $('#cs-pwrow', w).hidden = !swVal(e.target); if (swVal(e.target)) $('#cs-pass', w).focus(); });
    $('#cs-dice', w).addEventListener('click', () => { $('#cs-slug', w).value = freshSlug(name); });
  }
  function readLink(s) {
    const pw = swVal($('#cs-pw', s)), pass = $('#cs-pass', s).value, slugEl = $('#cs-slug', s), slug = slugEl.value.trim().toLowerCase();
    if (!SLUG.test(slug)) { CC.toast('Die Adresse braucht 3 bis 40 Zeichen: Kleinbuchstaben, Ziffern und Bindestriche', 'warn'); slugEl.focus(); return null; }
    if (taken(slug)) { CC.toast('Diese Adresse ist schon vergeben. Der Würfel findet eine freie.', 'warn'); slugEl.focus(); return null; }
    if (pw && pass.length < 6) { CC.toast('Das Passwort braucht mindestens 6 Zeichen', 'warn'); $('#cs-pass', s).focus(); return null; }
    const days = +segVal($('#cs-exp', s));
    return { by: CC.user.name, created: Date.now(), expires: days ? Date.now() + days * DAY : null, password: pw, uploads: !!$('#cs-up', s) && swVal($('#cs-up', s)),
      limit: +segVal($('#cs-limit', s)), notify: swVal($('#cs-notify', s)), message: $('#cs-msg', s).value.trim(), views: 0, slug };
  }
  function publish(link, title) {
    DATA.shares.unshift(link);
    DATA.activity.unshift({ t: Date.now(), who: CC.user.name, what: 'freigegeben', obj: link.name || link.path });
    // The prototype has no visitors, so someone opens the link a few seconds later.
    if (link.notify) setTimeout(() => {
      if (!DATA.shares.includes(link)) return;
      link.views += 1;
      CC.toast(`Jemand hat deinen Link zu ${title} geöffnet`, 'ok', { label: 'Freigaben', run: () => CC.run('openShares') });
    }, 6000);
  }

  CC.action('share', { label: 'Freigeben', g: 'IconLink', run: () => {
    const sel = Files.sel(), folder = Files.T().path;
    if (!sel.length) return CC.toast('Erst eine Datei oder einen Ordner wählen', 'warn');
    // Several files, or hits from a search below this folder, go out together under one link.
    if (sel.length > 1 || sel[0].at && sel[0].at !== folder) return shareMany(sel.map(pathOf));
    share(nodesOf(folder), folder);
  } });
  function share(items, folder) {
    if (items.length !== 1) return shareMany(items.map(k => FS.join(folder, k.name)));
    const n = items[0], path = FS.join(folder, n.name).replace(/::/g, '');
    if (FS.inArchive(folder)) return CC.toast('Aus einem Archiv lässt sich nichts freigeben', 'warn');
    const existingHtml = () => {
      const existing = DATA.shares.filter(s => s.path === path);
      return existing.length ? `<div class="eyebrow">Bestehende Links</div>${existing.map(s => frow(`<span class="mono cs-slug">${esc(HOST + s.slug)}</span>`, `<span class="pill glim-num">${s.expires ? `bis ${fmtDate(s.expires)}` : 'unbegrenzt'}</span><span class="pill glim-num">${plural(s.views, 'Aufruf', 'Aufrufe')}</span><button class="btn icon" data-reroll="${esc(s.slug)}" data-tip="Link neu würfeln, der alte führt dann ins Leere" aria-label="Link neu würfeln">${g('IconRetry')}</button><button class="btn icon" data-copy="${esc(HOST + s.slug)}" data-tip="Link kopieren" aria-label="Link kopieren">${g('IconCopy')}</button>`)).join('')}` : '';
    };
    const w = win({
      title: 'Freigeben',
      body: `<div class="well">${frow(`<span class="tile cs-tile">${g(FS.glyphOf(n))}</span><b class="cs-name">${esc(n.name)}</b>`, `<span class="pill glim-num">${fmtSize(FS.totalSize(n)) || '0 B'}</span>`)}</div>
        ${linkOptions(n.name, n.dir)}
        ${existingHtml() ? `<div class="well" id="cs-existing">${existingHtml()}</div>` : ''}`,
      foot: [{ start: esc(path) }, cancel, { label: 'Link erstellen', g: 'IconLink', primary: true, run: (s) => {
        const link = readLink(s);
        if (!link) return false;
        link.path = path;
        publish(link, n.name);
        setTimeout(() => done(link, n), 0);
      } }],
    });
    wireOptions(w, n.name);
    const box = $('#cs-existing', w);
    box?.addEventListener('click', (e) => {
      const b = e.target.closest('[data-reroll]');
      if (!b) return;
      const s = DATA.shares.find(x => x.slug === b.dataset.reroll), old = s.slug;
      s.slug = freshSlug(n.name);
      box.innerHTML = existingHtml();
      CC.toast('Neue Adresse gewürfelt, die alte führt ins Leere', 'ok', { label: 'Rückgängig', run: () => { if (!taken(old)) s.slug = old; if (box.isConnected) box.innerHTML = existingHtml(); } });
    });
  }
  CC.ck.share = share;

  // Die Sammelmappe: files from several folders wait here until they go out under one link.
  const Basket = [];
  const pathOf = (k) => FS.join(k.at || Files.T().path, k.name);
  const basketNodes = () => Basket.map(p => ({ path: p, node: FS.nodeAt(p) })).filter(x => x.node);
  // The other side sees a folder; two files with the same name from different folders get a number.
  const asFolder = (entries) => {
    const seen = {};
    return entries.map(({ path, node }) => {
      const n = (seen[node.name] = (seen[node.name] || 0) + 1);
      const dot = node.dir ? -1 : node.name.lastIndexOf('.');
      return { path, node, shown: n === 1 ? node.name : dot > 0 ? `${node.name.slice(0, dot)} (${n})${node.name.slice(dot)}` : `${node.name} (${n})` };
    });
  };

  CC.action('collect', { label: 'In die Sammelmappe', g: 'IconCollector', run: () => {
    const sel = Files.sel();
    if (!sel.length) return CC.toast('Erst eine Datei oder einen Ordner wählen', 'warn');
    if (FS.inArchive(Files.T().path)) return CC.toast('Aus einem Archiv lässt sich nichts freigeben', 'warn');
    const fresh = sel.map(pathOf).filter(p => !Basket.includes(p));
    Basket.push(...fresh);
    if (CC.view === 'files') CC.render();
    CC.toast(fresh.length ? `${plural(fresh.length, 'Objekt', 'Objekte')} in die Sammelmappe gelegt, jetzt ${Basket.length}` : 'Das liegt schon in der Sammelmappe', 'ok', { label: 'Sammelmappe', run: () => CC.run('collection') });
  } });
  CC.action('collection', { label: () => Basket.length ? `Sammelmappe (${Basket.length})` : 'Sammelmappe', g: 'IconCollector', run: () => {
    const body = () => {
      const items = basketNodes();
      return items.length ? `<div class="eyebrow">${plural(items.length, 'Objekt', 'Objekte')} aus ${plural(new Set(items.map(x => FS.parent(x.path))).size, 'Ordner', 'Ordnern')}, ${fmtSize(items.reduce((a, x) => a + FS.totalSize(x.node), 0)) || '0 B'}</div>
          <div class="cs-basket">${items.map(({ path, node }) => `<div class="cs-brow"><span class="tile">${g(FS.glyphOf(node))}</span><span class="t">${esc(node.name)}</span><span class="muted mono cs-bdir">${esc(FS.parent(path))}</span><span class="pill glim-num">${fmtSize(FS.totalSize(node)) || '0 B'}</span><button class="btn icon" data-bdel="${esc(path)}" data-tip="Aus der Sammelmappe nehmen" aria-label="Aus der Sammelmappe nehmen">${g('IconClose')}</button></div>`).join('')}</div>`
        : `<div class="empty">${g('IconCollector')}<div>Die Sammelmappe ist leer</div><div class="muted" style="font-size:12px">Markiere Dateien, auch in mehreren Ordnern oder in einem Suchergebnis, und wähle im Kontextmenü In die Sammelmappe.</div></div>`;
    };
    const w = win({
      title: 'Sammelmappe', wide: true,
      body: `<div class="well" id="cs-bbody">${body()}</div>`,
      foot: [{ start: 'Die Dateien bleiben, wo sie sind' }, { label: 'Leeren', g: 'IconEmpty', run: () => { Basket.length = 0; paint(); return false; } }, closeBtn,
        { label: 'Gemeinsam freigeben', g: 'IconLink', primary: true, run: () => { if (!basketNodes().length) return false; setTimeout(() => shareMany(Basket.slice(), true), 0); } }],
    });
    const paint = () => { $('#cs-bbody', w).innerHTML = body(); $('.win-foot .btn.primary', w).disabled = !basketNodes().length; if (CC.view === 'files') CC.render(); };
    w.addEventListener('click', (e) => { const b = e.target.closest('[data-bdel]'); if (b) { Basket.splice(Basket.indexOf(b.dataset.bdel), 1); paint(); } });
    $('.win-foot .btn.primary', w).disabled = !basketNodes().length;
  } });
  Files.sideRows.push(() => Basket.length ? `<div class="nav-row" data-run="collection">${g('IconCollector')}<span>Sammelmappe</span><span class="meta glim-num">${Basket.length}</span></div>` : '');

  function shareMany(paths, fromBasket = false) {
    const items = asFolder(paths.map(p => ({ path: p, node: FS.nodeAt(p) })).filter(x => x.node && !FS.inArchive(x.path)));
    if (!items.length) return CC.toast('Aus einem Archiv lässt sich nichts freigeben', 'warn');
    const dirs = new Set(items.map(x => FS.parent(x.path))).size;
    const suggest = `Sammlung vom ${new Date(CC.admin.now()).toLocaleDateString('de-AT', { day: '2-digit', month: '2-digit' })}`;
    const w = win({
      title: 'Sammelfreigabe',
      body: `<div class="well"><div class="eyebrow">${plural(items.length, 'Objekt', 'Objekte')} aus ${plural(dirs, 'Ordner', 'Ordnern')}, ${fmtSize(items.reduce((a, x) => a + FS.totalSize(x.node), 0)) || '0 B'}</div>
          <table class="tbl">${items.slice(0, 6).map(x => `<tr><td>${esc(x.node.name)}</td><td class="muted mono">${esc(FS.parent(x.path))}</td></tr>`).join('')}${items.length > 6 ? `<tr><td colspan="2">und ${items.length - 6} weitere</td></tr>` : ''}</table>
          ${frow('Name des Ordners', `<input class="field w" id="cs-title" value="${esc(suggest)}" autocomplete="off">`, 'Die Gegenseite sieht einen Ordner mit diesem Namen und darin alle Dateien nebeneinander. Die Dateien bleiben auf dem Server, wo sie sind.')}</div>
        ${linkOptions('sammlung', false)}`,
      foot: [{ start: 'Ein Link für alles' }, cancel, { label: 'Link erstellen', g: 'IconLink', primary: true, run: (s) => {
        const title = $('#cs-title', s).value.trim();
        if (!title) { CC.toast('Gib der Sammlung einen Namen', 'warn'); $('#cs-title', s).focus(); return false; }
        const link = readLink(s);
        if (!link) return false;
        Object.assign(link, { name: title, path: `${plural(items.length, 'Objekt', 'Objekte')} aus ${plural(dirs, 'Ordner', 'Ordnern')}`, items: items.map(x => x.path) });
        publish(link, title);
        if (fromBasket) { Basket.length = 0; if (CC.view === 'files') CC.render(); }
        setTimeout(() => done(link, { name: title, dir: true }, items), 0);
      } }],
    });
    wireOptions(w, 'sammlung');
  }

  function done(link, n, items) {
    const url = HOST + link.slug;
    win({
      title: 'Link erstellt', wide: !!items,
      body: `<div class="well cs-done"><div class="cs-qr">${qr(url)}</div><div class="cs-facts">
          <div class="cs-url"><input class="field grow mono" value="${esc(url)}" readonly aria-label="Link"><button class="btn icon" data-copy="${esc(url)}" data-tip="Link kopieren" aria-label="Link kopieren">${g('IconCopy')}</button></div>
          ${frow('Für', `<span class="pill">${esc(n.name)}</span>`)}
          ${frow('Gültig', `<span class="pill glim-num">${link.expires ? `bis ${fmtDate(link.expires)}` : 'unbegrenzt'}</span>`)}
          ${frow('Downloads', `<span class="pill glim-num">${link.limit ? `höchstens ${link.limit}` : 'ohne Limit'}</span>`)}
          ${frow('Passwort', `<span class="pill">${link.password ? 'ja' : 'nein'}</span>`)}
          ${n.dir && !items ? frow('Hochladen', `<span class="pill">${link.uploads ? 'erlaubt' : 'nein'}</span>`) : ''}
          ${frow('Nachricht beim Öffnen', `<span class="pill">${link.notify ? 'ja' : 'nein'}</span>`)}
        </div></div>
        ${items ? `<div class="well"><div class="eyebrow">So sieht es die Gegenseite ${info('Ein Ordner mit allen Dateien nebeneinander. Gleiche Namen aus verschiedenen Ordnern bekommen eine Nummer.')}</div>
          <div class="cs-visitor"><div class="cs-vhead">${g('IconFolderOpen')}<b>${esc(n.name)}</b><span class="muted">${plural(items.length, 'Objekt', 'Objekte')}</span></div>
          ${items.slice(0, 8).map(x => `<div class="cs-vrow">${g(FS.glyphOf(x.node))}<span>${esc(x.shown)}</span><span class="muted glim-num">${fmtSize(FS.totalSize(x.node)) || '0 B'}</span></div>`).join('')}${items.length > 8 ? `<div class="cs-vrow muted">und ${items.length - 8} weitere</div>` : ''}</div></div>` : ''}`,
      foot: [{ label: 'Link entfernen', g: 'IconUnlink', run: () => { DATA.shares = DATA.shares.filter(s => s !== link); CC.toast('Link entfernt'); } }, { label: 'Fertig', g: 'IconCheck', primary: true }],
      onOpen: (w) => setTimeout(() => $('.cs-url input', w).select(), 40),
    });
  }

  const file = MENUS.find(([m]) => m === 'Datei')[1];
  file.splice(file.indexOf('share') + 1, 0, 'collect', 'collection');
  document.addEventListener('click', (e) => {
    const b = e.target.closest('[data-copy]');
    if (!b) return;
    try { navigator.clipboard?.writeText(b.dataset.copy); } catch {}
    CC.toast('Link kopiert');
  });

  // A QR code's look: finder squares, timing lines and modules seeded by the link, on a white quiet zone.
  function qr(text) {
    const N = 29, r = rng(text), on = Array.from({ length: N }, () => Array(N).fill(false)), fixed = Array.from({ length: N }, () => Array(N).fill(false));
    const finder = (x, y) => { for (let i = -1; i <= 7; i++) for (let j = -1; j <= 7; j++) { const a = x + i, b = y + j; if (a < 0 || b < 0 || a >= N || b >= N) continue; fixed[b][a] = true; const ring = Math.max(Math.abs(i - 3), Math.abs(j - 3)); on[b][a] = ring !== 2 && ring <= 3; } };
    finder(0, 0); finder(N - 7, 0); finder(0, N - 7);
    for (let i = 8; i < N - 8; i++) { on[6][i] = on[i][6] = i % 2 === 0; fixed[6][i] = fixed[i][6] = true; }
    for (let i = -2; i <= 2; i++) for (let j = -2; j <= 2; j++) { const ring = Math.max(Math.abs(i), Math.abs(j)); on[22 + j][22 + i] = ring !== 1; fixed[22 + j][22 + i] = true; }
    for (let y = 0; y < N; y++) for (let x = 0; x < N; x++) if (!fixed[y][x]) on[y][x] = r() < .48;
    let d = '';
    for (let y = 0; y < N; y++) for (let x = 0; x < N; x++) if (on[y][x]) d += `M${x + 2} ${y + 2}h1v1h-1z`;
    return `<svg viewBox="0 0 ${N + 4} ${N + 4}" shape-rendering="crispEdges" role="img" aria-label="QR-Code für den Link"><rect width="${N + 4}" height="${N + 4}" fill="#fff"/><path d="${d}" fill="#161616"/></svg>`;
  }

  CC.css(`
.cu-drop { align-items: center; justify-content: center; text-align: center; gap: .35rem; padding: 2rem 1rem; cursor: pointer; width: 100%; color: var(--carbon-text); transition: background .12s; }
.cu-drop svg { width: 2rem; height: 2rem; color: var(--accent-glyph); margin-bottom: .3rem; }
.cu-drop .t { font-size: 16px; font-weight: 600; }
.cu-drop .muted { font-size: 13px; }
.cu-drop:hover, .cu-drop:focus-visible { background: var(--carbon-surface3); }
.cu-drop.over { background: color-mix(in srgb, var(--accent) 12%, var(--carbon-surface2)); outline: 2px dashed var(--accent-ink); outline-offset: -6px; }
.cu-path { max-width: 26rem; overflow: hidden; text-overflow: ellipsis; }
.cu-total { display: flex; flex-direction: column; gap: .4rem; padding-bottom: .3rem; }
.cu-total .frow { min-height: 1.6rem; }
.cu-list { display: flex; flex-direction: column; max-height: 16rem; overflow: auto; margin: 0 -.3rem; }
.cu-row { display: grid; grid-template-columns: 2.25rem minmax(0, 1fr) 8rem 5.5rem 4.2rem; align-items: center; gap: .6rem; min-height: 2.4rem; padding: 0 .3rem; border-radius: var(--radius-control); }
.cu-row:hover { background: var(--carbon-hover); }
.cu-row .t { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cu-row .tile { background: var(--carbon-surface3); }
.cu-row .pill { justify-self: end; background: var(--carbon-surface3); }
.cu-row.done .t { color: var(--carbon-text-sub); }
.cu-row > :nth-child(3):not(.bar) { grid-column: 3 / 5; justify-self: end; }
.cu-ok { display: inline-flex; align-items: center; gap: .4rem; font-size: 12px; color: var(--carbon-text-sub); }
.cu-ok i { width: .5rem; height: .5rem; border-radius: 50%; background: var(--status-ok-solid); }
.cu-acts { display: flex; gap: .25rem; justify-content: flex-end; }
.cu-acts .btn, #cu-queue .eyebrow .btn { width: 1.8rem; height: 1.8rem; background: transparent; }
.cu-acts .btn:hover, #cu-queue .eyebrow .btn:hover { background: var(--carbon-surface3); }
#cu-queue .eyebrow .btn + .btn { margin-left: 0; }
.cu-empty { font-size: 13px; padding: .3rem 0 .2rem; }
.cs-tile { width: 1.75rem; height: 1.75rem; }
.cs-tile svg { color: var(--accent-glyph); }
.cs-name { font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cs-slug { font-size: 12.5px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.well .frow .pill + .pill, .well .frow .pill + .btn { margin-left: .3rem; }
.cs-secret { position: relative; display: inline-flex; }
.cs-secret .field { padding-right: 2.4rem; }
.cs-eye { position: absolute; right: .35rem; top: 50%; transform: translateY(-50%); width: 1.7rem; height: 1.7rem; display: grid; place-items: center; color: var(--carbon-text-muted); border-radius: var(--radius-pill); }
.cs-eye:hover { color: var(--carbon-text); }
.cs-eye svg { width: 1rem; height: 1rem; }
.cs-done { flex-direction: row; gap: 1.2rem; align-items: center; }
.cs-qr { width: 11rem; flex: none; border-radius: var(--radius-control); overflow: hidden; }
.cs-qr svg { width: 100%; height: auto; }
.cs-facts { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: .35rem; }
.cs-url { display: flex; gap: .4rem; margin-bottom: .3rem; }
.cs-url .field { font-size: 12.5px; }
.cs-slugf { display: flex; align-items: center; gap: .35rem; min-width: 0; flex: 1; justify-content: flex-end; }
.cs-host { font-size: 12px; white-space: nowrap; flex: none; }
.cs-slugf .field { width: 11rem; min-width: 9.5rem; flex: 0 1 11rem; }
.cs-basket { display: flex; flex-direction: column; max-height: 20rem; overflow: auto; margin: 0 -.3rem; }
.cs-brow { display: grid; grid-template-columns: 2.25rem minmax(0, 1fr) minmax(0, 1fr) 5.5rem 2rem; align-items: center; gap: .6rem; min-height: 2.4rem; padding: 0 .3rem; border-radius: var(--radius-control); }
.cs-brow:hover { background: var(--carbon-hover); }
.cs-brow .tile { background: var(--carbon-surface3); }
.cs-brow .t, .cs-bdir { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cs-bdir { font-size: 12px; }
.cs-brow .pill { justify-self: end; background: var(--carbon-surface3); }
.cs-brow .btn { width: 1.8rem; height: 1.8rem; background: transparent; }
.cs-visitor { border-radius: var(--radius-control); background: var(--carbon-surface); padding: .5rem .7rem; display: flex; flex-direction: column; gap: .1rem; }
.cs-vhead { display: flex; align-items: center; gap: .5rem; padding-bottom: .35rem; margin-bottom: .2rem; box-shadow: inset 0 -1px 0 var(--carbon-hover); }
.cs-vhead svg, .cs-vrow svg { width: 1rem; height: 1rem; color: var(--accent-glyph); flex: none; }
.cs-vhead .muted { margin-left: auto; font-size: 12px; }
.cs-vrow { display: flex; align-items: center; gap: .55rem; min-height: 1.8rem; font-size: 13px; }
.cs-vrow span:first-of-type { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cs-vrow .muted { font-size: 12px; }
@media (max-width: 760px) {
  .cs-brow { grid-template-columns: 2.25rem minmax(0, 1fr) 2rem; }
  .cs-brow .cs-bdir, .cs-brow .pill { display: none; }
  .cs-host { display: none; }
  .cu-row { grid-template-columns: 2.25rem minmax(0, 1fr) 4.5rem 4.2rem; }
  .cu-row > .bar { display: none; }
  .cu-row > :nth-child(3):not(.bar) { grid-column: 3; }
  .cu-path { max-width: 11rem; }
  .cs-done { flex-direction: column; align-items: stretch; }
  .cs-qr { width: 9rem; align-self: center; }
  .cs-secret .field.w { width: 100%; }
  .cs-secret { flex: 1; }
}
`);
})();
