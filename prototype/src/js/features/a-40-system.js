// Integrationen, App and Info, the help actions and the activity page.
(() => {
  const A = CC.admin;
  const { pill, row, list, foot, btn, field, fail } = A;

  DATA.integrations = DATA.integrations || { officeUrl: '', officeSecret: '', thumbs: true, videoPreview: false, webdav: false, ffmpeg: '7.1.1', ffmpegPath: '/usr/bin/ffmpeg' };
  DATA.app = DATA.app || { autoCheck: true, image: 'ghcr.io/junkerderprovinz/castlecommander', tag: 'latest', logLevel: 'info', started: NOW - 3 * DAY - 4 * 3600000 };
  const appdata = FS.nodeAt('/mnt/user/appdata');
  if (appdata && !FS.kids(appdata).some(k => k.name === 'castlecommander')) {
    appdata.children.push({ name: 'castlecommander', dir: true, time: NOW - 3 * DAY, owner: 'nobody', group: 'users', mode: '0775', children: [
      { name: 'exports', dir: true, children: [], time: NOW - 3 * DAY, owner: 'nobody', group: 'users', mode: '0775' },
      { name: 'config.yaml', size: 4 * KB, time: NOW - 3 * DAY, owner: 'nobody', group: 'users', mode: '0664' },
      { name: 'castlecommander.db', size: 22 * MB, time: NOW - 600000, owner: 'nobody', group: 'users', mode: '0664' },
    ] });
  }
  const today = new Date(NOW).toISOString().slice(0, 10);

  /* Integrationen */
  CC.settingsTab({ id: 'integrations', label: 'Integrationen', g: 'IconModules', order: 100, render(host) {
    const it = DATA.integrations, rc = DATA.rclone;
    host.innerHTML = CC.card('OnlyOffice',
      frow('Server-Adresse', `<span class="a-ctl">${field(it.officeUrl, 'id="a-oourl" placeholder="https://office.tower.lan"')}${btn('Testen', 'IconHealth', 'id="a-ootest"')}</span>`, 'Mit einem OnlyOffice-Server öffnen sich Word-, Excel- und PowerPoint-Dateien zum Bearbeiten im Browser.')
      + frow('JWT-Schlüssel', A.pwField(`id="a-oosecret" value="${esc(it.officeSecret)}" placeholder="Aus JWT_SECRET"`), 'Steht in der Konfiguration des OnlyOffice-Containers unter JWT_SECRET. Ohne ihn lehnt der Server jede Datei ab.'))
      + CC.card('Videos und Bilder',
        frow('Vorschaubilder', sw(it.thumbs, 'data-it="thumbs"'), 'Kleine Bilder für Fotos und Videos in der Vorschau. ffmpeg erzeugt sie im Hintergrund.')
        + frow('Videos im Browser abspielen', sw(it.videoPreview, 'data-it="videoPreview"'), 'Auch Formate wie MKV. ffmpeg wandelt sie dafür beim Abspielen um, das kostet Rechenzeit.')
        + frow('ffmpeg', pill(`${it.ffmpeg}, ${it.ffmpegPath}`, 'mono'), 'Ist im Container dabei.'))
      + CC.card('WebDAV und rclone',
        frow('WebDAV-Server', sw(it.webdav, 'data-it="webdav"'), 'Damit bindest du die Dateien als Laufwerk ein, etwa im Windows-Explorer oder in einer Handy-App. Es gelten dieselben Benutzer und Regeln.')
        + (it.webdav ? frow('Adresse', A.copyField('https://castle.tower.lan/dav/')) : '')
        + frow('rclone', pill(`${rc.version}, ${rc.path}`, 'mono'), 'Hängt die Fernziele ein. Das Programm ist im Container dabei.'));
    host.addEventListener('change', e => {
      const k = e.target.dataset.it;
      if (k) { it[k] = swVal(e.target); if (k === 'webdav') A.redraw(); CC.toast('Gespeichert'); return; }
      if (e.target.id === 'a-oourl') { it.officeUrl = e.target.value.trim(); CC.toast('Gespeichert'); }
      if (e.target.id === 'a-oosecret') { it.officeSecret = e.target.value; CC.toast('Gespeichert'); }
    });
    $('#a-oourl', host).addEventListener('input', () => { const b = $('#a-ootest', host); b.className = 'btn'; b.innerHTML = g('IconHealth') + 'Testen'; });
    $('#a-ootest', host).addEventListener('click', e => {
      const b = e.currentTarget, url = $('#a-oourl', host).value.trim();
      if (!url) return fail('Erst eine Adresse eintragen', b);
      if (!/^https?:\/\/[^/\s]+/.test(url)) return fail('Die Adresse beginnt mit http:// oder https://', b);
      it.officeUrl = url;
      b.disabled = true;
      b.innerHTML = g('IconRefresh') + 'Prüfe';
      setTimeout(() => {
        b.disabled = false;
        if (!$('#a-oosecret', host).value) { b.className = 'btn bad'; b.innerHTML = g('IconClose') + 'Abgelehnt'; A.shake(b); CC.toast('Der Server antwortet, will aber den JWT-Schlüssel', 'warn'); return; }
        b.className = 'btn ok'; b.innerHTML = g('IconCheck') + 'Verbunden';
      }, 800);
    });
  } });

  /* App */
  function exportWin() {
    const w = A.stepWin({ title: 'Einstellungen exportieren' });
    w.set(`<div class="well">${frow('Ordner', A.pathField('/mnt/user/appdata/castlecommander/exports', 'a-exdir'), 'Der Server schreibt die Datei hierhin, nicht in die Downloads deines Browsers.')}
        ${frow('Dateiname', field(`castlecommander-einstellungen-${today}.json`, 'id="a-exname"'))}</div>
      <div class="well">${frow('Benutzer, Gruppen und Regeln mitnehmen', sw(true, 'id="a-exusers"'))}
        ${frow('Passwörter und Zugangsdaten', pill('Bleiben draußen'), 'Passwörter, Tokens, Passkeys und die Zugangsdaten der Fernziele kommen nie in die Datei.')}</div>`,
      [cancel, { label: 'Exportieren', g: 'IconExport', primary: true, run: (el) => {
        const dir = $('#a-exdir', el).value.trim().replace(/\/+$/, ''), name = $('#a-exname', el).value.trim();
        const node = FS.nodeAt(dir);
        if (!node || !node.dir) return fail('Diesen Ordner gibt es nicht', $('#a-exdir', el));
        if (!/^[^/\\]+\.json$/.test(name)) return fail('Der Dateiname endet auf .json', $('#a-exname', el));
        if (FS.kids(node).some(k => k.name === name)) return fail('Diese Datei gibt es dort schon', $('#a-exname', el));
        node.children.push({ name, size: (swVal($('#a-exusers', el)) ? 14 : 6) * KB, time: A.now(), owner: 'nobody', group: 'users', mode: '0664' });
        const full = FS.join(dir, name);
        DATA.activity.unshift({ t: A.now(), who: CC.user.name, what: 'exportiert', obj: full });
        w.set(`<div class="well">${frow('Gespeichert unter', A.copyField(full, '', true))}</div>`,
          [{ label: 'Im Dateimanager zeigen', g: 'IconFolderOpen', run: () => { Files.go(dir); const i = Files.entries().findIndex(k => k.name === name); if (i >= 0) { Files.T().cursor = i; CC.render(); } } }, { label: 'Fertig', g: 'IconCheck', primary: true }]);
        return false;
      } }]);
  }

  function importWin() {
    const exports = FS.kids(FS.nodeAt('/mnt/user/appdata/castlecommander/exports')).filter(k => k.name.endsWith('.json')).sort((a, b) => b.time - a.time);
    const start = exports[0] ? `/mnt/user/appdata/castlecommander/exports/${exports[0].name}` : '';
    win({ title: 'Einstellungen importieren',
      body: `<div class="well">${frow('Datei', A.pathField(start, 'a-imfile', { exts: ['json'], attr: 'placeholder="/mnt/user/appdata/castlecommander/exports/…json"' }), 'Eine Datei, die CastleCommander exportiert hat.')}
        ${frow('Benutzer, Gruppen und Regeln übernehmen', sw(true, 'id="a-imusers"'), 'Passwörter, Passkeys und Tokens bleiben hier, wie sie sind.')}</div>`,
      foot: [cancel, { label: 'Importieren', g: 'IconImport', primary: true, run: (w) => {
        const p = $('#a-imfile', w).value.trim(), n = FS.nodeAt(p);
        if (!n || n.dir) return fail('Diese Datei gibt es nicht', $('#a-imfile', w));
        if (FS.ext(n.name) !== 'json') return fail('Das ist keine exportierte Einstellungsdatei', $('#a-imfile', w));
        CC.toast(`Einstellungen aus ${n.name} übernommen`);
      } }] });
  }

  function restart() {
    const running = Jobs.running();
    A.confirmWin({ title: 'Neu starten', verb: 'Neu starten', glyph: 'IconRefresh',
      text: `<p>CastleCommander ist ein paar Sekunden nicht erreichbar und lädt die Seite danach neu.</p>${running.length ? `<p>${running.length === 1 ? 'Eine laufende Aufgabe bricht' : `${running.length} laufende Aufgaben brechen`} dabei ab.</p>` : ''}`,
      run: () => {
        running.forEach(Jobs.cancel);
        const veil = document.createElement('div');
        veil.className = 'scrim glim-modal-backdrop a-veil';
        veil.innerHTML = `<div class="glim-card a-veil-card">${g('IconRefresh')}<span>CastleCommander startet neu</span></div>`;
        document.body.appendChild(veil);
        setTimeout(() => { veil.remove(); DATA.app.started = A.now(); A.redraw(); CC.toast('CastleCommander läuft wieder'); }, 2200);
      } });
  }

  CC.settingsTab({ id: 'app', label: 'App', g: 'IconTabApp', order: 110, render(host) {
    const ap = DATA.app;
    host.innerHTML = CC.card('Updates',
      frow('Automatisch nach Updates suchen', sw(ap.autoCheck, 'id="a-auto"'), 'Einmal am Tag. Installieren musst du ein Update selbst, auf Unraid im Docker-Tab.')
      + frow('Docker-Image', A.copyField(ap.image))
      + frow('Tag', pill(ap.tag, 'mono'), 'Den Tag legst du im Unraid-Template fest. latest folgt jeder neuen Version.')
      + foot(btn('Nach Updates suchen', 'IconSync', 'id="a-upd"')))
      + CC.card('Einstellungen sichern',
        frow('In eine Datei schreiben', btn('Exportieren', 'IconExport', 'id="a-export"'), 'Alle Einstellungen als JSON-Datei auf dem Server, ohne Passwörter.')
        + frow('Aus einer Datei holen', btn('Importieren', 'IconImport', 'id="a-import"'), 'Ersetzt die Einstellungen hier durch die aus der Datei.'))
      + CC.card('Dienst',
        frow('Protokoll', seg([['error', 'Nur Fehler'], ['info', 'Normal'], ['debug', 'Ausführlich']], ap.logLevel, 'id="a-log"'), 'Wie viel CastleCommander ins Container-Log schreibt. Auf Unraid: Docker-Tab, CastleCommander, Logs.')
        + frow('Läuft', `<span class="a-ctl">${pill(A.ago(ap.started).replace('vor ', 'seit '))}${btn('Neu starten', 'IconRefresh', 'id="a-restart"')}</span>`));
    $('#a-auto', host).addEventListener('change', e => { ap.autoCheck = swVal(e.target); CC.toast('Gespeichert'); });
    $('#a-log', host).addEventListener('change', e => { ap.logLevel = segVal(e.target); CC.toast('Gespeichert'); });
    $('#a-export', host).addEventListener('click', exportWin);
    $('#a-import', host).addEventListener('click', importWin);
    $('#a-restart', host).addEventListener('click', restart);
    $('#a-upd', host).addEventListener('click', e => {
      const b = e.currentTarget;
      b.disabled = true;
      b.innerHTML = g('IconSync') + 'Suche';
      setTimeout(() => { b.disabled = false; b.className = 'btn ok'; b.innerHTML = `${g('IconUpToDate')}Aktuell, ${esc(DATA.version.app)}`; }, 900);
    });
  } });

  /* Info */
  const NEWS = [
    { id: 'files', g: 'IconFiles', text: 'Zwei Fenster mit Tabs, Ordnerbaum und Funktionstasten', go: () => CC.go('files') },
    { id: 'look', g: 'IconTabLook', text: 'Design, Akzentfarbe, Ecken und Zeilenhöhe nach deinem Geschmack', go: () => CC.go('settings', 'look') },
    { id: 'users', g: 'IconAccounts', text: 'Benutzer & Gruppen: jede Person mit Rolle und eigenem Bereich', go: () => CC.go('settings', 'users') },
    { id: 'rules', g: 'IconShieldOn', text: 'Zugriffsregeln und eine Probe, wer was sieht', go: () => CC.go('settings', 'rules') },
    { id: 'remotes', g: 'IconCloud', text: 'Fernziele über rclone, von SFTP bis Google Drive', go: () => CC.go('settings', 'remotes') },
    { id: 'shares', g: 'IconLink', text: 'Freigaben mit Ablaufdatum und Passwort', go: () => CC.go('settings', 'shares') },
    { id: 'security', g: 'IconTabSecurity', text: 'Zwei-Faktor-Authentifizierung, Passkeys und API-Tokens', go: () => CC.go('settings', 'security') },
    { id: 'activity', g: 'IconLog', text: 'Aktivität: wer wann was getan hat', go: () => CC.run('openActivity') },
  ];
  const seen = () => CC.prefs.newsSeen || [];
  const unseen = () => NEWS.filter(n => !seen().includes(n.id)).length;
  const versionUrl = { app: 'https://github.com/junkerderprovinz/castlecommander/releases/tag/v', glimstone: 'https://github.com/junkerderprovinz/glimstone/releases/tag/v', quantum: 'https://github.com/gtsteffaniak/filebrowser/releases/tag/v' };
  const vlink = (k) => `<a class="a-vlink glim-num" href="${versionUrl[k]}${esc(DATA.version[k])}" target="_blank" rel="noopener" data-tip="Versionshinweise auf GitHub">${esc(String(DATA.version[k]).replace(/^v/, ''))}</a>`;
  const readmeBtn = ({ tile, mark, glyph, name, sub, attr }) => `<div class="glim-readme-btn-unit group ${tile}"><button class="glim-readme-btn glim-brand-tile" ${attr}><span class="glim-readme-btn-mark ${mark}">${g(glyph)}</span><span class="glim-readme-btn-text"><span class="glim-readme-btn-name">${esc(name)}</span><span class="glim-readme-btn-sub">${esc(sub)}</span></span></button></div>`;

  function bugReport() {
    const report = { app: DATA.version, prefs: { ...CC.prefs }, sources: DATA.sources.map(s => ({ path: s.path, indexed: s.indexed, readOnly: s.readOnly })), remotes: DATA.remotes.map(r => ({ type: r.type, state: r.state })), users: DATA.users.length, jobs: Jobs.list.map(j => ({ kind: j.kind, state: j.state })) };
    const a = document.createElement('a');
    a.href = URL.createObjectURL(new Blob([JSON.stringify(report, null, 2)], { type: 'application/json' }));
    a.download = `castlecommander-fehlerbericht-${today}.json`;
    a.click();
    setTimeout(() => URL.revokeObjectURL(a.href), 1000);
    CC.toast(`${a.download} liegt in deinen Downloads`);
  }

  function coffeeWin() {
    win({ title: 'Kaffee spendieren',
      body: `<div class="well a-text"><p>Wirf deinem Ritter eine Münze zu: Das deckt einen Teil der Kosten und hält das Projekt am Leben. Alles, was ich baue, kostet nichts und zeigt keine Werbung.</p><p>Buy Me a Coffee nimmt Karte, PayPal sowie Apple Pay und Google Pay. Ein Konto brauchst du dort nicht.</p></div>`,
      foot: [cancel, { label: 'Zu Buy Me a Coffee', g: 'IconCoffee', primary: true, run: () => { window.open('https://buymeacoffee.com/junkerderprovinz', '_blank', 'noopener'); } }] });
  }

  CC.settingsTab({ id: 'info', label: 'Info', g: 'IconInfo', order: 120, render(host) {
    const n = unseen();
    host.innerHTML = CC.card('Version',
      frow('CastleCommander', vlink('app'))
      + frow('GlimStone', vlink('glimstone'), 'Die Designsprache, in der CastleCommander gebaut ist.')
      + frow('FileBrowser Quantum', vlink('quantum'), 'CastleCommander baut auf einem Fork von FileBrowser Quantum auf.')
      + foot(`<button class="btn" data-run="whatsNew"><span class="a-dot ${n ? '' : 'seen'}"></span>${g('IconWhatsNew')}${n ? `Neu · ${n}` : 'Neu'}</button>`))
      + CC.card('Hilfe',
        frow('Handbuch', btn('Öffnen', 'IconManual', 'data-run="docs"'), 'Eine kurze Einführung, wie CastleCommander gedacht ist.')
        + frow('Tastenkürzel', btn('Anzeigen', 'IconKeyboard', 'data-run="shortcuts"'))
        + frow('Fehlerbericht', btn('Herunterladen', 'IconDiagnostics', 'id="a-bug"'), 'Packt Einstellungen und den Zustand der Aufgaben in eine Datei, ohne Passwörter, Schlüssel oder Zugangsdaten. Häng sie an ein Issue auf GitHub an.'))
      + CC.card('Über CastleCommander', `<div class="a-about">
        <p>Ein Kreuzzug von einem einzigen Ritter: gut aussehende Open-Source-Werkzeuge, die es so, wie ich sie wollte, nicht gab, gebaut, bis nichts mehr fehlt. Kostenlos, ohne Konto, ohne Telemetrie, ohne Werbung und ohne Bezahlstufe. Kein Sternchen, nirgends. Nichts Lesbares verlässt je deine eigenen Mauern. Geschmiedet an Abenden und Wochenenden, mit Herz und Sturheit.</p>
        <p class="a-gap">Wenn sich CastleCommander einen Platz auf deinem Server verdient hat, wirf deinem Ritter eine Münze zu: Das hilft bei den Kosten und hält das Projekt am Leben. Und es lässt das Herz dieses Ritters ein wenig schneller schlagen.</p>
        <div class="glim-readme-btn-rows glim-about-give">${readmeBtn({ tile: 'glim-tile-coffee', mark: 'a-coffee-mark', glyph: 'IconCoffee', name: 'Kaffee', sub: 'Buy Me a Coffee', attr: 'id="a-coffee"' })}</div>
        <p class="a-gap">Probleme, Wünsche oder Ideen? Mach gern ein Issue auf GitHub auf.</p>
        <div class="glim-readme-btn-rows">${readmeBtn({ tile: 'glim-tile-github', mark: 'glim-github-mark', glyph: 'IconGithub', name: 'GitHub', sub: 'Issue öffnen', attr: 'id="a-github"' })}</div></div>`);
    $('#a-bug', host).addEventListener('click', bugReport);
    $('#a-coffee', host).addEventListener('click', coffeeWin);
    $('#a-github', host).addEventListener('click', () => window.open('https://github.com/junkerderprovinz/castlecommander/issues', '_blank', 'noopener'));
  } });

  function whatsNewWin() {
    const w = A.stepWin({ title: `Neu in ${DATA.version.app}`, wide: true });
    const render = () => {
      const left = unseen();
      w.set(`<div class="a-newtop">${left ? btn(`Alle als gesehen markieren · ${left}`, 'IconCheck', 'id="a-newall"') : btn(`Wieder alle zeigen · ${NEWS.length}`, 'IconEye', 'id="a-newall"')}</div>
        <div class="well a-release"><span class="notch">${esc(DATA.version.app)}</span>
          <p class="a-text">Die erste Vorschau: CastleCommander als Dateimanager mit zwei Fenstern für Unraid, dazu alles, was ein Server mit mehreren Personen braucht. Alle Daten sind ausgedacht, nichts davon erreicht einen echten Server.</p>
          <div class="a-newlist">${NEWS.map(n => `<div class="a-newrow ${seen().includes(n.id) ? 'seen' : ''}"><span class="a-dot ${seen().includes(n.id) ? 'seen' : ''}"></span><span class="tile">${g(n.g)}</span><span class="t">${esc(n.text)}</span><button class="btn" data-newgo="${n.id}">${g('IconForward')}Hingehen</button></div>`).join('')}</div></div>`,
        [closeBtn]);
    };
    w.el.addEventListener('click', e => {
      if (e.target.closest('#a-newall')) { CC.prefs.newsSeen = unseen() ? NEWS.map(n => n.id) : []; A.setPref('newsSeen', CC.prefs.newsSeen); render(); return; }
      const go = e.target.closest('[data-newgo]');
      if (go) {
        const n = NEWS.find(x => x.id === go.dataset.newgo);
        A.setPref('newsSeen', [...new Set([...seen(), n.id])]);
        w.close();
        n.go();
      }
    });
    render();
  }

  function docsWin() {
    const part = (title, text) => `<div class="well"><div class="eyebrow">${esc(title)}</div><div class="a-text">${text}</div></div>`;
    win({ title: 'Handbuch', wide: true,
      body: part('Zwei Fenster', '<p>Links und rechts steht je ein Ordner. Tab wechselt zwischen ihnen, F5 kopiert das Markierte ins andere Fenster und F6 verschiebt es. Mit Strg+1 und Strg+2 schaltest du zwischen einem und zwei Fenstern um.</p>')
        + part('Markieren', '<p>Einfg markiert eine Zeile und springt zur nächsten, die Leertaste zeigt die Datei groß in der Schnellansicht. Strg+Klick nimmt einzelne dazu, Umschalt+Klick einen ganzen Bereich. Mit + markierst du nach Muster, etwa alle *.jpg.</p>')
        + part('Finden', '<p>Ein / öffnet den Schnellfilter im aktuellen Ordner. Alt+F7 sucht auf dem ganzen Server, schnell dank des Index unter Einstellungen, Quellen.</p>')
        + part('Teilen und Fernziele', '<p>Über Datei, Freigeben bekommst du einen Link mit Ablaufdatum und auf Wunsch einem Passwort. Google Drive, ein NAS oder einen anderen Server hängst du unter Einstellungen, Fernziele ein. Danach liegen sie im Ordnerbaum wie jeder andere Ordner.</p>')
        + part('Wer was darf', '<p>Jede Person hat eine Rolle und einen Bereich, aus dem sie nicht herauskommt. Zugriffsregeln verbergen darin einzelne Ordner oder machen sie schreibgeschützt. Unter Konto und Sicherheit schaltest du die Zwei-Faktor-Authentifizierung ein.</p>'),
      foot: [{ label: 'Tastenkürzel', g: 'IconKeyboard', run: () => setTimeout(() => CC.run('shortcuts'), 0) }, closeBtn] });
  }

  /* Aktivität */
  const KINDS = { kopiert: 'IconCopy', verschoben: 'IconMove', umbenannt: 'IconPencil', hochgeladen: 'IconUpload', freigegeben: 'IconLink', angemeldet: 'IconSignIn', gelöscht: 'IconTrash', exportiert: 'IconExport' };
  const act = { who: null, what: null, dir: -1 };
  const pathIn = (obj) => (obj.match(/\/mnt\/\S+|\/boot\S*/) || [])[0];

  CC.registerView('activity', {
    page: true,
    render(host) {
      const all = DATA.activity;
      const shown = all.filter(a => (!act.who || a.who === act.who) && (!act.what || a.what === act.what)).sort((a, b) => (a.t - b.t) * act.dir);
      const dayStart = NOW - (NOW % DAY);
      const filtered = act.who || act.what;
      const stat = (num, label) => `<div class="well a-stat"><span class="glim-num">${num}</span><span>${label}</span></div>`;
      const head = (k, label) => `<button class="a-hbtn ${act[k] ? 'on' : ''}" data-af="${k}" data-tip="Filtern">${esc(act[k] ? (k === 'who' ? DATA.users.find(u => u.name === act.who)?.full || act.who : act.what) : label)}${g('IconExpand')}</button>`;
      host.innerHTML = `<div class="page-inner">
        ${CC.card('Überblick', `<div class="a-stats">${stat(filtered ? `${shown.length} von ${all.length}` : all.length, filtered ? 'Einträge im Filter' : 'Einträge')}${stat(all.filter(a => a.t >= dayStart).length, 'Heute')}${stat(new Set(all.map(a => a.who)).size, 'Personen')}</div>`
          + (filtered ? foot(btn('Filter zurücksetzen', 'IconClose', 'data-afreset')) : ''))}
        ${CC.card(filtered ? 'Gefilterte Einträge' : 'Alle Einträge', `<div class="a-actlist">
          <div class="a-lhead"><span></span><span>Was</span>${head('who', 'Wer')}${head('what', 'Art')}<button class="a-hbtn" data-asort>Wann${g(act.dir < 0 ? 'IconMoveDown' : 'IconMoveUp')}</button></div>
          ${shown.length ? shown.map(a => {
            const p = pathIn(a.obj), node = p && FS.nodeAt(p);
            return `<div class="a-row a-arow" ${node ? `data-open="${esc(node.dir ? p : FS.parent(p))}" tabindex="0" data-tip="Im Dateimanager zeigen"` : ''}>
              <span class="tile">${g(KINDS[a.what] || 'IconLog')}</span><span class="a-name"><span class="t">${esc(a.obj)}</span></span>
              ${pill(DATA.users.find(u => u.name === a.who)?.full || a.who)}${pill(a.what)}${pill(A.ago(a.t), 'glim-num')}</div>`;
          }).join('') : `<div class="empty">${g('IconFilter')}<div>Nichts passt zum Filter</div></div>`}
        </div>`)}</div>`;
      host.onclick = e => {
        const f = e.target.closest('[data-af]');
        if (f) {
          const k = f.dataset.af, r = f.getBoundingClientRect();
          const values = [...new Set(DATA.activity.map(a => a[k]))];
          menu(r.left, r.bottom + 6, [{ label: 'Alle', checked: () => !act[k], run: () => { act[k] = null; CC.render(); } }, '-',
            ...values.map(v => ({ label: k === 'who' ? (DATA.users.find(u => u.name === v)?.full || v) : v, g: k === 'what' ? KINDS[v] || 'IconLog' : null, checked: () => act[k] === v, run: () => { act[k] = v; CC.render(); } }))]);
          e.stopPropagation();
          return;
        }
        if (e.target.closest('[data-asort]')) { act.dir *= -1; CC.render(); return; }
        if (e.target.closest('[data-afreset]')) { act.who = act.what = null; CC.render(); return; }
        const o = e.target.closest('[data-open]');
        if (o) Files.go(o.dataset.open);
      };
      host.onkeydown = e => { if (e.key === 'Enter' && e.target.dataset.open) Files.go(e.target.dataset.open); };
    },
  });

  CC.action('about', { label: 'Über CastleCommander', g: 'IconInfo', run: () => CC.go('settings', 'info') });
  CC.action('mounts', { label: 'Fernziele', g: 'IconCloud', run: () => CC.go('settings', 'remotes') });
  CC.action('openShares', { label: 'Freigaben', g: 'IconLink', run: () => CC.go('settings', 'shares') });
  CC.action('openActivity', { label: 'Aktivität', g: 'IconLog', run: () => CC.go('activity') });
  CC.action('docs', { label: 'Handbuch', g: 'IconManual', run: docsWin });
  CC.action('whatsNew', { label: 'Neuigkeiten', g: 'IconWhatsNew', run: whatsNewWin });

  CC.css(`
.a-vlink { height: 1.6rem; padding: 0 .7rem; border-radius: var(--radius-pill); background: var(--carbon-surface2); color: var(--carbon-text); text-decoration: none; display: inline-flex; align-items: center; font-size: 13px; }
.a-vlink:hover, .a-vlink:focus-visible { background: color-mix(in srgb, var(--carbon-surface3), var(--accent) 22%); }
.a-dot { width: .5rem; height: .5rem; border-radius: 50%; background: #6fdc8c; box-shadow: 0 0 6px #6fdc8c; flex: none; }
.a-dot.seen { background: var(--carbon-text-muted); box-shadow: none; }
.a-about p { margin: 0; line-height: 1.6; }
.a-about .a-gap { margin-top: 1.5em; }
.a-about .glim-readme-btn-rows { margin-top: .75rem; }
.a-about .glim-about-give { margin-top: 1.5em; }
.a-coffee-mark { color: var(--brand-coffee); }
.a-newtop { display: flex; justify-content: flex-end; }
.a-release { position: relative; padding-top: 1.4rem; margin-top: .6rem; }
.a-release .notch { left: .9rem; }
.a-release > .a-text { margin: 0 0 .4rem; }
.a-newlist { display: flex; flex-direction: column; gap: 2px; }
.a-newrow { display: grid; grid-template-columns: .5rem 2rem 1fr auto; align-items: center; gap: .6rem; min-height: 2.6rem; padding: 0 .5rem; border-radius: var(--radius-control); background: var(--carbon-surface); }
.a-newrow .tile { width: 1.75rem; height: 1.75rem; border-radius: var(--radius-control); background: var(--carbon-surface2); display: grid; place-items: center; }
.a-newrow .tile svg { width: 1rem; height: 1rem; color: var(--accent-glyph); }
.a-newrow.seen .t { color: var(--carbon-text-muted); }
.a-newrow .btn { background: var(--carbon-surface2); }
.a-stats { display: grid; grid-template-columns: repeat(3, 1fr); gap: .8rem; }
.a-stat { gap: .1rem; }
.a-stat span:first-child { font-size: 22px; font-weight: 600; }
.a-stat span:last-child { font-size: 12px; color: var(--carbon-text-muted); }
.a-actlist { display: flex; flex-direction: column; gap: 2px; margin: 0 -.5rem; }
.a-lhead, .a-arow { display: grid; grid-template-columns: 2.25rem minmax(0, 1fr) 10rem 7.5rem 7.5rem; gap: .6rem; align-items: center; padding: 0 .5rem; }
.a-lhead { height: 1.9rem; font-size: 12px; color: var(--carbon-text-muted); }
.a-arow .pill { justify-self: end; }
.a-hbtn { justify-self: end; display: inline-flex; align-items: center; gap: .3rem; height: 1.6rem; padding: 0 .45rem; border-radius: var(--radius-control); color: inherit; }
.a-hbtn svg { width: .7rem; height: .7rem; }
.a-hbtn:hover { background: var(--carbon-hover); color: var(--carbon-text); }
.a-hbtn.on { background: var(--accent); color: var(--accent-contrast); }
.a-veil { z-index: 90; }
.a-veil-card { display: flex; align-items: center; gap: .7rem; padding: 1rem 1.3rem; }
.a-veil-card svg { width: 1.1rem; height: 1.1rem; color: var(--accent-glyph); animation: a-spin 1s linear infinite; }
@keyframes a-spin { to { rotate: 360deg; } }
@media (max-width: 760px) { .a-lhead, .a-arow { grid-template-columns: 2.25rem minmax(0, 1fr) auto; } .a-lhead > :nth-child(n+3):not(:last-child), .a-arow .pill:not(:last-child) { display: none; } .a-stats { grid-template-columns: 1fr 1fr 1fr; } }
`);
})();
