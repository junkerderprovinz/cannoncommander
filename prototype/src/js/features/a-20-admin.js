// Administration tabs: Quellen, Fernziele, Benutzer und Gruppen, Zugriffsregeln and Freigaben.
(() => {
  const A = CC.admin;
  const { pill, bareBtn, row, list, foot, btn, field, pathField, dropdown, fail } = A;
  const onTab = (id) => CC.view === 'settings' && CC.settingsTab_ === id;
  // A click on a row opens it, a click on one of its buttons does only that button's job.
  const clicks = (host, map) => host.addEventListener('click', e => {
    if (e.target.closest('[data-copy], [data-pickfor], .a-dd, .a-eye')) return;
    for (const [attr, fn] of Object.entries(map)) {
      const el = e.target.closest(`[data-${attr}]`);
      if (el && host.contains(el)) { fn(el.dataset[attr.replace(/-./g, m => m[1].toUpperCase())], el, e); return; }
    }
  });
  const enterOpens = (host) => host.addEventListener('keydown', e => { if (e.key === 'Enter' && e.target.classList.contains('a-row')) e.target.click(); });

  /* Quellen */
  DATA.indexing = DATA.indexing || { every: 'daily', last: NOW - 5.5 * 3600000 };
  const srcGlyph = (s) => s.path === '/boot' ? 'IconFlash' : s.path.startsWith('/mnt/remotes') ? 'IconCloud' : 'IconDatabase';

  function reindex(s) {
    if (s.job) return;
    s.job = Jobs.start({ kind: 'index', label: `Einlesen: ${s.name}`, speed: .7, done: () => {
      s.job = null;
      s.indexed = true;
      if (!s.files) { let n = 0, size = 0; FS.walk(k => { if (!k.dir) { n++; size += k.size || 0; } }, FS.nodeAt(s.path) || ROOT, s.path); Object.assign(s, { files: n, size }); }
      DATA.indexing.last = A.now();
      CC.toast(`${s.name} ist eingelesen`);
      if (onTab('sources')) A.redraw();
    } });
    CC.toast(`${s.name} wird neu eingelesen`, 'ok', { label: 'Aufgaben', run: () => CC.run('jobs') });
  }

  function sourceWin(s) {
    const isNew = !s;
    const d = s || { name: '', path: '/mnt/user', readOnly: false, indexed: true, skipHidden: true, exclude: [] };
    win({
      title: isNew ? 'Quelle hinzufügen' : 'Quelle bearbeiten',
      body: `<div class="well">${frow('Name', field(d.name, 'id="a-sname" placeholder="Etwa Disk 3"', 'm'))}${frow('Ordner', pathField(d.path, 'a-spath'))}</div>
        <div class="well">${frow('Nur lesen', sw(d.readOnly, 'id="a-sro"'), 'Niemand kann hier etwas ändern, löschen oder hochladen, auch Admins nicht.')}
          ${frow('Für die Suche einlesen', sw(d.indexed, 'id="a-six"'), 'Der Index macht die Suche und die Ordnergrößen schnell. Das Einlesen läuft als Aufgabe im Hintergrund.')}
          ${frow('Versteckte Dateien auslassen', sw(d.skipHidden !== false, 'id="a-shid"'), 'Dateien und Ordner, deren Name mit einem Punkt beginnt, kommen nicht in den Index.')}</div>
        <div class="well"><div class="eyebrow">Nicht einlesen ${info('Ein Muster pro Zeile. Ein Stern steht für beliebige Zeichen, etwa *.tmp oder appdata/*/cache.')}</div>
          <textarea class="field" id="a-sex" rows="3" spellcheck="false" placeholder="*.tmp">${esc((d.exclude || []).join('\n'))}</textarea></div>`,
      foot: [cancel, { label: isNew ? 'Hinzufügen' : 'Speichern', g: isNew ? 'IconAdd' : 'IconSave', primary: true, run: (w) => {
        const name = $('#a-sname', w).value.trim(), path = $('#a-spath', w).value.trim().replace(/\/+$/, '') || '/';
        if (!name) return fail('Gib der Quelle einen Namen', $('#a-sname', w));
        if (DATA.sources.some(x => x !== s && x.name.toLowerCase() === name.toLowerCase())) return fail('Diesen Namen gibt es schon', $('#a-sname', w));
        const n = FS.nodeAt(path);
        if (!n || !n.dir) return fail('Diesen Ordner gibt es nicht', $('#a-spath', w));
        if (DATA.sources.some(x => x !== s && x.path === path)) return fail('Dieser Ordner ist schon eine Quelle', $('#a-spath', w));
        const next = { name, path, readOnly: swVal($('#a-sro', w)), indexed: swVal($('#a-six', w)), skipHidden: swVal($('#a-shid', w)), exclude: $('#a-sex', w).value.split('\n').map(x => x.trim()).filter(Boolean) };
        const target = s || { files: 0, size: 0 };
        const startIndex = next.indexed && (isNew || !s.indexed || s.path !== path);
        if (!next.indexed) Object.assign(target, { files: 0, size: 0 });
        if (s && s.path !== path) Object.assign(target, { files: 0, size: 0 });
        Object.assign(target, next, startIndex ? { indexed: false } : {});
        if (isNew) DATA.sources.push(target);
        A.redraw();
        CC.toast(isNew ? `${name} hinzugefügt` : 'Gespeichert');
        if (startIndex) reindex(target);
      } }],
    });
  }

  CC.settingsTab({ id: 'sources', label: 'Quellen', g: 'IconTabStorage', order: 40, render(host) {
    const rows = DATA.sources.map((s, i) => row({
      tile: g(srcGlyph(s)), name: s.name, attr: `data-open="${i}"`,
      badges: [pill(s.path, 'mono'), s.job ? pill('Liest ein', 'run') : s.indexed ? pill(fmtSize(s.size)) + pill(`${s.files.toLocaleString('de-AT')} Dateien`, 'glim-num') : pill('Kein Index', 'state'), s.readOnly ? pill('Nur lesen', 'state') : ''],
      acts: [s.indexed && !s.job ? bareBtn('IconRefresh', 'Neu einlesen', `data-reindex="${i}"`) : '', bareBtn('IconPencil', 'Bearbeiten', `data-open="${i}"`), bareBtn('IconTrash', 'Entfernen', `data-del="${i}"`)],
    }));
    const ix = DATA.indexing;
    host.innerHTML = CC.card('Quellen', list(rows, ['IconDatabase', 'Noch keine Quelle']) + foot(btn('Quelle hinzufügen', 'IconAdd', 'data-add')))
      + CC.card('Index',
        frow('Automatisch neu einlesen', seg([['hourly', 'Stündlich'], ['daily', 'Täglich'], ['manual', 'Nur von Hand']], ix.every, 'id="a-ixevery"'), 'Hält die Suche aktuell, wenn Dateien an CannonCommander vorbei geändert werden, etwa durch andere Container.')
        + frow('Zuletzt eingelesen', `<span class="a-ctl">${pill(A.ago(ix.last))}${btn('Alle neu einlesen', 'IconRefresh', 'data-reindexall')}</span>`));
    clicks(host, {
      reindex: (i) => reindex(DATA.sources[+i]),
      del: (i) => {
        const s = DATA.sources[+i];
        A.confirmWin({ title: 'Quelle entfernen', verb: 'Entfernen', glyph: 'IconTrash',
          text: `<p>${esc(s.name)} verschwindet aus CannonCommander, mit ihrem Index.</p><p>Die Dateien in <span class="mono">${esc(s.path)}</span> bleiben auf dem Server, wie sie sind.</p>`,
          run: () => { DATA.sources = DATA.sources.filter(x => x !== s); A.redraw(); CC.toast(`${s.name} entfernt`); } });
      },
      add: () => sourceWin(null),
      reindexall: () => DATA.sources.filter(s => s.indexed).forEach(reindex),
      open: (i) => sourceWin(DATA.sources[+i]),
    });
    enterOpens(host);
    $('#a-ixevery', host).addEventListener('change', e => { ix.every = segVal(e.target); CC.toast('Gespeichert'); });
  } });

  /* Fernziele */
  DATA.rclone = DATA.rclone || { automount: true, cache: 'writes', cacheSize: 50, version: '1.71.2', path: '/usr/bin/rclone' };
  const TYPES = [
    { id: 'sftp', label: 'SFTP', g: 'IconNetwork', sub: 'Server mit SSH' },
    { id: 'smb', label: 'SMB', g: 'IconNetwork', sub: 'NAS und Windows' },
    { id: 'webdav', label: 'WebDAV', g: 'IconBrowser', sub: 'Nextcloud und mehr' },
    { id: 's3', label: 'S3', g: 'IconCloud', sub: 'AWS, MinIO, R2' },
    { id: 'gdrive', label: 'Google Drive', g: 'IconCloud', sub: 'Anmelden bei Google', oauth: 'Google' },
    { id: 'onedrive', label: 'OneDrive', g: 'IconCloud', sub: 'Anmelden bei Microsoft', oauth: 'Microsoft' },
    { id: 'dropbox', label: 'Dropbox', g: 'IconCloud', sub: 'Anmelden bei Dropbox', oauth: 'Dropbox' },
  ];
  const typeOf = (label) => TYPES.find(t => t.label === label || t.id === label) || TYPES[0];
  const FIELDS = {
    sftp: [{ k: 'host', l: 'Server', ph: 'nas.lan' }, { k: 'port', l: 'Port', def: '22', w: 's' }, { k: 'user', l: 'Benutzer' }, { k: 'pass', l: 'Passwort', pw: true }],
    smb: [{ k: 'host', l: 'Server', ph: 'keller' }, { k: 'share', l: 'Freigabe', ph: 'archiv' }, { k: 'user', l: 'Benutzer' }, { k: 'pass', l: 'Passwort', pw: true }, { k: 'domain', l: 'Domäne', ph: 'WORKGROUP', opt: true }],
    webdav: [{ k: 'url', l: 'Adresse', ph: 'https://cloud.example.org/remote.php/dav/files/jdp', w: 'w' }, { k: 'vendor', l: 'Anbieter', dd: [['nextcloud', 'Nextcloud'], ['owncloud', 'ownCloud'], ['other', 'Andere']] }, { k: 'user', l: 'Benutzer' }, { k: 'pass', l: 'Passwort', pw: true }],
    s3: [{ k: 'provider', l: 'Anbieter', dd: [['aws', 'AWS'], ['minio', 'MinIO'], ['wasabi', 'Wasabi'], ['r2', 'Cloudflare R2'], ['other', 'Andere']] }, { k: 'endpoint', l: 'Endpunkt', ph: 'https://s3.eu-central-1.amazonaws.com', w: 'w', opt: true, tip: 'Nur nötig, wenn es nicht AWS ist.' }, { k: 'region', l: 'Region', ph: 'eu-central-1' }, { k: 'key', l: 'Access Key' }, { k: 'secret', l: 'Secret Key', pw: true }, { k: 'bucket', l: 'Bucket' }],
    oauth: [{ k: 'folder', l: 'Ordner beim Anbieter', ph: '/', opt: true, tip: 'Leer lassen für alles. Sonst nur dieser Ordner, etwa /Fotos.' }],
  };
  const SEED = { gdrive: { account: 'junker@example.org', folder: '' }, 'nas-keller': { host: 'keller', share: 'archiv', user: 'jdp', domain: '' } };
  DATA.remotes.forEach(r => { r.cfg = r.cfg || { ...(SEED[r.name] || {}) }; });

  function remoteWin(r) {
    const isNew = !r;
    const w = A.stepWin({ title: isNew ? 'Fernziel hinzufügen' : 'Fernziel bearbeiten', wide: true });
    let type = r ? typeOf(r.type) : null;
    const draft = r ? { name: r.name, mount: r.mount, ...r.cfg } : {};
    let mountTouched = !!r, tested = false;

    const pick = (q = '') => {
      const shown = TYPES.filter(t => (t.label + ' ' + t.sub).toLowerCase().includes(q.toLowerCase()));
      return shown.length ? shown.map(t => `<button class="a-tile" data-type="${t.id}"><span class="tile">${g(t.g)}</span><span><b>${esc(t.label)}</b><small>${esc(t.sub)}</small></span></button>`).join('')
        : `<div class="empty">${g('IconSearch')}<div>Nichts gefunden. SFTP und WebDAV passen auf fast jeden Server.</div></div>`;
    };
    const showPick = () => {
      w.set(`<div class="a-pickhead"><input class="field m" id="a-rq" placeholder="Typ suchen" aria-label="Typ suchen"></div><div class="a-tiles" id="a-tiles">${pick()}</div>`, [cancel]);
      const tiles = $('#a-tiles', w.el);
      tiles.style.minHeight = tiles.offsetHeight + 'px';
      $('#a-rq', w.el).addEventListener('input', e => { tiles.innerHTML = pick(e.target.value); });
      $('#a-rq', w.el).addEventListener('keydown', e => { if (e.key === 'Escape' && e.target.value) { e.stopPropagation(); e.target.value = ''; tiles.innerHTML = pick(); } });
    };
    const read = () => {
      $$('[data-rf]', w.el).forEach(i => { draft[i.dataset.rf] = i.classList.contains('a-dd') ? i.dataset.val : i.value.trim(); });
      draft.name = $('#a-rname', w.el).value.trim();
      draft.mount = $('#a-rmount', w.el).value.trim();
    };
    const missing = () => {
      for (const f of FIELDS[type.oauth ? 'oauth' : type.id]) {
        if (f.opt || f.dd) continue;
        if (f.pw && !isNew) continue;
        if (!draft[f.k]) return [`${f.l} fehlt noch`, $(`[data-rf="${f.k}"]`, w.el)];
      }
      if (type.oauth && !draft.account) return [`Erst bei ${type.oauth} anmelden`, $('#a-oauth', w.el)];
      return null;
    };
    const showForm = () => {
      const fs = FIELDS[type.oauth ? 'oauth' : type.id];
      const ctl = (f) => f.dd ? dropdown(f.dd, draft[f.k] || f.dd[0][0], `data-rf="${f.k}"`)
        : f.pw ? A.pwField(`data-rf="${f.k}" ${!isNew ? 'placeholder="Unverändert"' : ''}`, 'm')
        : field(draft[f.k] || '', `data-rf="${f.k}" placeholder="${esc(f.ph || '')}"`, f.w || 'm');
      const account = type.oauth ? frow('Konto', draft.account ? `<span class="a-ctl">${pill(draft.account)}${btn('Anderes Konto', 'IconSignIn', 'id="a-oauth"')}</span>` : btn(`Bei ${type.oauth} anmelden`, 'IconSignIn', 'id="a-oauth"'), 'Öffnet die Anmeldung des Anbieters. CannonCommander bekommt nur ein Token, nie dein Passwort.') : '';
      w.set(`<div class="well"><div class="eyebrow">${esc(type.label)}</div>${account}${fs.map(f => frow(f.l + (f.opt ? ' <small>optional</small>' : ''), ctl(f), f.tip)).join('')}</div>
        <div class="well">${frow('Name', field(draft.name || '', 'id="a-rname" placeholder="etwa nas-keller"', 'm'), 'Kleinbuchstaben, Ziffern und Bindestriche. rclone kennt das Fernziel unter diesem Namen.')}
          ${frow('Einhängen unter', pathField(draft.mount || '/mnt/remotes/', 'a-rmount'), 'Unter diesem Ordner erscheinen die Dateien im Dateimanager und für andere Container.')}
          ${frow('Verbindung', btn(tested ? 'Verbunden' : 'Testen', tested ? 'IconCheck' : 'IconHealth', `id="a-rtest" class="${tested ? 'ok' : ''}"`))}</div>`,
        [...(isNew ? [{ label: 'Zurück', g: 'IconBack', run: () => { read(); showPick(); return false; } }] : []), cancel, { label: isNew ? 'Hinzufügen' : 'Speichern', g: isNew ? 'IconAdd' : 'IconSave', primary: true, run: save }]);
      $('#a-rname', w.el).addEventListener('input', e => { if (!mountTouched) $('#a-rmount', w.el).value = '/mnt/remotes/' + e.target.value.trim().toLowerCase(); });
      $('#a-rmount', w.el).addEventListener('input', () => { mountTouched = true; });
      w.el.oninput = () => { if (tested) { tested = false; const b = $('#a-rtest', w.el); b.className = 'btn'; b.innerHTML = g('IconHealth') + 'Testen'; } };
    };
    const save = (el) => {
      read();
      const name = draft.name;
      if (!/^[a-z0-9][a-z0-9_-]*$/.test(name)) return fail(name ? 'Nur Kleinbuchstaben, Ziffern, _ und -' : 'Gib dem Fernziel einen Namen', $('#a-rname', el));
      if (DATA.remotes.some(x => x !== r && x.name === name)) return fail('Diesen Namen gibt es schon', $('#a-rname', el));
      if (!/^\/mnt\/[^/]+\/.+/.test(draft.mount)) return fail('Der Ordner muss unter /mnt liegen, etwa /mnt/remotes/' + name, $('#a-rmount', el));
      const m = missing();
      if (m) return fail(...m);
      const cfg = { ...draft };
      delete cfg.name; delete cfg.mount;
      if (!isNew && !cfg.pass) cfg.pass = r.cfg.pass;
      const remoteLabel = type.id === 'smb' ? `SMB: //${cfg.host}/${cfg.share}` : `rclone: ${name}`;
      const parent = FS.nodeAt('/mnt/remotes');
      if (r) {
        const node = FS.kids(parent).find(k => k.name === FS.split(r.mount).pop());
        if (node) Object.assign(node, { name: FS.split(draft.mount).pop(), remote: remoteLabel });
        Object.assign(r, { name, mount: draft.mount, cfg });
      } else {
        DATA.remotes.push({ name, type: type.label, mount: draft.mount, state: 'verbunden', cfg });
        if (draft.mount.startsWith('/mnt/remotes/') && !FS.kids(parent).some(k => k.name === FS.split(draft.mount).pop())) parent.children.push({ name: FS.split(draft.mount).pop(), dir: true, children: [], time: A.now(), owner: 'nobody', group: 'users', mode: '0775', remote: remoteLabel });
      }
      A.redraw();
      CC.toast(isNew ? `${name} ist eingehängt unter ${draft.mount}` : 'Gespeichert');
    };
    w.el.addEventListener('click', e => {
      const t = e.target.closest('[data-type]');
      if (t) {
        type = TYPES.find(x => x.id === t.dataset.type);
        FIELDS[type.oauth ? 'oauth' : type.id].forEach(f => { if (f.def && !draft[f.k]) draft[f.k] = f.def; });
        tested = false;
        showForm();
        return;
      }
      if (e.target.closest('#a-oauth')) {
        read();
        draft.account = draft.account ? 'familie@example.org' : 'junker@example.org';
        CC.toast(`Angemeldet bei ${type.oauth} als ${draft.account}`);
        showForm();
        return;
      }
      const test = e.target.closest('#a-rtest');
      if (test) {
        read();
        const m = missing();
        if (m) { fail(m[0], test); return; }
        test.disabled = true;
        test.innerHTML = g('IconRefresh') + 'Prüfe';
        setTimeout(() => { tested = true; test.disabled = false; test.className = 'btn ok'; test.innerHTML = g('IconCheck') + 'Verbunden'; }, 700);
      }
    });
    type ? showForm() : showPick();
  }

  CC.settingsTab({ id: 'remotes', label: 'Fernziele', g: 'IconCloud', order: 50, render(host) {
    const rc = DATA.rclone;
    const rows = DATA.remotes.map((r, i) => row({
      tile: g(typeOf(r.type).g), name: r.name, attr: `data-open="${i}"`,
      badges: [pill(r.type), pill(r.mount, 'mono'), r.state !== 'verbunden' ? pill('Ausgehängt', 'state') : ''],
      acts: [r.state === 'verbunden' ? bareBtn('IconUnlink', 'Aushängen', `data-mount="${i}"`) : bareBtn('IconLink', 'Einhängen', `data-mount="${i}"`), bareBtn('IconPencil', 'Bearbeiten', `data-open="${i}"`), bareBtn('IconTrash', 'Entfernen', `data-del="${i}"`)],
    }));
    host.innerHTML = CC.card('Fernziele', list(rows, ['IconCloud', 'Noch kein Fernziel']) + foot(btn('Fernziel hinzufügen', 'IconAdd', 'data-add')))
      + CC.card('Einhängen',
        frow('Beim Start einhängen', sw(rc.automount, 'id="a-rcauto"'), 'Hängt alle Fernziele ein, sobald der Container startet.')
        + frow('Zwischenspeicher', seg([['off', 'Aus'], ['writes', 'Beim Schreiben'], ['full', 'Voll']], rc.cache, 'id="a-rccache"'), 'rclone legt Dateien hier kurz ab, damit Programme sie wie lokale Dateien öffnen können. Voll ist am schnellsten und braucht am meisten Platz.')
        + (rc.cache !== 'off' ? frow('Höchstens', seg([[10, '10 GB'], [50, '50 GB'], [200, '200 GB']], rc.cacheSize, 'id="a-rcsize"'), 'Liegt unter /mnt/cache/appdata/cannoncommander/rclone.') : ''));
    clicks(host, {
      mount: (i) => {
        const r = DATA.remotes[+i];
        r.state = r.state === 'verbunden' ? 'ausgehängt' : 'verbunden';
        A.redraw();
        CC.toast(r.state === 'verbunden' ? `${r.name} ist eingehängt` : `${r.name} ist ausgehängt`);
      },
      del: (i) => {
        const r = DATA.remotes[+i];
        A.confirmWin({ title: 'Fernziel entfernen', verb: 'Entfernen', glyph: 'IconTrash',
          text: `<p>${esc(r.name)} wird ausgehängt und vergessen, mit den Zugangsdaten.</p><p>Die Dateien bei ${esc(r.type)} bleiben, wo sie sind.</p>`,
          run: () => {
            DATA.remotes = DATA.remotes.filter(x => x !== r);
            const parent = FS.nodeAt('/mnt/remotes');
            parent.children = parent.children.filter(k => k.name !== FS.split(r.mount).pop());
            A.redraw();
            CC.toast(`${r.name} entfernt`);
          } });
      },
      add: () => remoteWin(null),
      open: (i) => remoteWin(DATA.remotes[+i]),
    });
    enterOpens(host);
    host.addEventListener('change', e => {
      if (e.target.id === 'a-rcauto') rc.automount = swVal(e.target);
      else if (e.target.id === 'a-rccache') { rc.cache = segVal(e.target); A.redraw(); }
      else if (e.target.id === 'a-rcsize') rc.cacheSize = +segVal(e.target);
      else return;
      CC.toast('Gespeichert');
    });
  } });

  /* Benutzer und Gruppen */
  const ROLES = [['Admin', 'Admin'], ['Nutzer', 'Nutzer'], ['Nur lesen', 'Nur lesen']];
  const initials = (u) => (u.full || u.name).split(/\s+/).map(x => x[0]).join('').slice(0, 2).toUpperCase();
  const admins = () => DATA.users.filter(u => u.role === 'Admin');

  function setMembers(group, names) {
    group.members = names;
    DATA.users.forEach(u => {
      const has = names.includes(u.name);
      u.groups = has ? [...new Set([...u.groups, group.name])] : u.groups.filter(x => x !== group.name);
    });
  }

  function userWin(u) {
    const isNew = !u;
    const d = u || { name: '', full: '', role: 'Nutzer', groups: [], scope: '/mnt/user' };
    const self = u && u.name === CC.user.name;
    let pwOpen = isNew;
    const w = A.stepWin({ title: isNew ? 'Benutzer anlegen' : 'Benutzer bearbeiten' });
    const pwPart = () => pwOpen
      ? frow('Passwort', A.pwField('id="a-upw"', 'm'), 'Mindestens 12 Zeichen.') + frow('Wiederholen', A.pwField('id="a-upw2"', 'm'))
      : frow('Passwort', `<span class="a-ctl">${pill('Gesetzt')}${btn('Neu setzen', 'IconKey', 'id="a-upwset"')}</span>`, self ? 'Dein eigenes Passwort änderst du unter Konto und Sicherheit.' : 'Ein neues Passwort meldet den Benutzer überall ab.');
    const render = () => {
      const v = d;
      w.set(`<div class="well">${frow('Anmeldename', field(v.name, `id="a-uname" ${self ? 'readonly' : ''} placeholder="etwa anna"`, 'm'), 'Kleinbuchstaben, damit meldet sich die Person an.')}${frow('Anzeigename', field(v.full, 'id="a-ufull" placeholder="etwa Anna Huber"', 'm'))}</div>
        <div class="well">${frow('Rolle', seg(ROLES, v.role, `id="a-urole"`), 'Admins dürfen alles, auch hier in den Einstellungen. Nutzer arbeiten mit den Dateien in ihrem Bereich. Nur lesen kann ansehen und herunterladen.')}
          ${frow('Bereich', pathField(v.scope, 'a-uscope'), 'Die Person sieht nur diesen Ordner und alles darin.')}</div>
        <div class="well"><div class="eyebrow">Gruppen</div>${DATA.groups.length ? DATA.groups.map(gr => frow(esc(gr.name), sw(v.groups.includes(gr.name), `data-ug="${esc(gr.name)}"`), gr.note)).join('') : '<div class="muted">Noch keine Gruppe</div>'}</div>
        ${self ? '' : `<div class="well" id="a-upwwell">${pwPart()}</div>`}`,
        [cancel, { label: isNew ? 'Anlegen' : 'Speichern', g: isNew ? 'IconAdd' : 'IconSave', primary: true, run: save }]);
    };
    const current = () => ({ name: $('#a-uname', w.el).value.trim(), full: $('#a-ufull', w.el).value.trim(), role: segVal($('#a-urole', w.el)), scope: $('#a-uscope', w.el).value.trim(), groups: $$('[data-ug]', w.el).filter(swVal).map(x => x.dataset.ug) });
    const save = (el) => {
      const c = current();
      if (!/^[a-z][a-z0-9._-]{1,31}$/.test(c.name)) return fail(c.name ? 'Der Anmeldename braucht Kleinbuchstaben, mindestens zwei Zeichen' : 'Der Anmeldename fehlt noch', $('#a-uname', el));
      if (DATA.users.some(x => x !== u && x.name === c.name)) return fail('Diesen Anmeldenamen gibt es schon', $('#a-uname', el));
      const n = FS.nodeAt(c.scope);
      if (!n || !n.dir) return fail('Diesen Ordner gibt es nicht', $('#a-uscope', el));
      if (self && c.role !== 'Admin') return fail('Deine eigene Admin-Rolle kann dir nur ein anderer Admin nehmen', $('#a-urole', el));
      if (u && u.role === 'Admin' && c.role !== 'Admin' && admins().length === 1) return fail('Es braucht mindestens einen Admin', $('#a-urole', el));
      if (pwOpen) {
        const p1 = $('#a-upw', el).value, p2 = $('#a-upw2', el).value;
        if (p1.length < 12) return fail('Das Passwort braucht mindestens 12 Zeichen', $('#a-upw', el));
        if (p1 !== p2) return fail('Die beiden Passwörter sind nicht gleich', $('#a-upw2', el));
      }
      const target = u || { last: null, twofa: false };
      const oldName = target.name;
      Object.assign(target, { name: c.name, full: c.full || c.name, role: c.role, scope: c.scope, groups: c.groups });
      if (isNew) DATA.users.push(target);
      if (oldName && oldName !== c.name) DATA.rules.forEach(r => { if (r.who === oldName) r.who = c.name; });
      DATA.groups.forEach(gr => { gr.members = gr.members.filter(m => m !== oldName && m !== c.name); if (c.groups.includes(gr.name)) gr.members.push(c.name); });
      A.redraw();
      CC.toast(isNew ? `${target.full} angelegt` : pwOpen ? 'Gespeichert, das neue Passwort gilt ab sofort' : 'Gespeichert');
    };
    w.el.addEventListener('click', e => {
      if (e.target.closest('#a-upwset')) { pwOpen = true; $('#a-upwwell', w.el).innerHTML = pwPart(); $('#a-upw', w.el).focus(); }
    });
    render();
  }

  function groupWin(gr) {
    const isNew = !gr;
    const d = gr || { name: '', note: '', members: [] };
    win({
      title: isNew ? 'Gruppe anlegen' : 'Gruppe bearbeiten',
      body: `<div class="well">${frow('Name', field(d.name, 'id="a-gname" placeholder="etwa familie"', 'm'))}${frow('Beschreibung', field(d.note, 'id="a-gnote" placeholder="Wofür die Gruppe da ist"', 'w'))}</div>
        <div class="well"><div class="eyebrow">Mitglieder</div>${DATA.users.map(u => frow(`${esc(u.full)} <small>${esc(u.name)}</small>`, sw(d.members.includes(u.name), `data-gm="${esc(u.name)}"`))).join('')}</div>`,
      foot: [cancel, { label: isNew ? 'Anlegen' : 'Speichern', g: isNew ? 'IconAdd' : 'IconSave', primary: true, run: (w) => {
        const name = $('#a-gname', w).value.trim();
        if (!/^[a-z0-9][a-z0-9_-]*$/.test(name)) return fail(name ? 'Nur Kleinbuchstaben, Ziffern, _ und -' : 'Gib der Gruppe einen Namen', $('#a-gname', w));
        if (DATA.groups.some(x => x !== gr && x.name === name)) return fail('Diese Gruppe gibt es schon', $('#a-gname', w));
        const target = gr || { members: [] };
        const old = target.name;
        if (old && old !== name) {
          DATA.users.forEach(u => { u.groups = u.groups.map(x => x === old ? name : x); });
          DATA.rules.forEach(r => { if (r.who === '@' + old) r.who = '@' + name; });
        }
        Object.assign(target, { name, note: $('#a-gnote', w).value.trim() });
        if (isNew) DATA.groups.push(target);
        setMembers(target, $$('[data-gm]', w).filter(swVal).map(x => x.dataset.gm));
        A.redraw();
        CC.toast(isNew ? `Gruppe ${name} angelegt` : 'Gespeichert');
      } }],
    });
  }

  CC.settingsTab({ id: 'users', label: 'Benutzer & Gruppen', g: 'IconAccounts', order: 60, render(host) {
    const urows = DATA.users.map((u, i) => row({
      tile: `<i>${esc(initials(u))}</i>`, name: u.full, attr: `data-uopen="${i}"`,
      tip: `Meldet sich als ${u.name} an.${u.groups.length ? ` Gruppen: ${u.groups.join(', ')}.` : ''}${u.twofa ? ' Mit Zwei-Faktor-Authentifizierung.' : ''}`,
      badges: [u.name === CC.user.name ? pill('Du', 'state') : '', pill(u.role), pill(u.scope, 'mono'), pill(u.last ? A.ago(u.last) : 'Noch nie angemeldet')],
      acts: [bareBtn('IconPencil', 'Bearbeiten', `data-uopen="${i}"`), u.name === CC.user.name ? '' : bareBtn('IconTrash', 'Entfernen', `data-udel="${i}"`)],
    }));
    const grows = DATA.groups.map((gr, i) => row({
      tile: g('IconAccounts'), name: gr.name, tip: gr.note || null, attr: `data-gopen="${i}"`,
      badges: [pill(gr.members.length === 1 ? '1 Mitglied' : `${gr.members.length} Mitglieder`)],
      acts: [bareBtn('IconPencil', 'Bearbeiten', `data-gopen="${i}"`), bareBtn('IconTrash', 'Entfernen', `data-gdel="${i}"`)],
    }));
    host.innerHTML = CC.card('Benutzer', list(urows, ['IconAccounts', 'Noch niemand angelegt']) + foot(btn('Benutzer anlegen', 'IconAdd', 'data-uadd')))
      + CC.card('Gruppen', list(grows, ['IconAccounts', 'Noch keine Gruppe']) + foot(btn('Gruppe anlegen', 'IconAdd', 'data-gadd')));
    clicks(host, {
      udel: (i) => {
        const u = DATA.users[+i];
        if (u.role === 'Admin' && admins().length === 1) return fail('Es braucht mindestens einen Admin');
        const shares = DATA.shares.filter(s => s.by === u.name).length;
        A.confirmWin({ title: 'Benutzer entfernen', verb: 'Entfernen', glyph: 'IconTrash',
          text: `<p>${esc(u.full)} kann sich danach nicht mehr anmelden${shares ? `, und ${shares === 1 ? 'die Freigabe' : `die ${shares} Freigaben`} von ${esc(u.name)} enden` : ''}.</p><p>Die Dateien bleiben, wo sie sind.</p>`,
          run: () => {
            DATA.users = DATA.users.filter(x => x !== u);
            DATA.groups.forEach(gr => { gr.members = gr.members.filter(m => m !== u.name); });
            DATA.rules = DATA.rules.filter(r => r.who !== u.name);
            DATA.shares = DATA.shares.filter(s => s.by !== u.name);
            A.redraw();
            CC.toast(`${u.full} entfernt`);
          } });
      },
      gdel: (i) => {
        const gr = DATA.groups[+i];
        A.confirmWin({ title: 'Gruppe entfernen', verb: 'Entfernen', glyph: 'IconTrash',
          text: `<p>Die Gruppe ${esc(gr.name)} und ihre Zugriffsregeln verschwinden.</p><p>${gr.members.length ? 'Ihre Mitglieder bleiben als Benutzer erhalten.' : 'Sie hat keine Mitglieder.'}</p>`,
          run: () => {
            DATA.groups = DATA.groups.filter(x => x !== gr);
            DATA.users.forEach(u => { u.groups = u.groups.filter(x => x !== gr.name); });
            DATA.rules = DATA.rules.filter(r => r.who !== '@' + gr.name);
            A.redraw();
            CC.toast(`Gruppe ${gr.name} entfernt`);
          } });
      },
      uadd: () => userWin(null),
      gadd: () => groupWin(null),
      uopen: (i) => userWin(DATA.users[+i]),
      gopen: (i) => groupWin(DATA.groups[+i]),
    });
    enterOpens(host);
  } });

  /* Zugriffsregeln */
  const RULES = [['verbergen', 'Verbergen', 'IconEyeOff'], ['nur lesen', 'Nur lesen', 'IconEye'], ['erlauben', 'Erlauben', 'IconCheck']];
  const ruleLabel = (r) => (RULES.find(x => x[0] === r) || RULES[0])[1];
  const whoLabel = (who) => who === 'alle' ? 'Alle' : who.startsWith('@') ? `Gruppe ${who.slice(1)}` : (DATA.users.find(u => u.name === who)?.full || who);
  const glob = (pat, s) => new RegExp('^' + pat.replace(/[.+?^${}()|[\]\\]/g, '\\$&').replace(/\*/g, '.*') + '$', 'i').test(s);
  const matches = (r, user, path) => {
    const who = r.who === 'alle' || r.who === user.name || (r.who.startsWith('@') && user.groups.includes(r.who.slice(1)));
    if (!who) return false;
    if (r.path.includes('*')) return FS.split(path).some(part => glob(r.path, part)) || glob(r.path, path);
    return path === r.path || path.startsWith(r.path + '/');
  };
  function verdict(user, path) {
    if (!(path === user.scope || path.startsWith(user.scope + '/'))) return ['Verborgen', 'state', `Liegt außerhalb des Bereichs ${user.scope}.`];
    const i = DATA.rules.findIndex(r => matches(r, user, path));
    if (i < 0) return [user.role === 'Nur lesen' ? 'Nur lesen' : 'Sichtbar', user.role === 'Nur lesen' ? 'state' : 'ok', user.role === 'Nur lesen' ? 'Keine Regel passt, die Rolle erlaubt nur Lesen.' : 'Keine Regel passt, es gilt der Bereich.'];
    const r = DATA.rules[i];
    const out = r.rule === 'verbergen' ? ['Verborgen', 'state'] : r.rule === 'nur lesen' || user.role === 'Nur lesen' ? ['Nur lesen', 'state'] : ['Sichtbar', 'ok'];
    return [...out, `Regel ${i + 1}: ${whoLabel(r.who)}, ${r.path}, ${ruleLabel(r.rule)}.`];
  }

  function ruleWin(rule) {
    const isNew = !rule;
    const d = rule ? { ...rule } : { who: 'alle', path: '', rule: 'verbergen' };
    let kind = d.who === 'alle' ? 'all' : d.who.startsWith('@') ? 'group' : 'user';
    const w = A.stepWin({ title: isNew ? 'Regel anlegen' : 'Regel bearbeiten' });
    const render = () => {
      const whoCtl = kind === 'user' ? frow('Benutzer', dropdown(DATA.users.map(u => [u.name, u.full]), d.who, 'id="a-rwho"'))
        : kind === 'group' ? (DATA.groups.length ? frow('Gruppe', dropdown(DATA.groups.map(gr => ['@' + gr.name, gr.name]), d.who, 'id="a-rwho"')) : frow('Gruppe', pill('Noch keine Gruppe', 'state'))) : '';
      w.set(`<div class="well">${frow('Gilt für', seg([['user', 'Benutzer'], ['group', 'Gruppe'], ['all', 'Alle']], kind, 'id="a-rkind"'))}${whoCtl}</div>
        <div class="well">${frow('Pfad oder Muster', pathField(d.path, 'a-rpath', { attr: 'placeholder="/mnt/user/backups oder *.tmp"' }), 'Ein Pfad gilt für den Ordner und alles darin. Ein Muster mit Stern passt auf Namen überall, etwa *.partial*.')}
          ${frow('Regel', seg(RULES, d.rule, 'id="a-rrule"'), 'Erlauben hebt eine Regel weiter unten auf, etwa für einen Unterordner. Die erste passende Regel gilt.')}</div>`,
        [cancel, { label: isNew ? 'Anlegen' : 'Speichern', g: isNew ? 'IconAdd' : 'IconSave', primary: true, run: save }]);
      $('#a-rkind', w.el).addEventListener('change', e => {
        read();
        kind = segVal(e.target);
        d.who = kind === 'all' ? 'alle' : kind === 'group' ? (DATA.groups[0] ? '@' + DATA.groups[0].name : '') : DATA.users[0].name;
        render();
      });
    };
    const read = () => {
      d.path = $('#a-rpath', w.el).value.trim();
      d.rule = segVal($('#a-rrule', w.el));
      const who = $('#a-rwho', w.el);
      if (who) d.who = A.ddVal(who);
    };
    const save = (el) => {
      read();
      if (kind === 'group' && !d.who) return fail('Leg erst eine Gruppe an', $('#a-rkind', el));
      if (!d.path) return fail('Pfad oder Muster fehlt noch', $('#a-rpath', el));
      if (!d.path.includes('*')) {
        d.path = d.path.replace(/\/+$/, '') || '/';
        const n = FS.nodeAt(d.path);
        if (!d.path.startsWith('/') || !n) return fail('Diesen Pfad gibt es nicht. Für ein Muster nimm einen Stern, etwa *.tmp', $('#a-rpath', el));
      }
      if (rule) Object.assign(rule, d); else DATA.rules.push(d);
      A.redraw();
      CC.toast(isNew ? 'Regel angelegt' : 'Gespeichert');
    };
    render();
  }

  let probe = { who: 'anna', path: '/mnt/user/backups' };
  CC.settingsTab({ id: 'rules', label: 'Zugriffsregeln', g: 'IconShieldOn', order: 70, render(host) {
    const n = DATA.rules.length;
    const rows = DATA.rules.map((r, i) => row({
      tile: g((RULES.find(x => x[0] === r.rule) || RULES[0])[2]), name: r.path, mono: true, attr: `data-open="${i}"`,
      badges: [pill(whoLabel(r.who)), pill(ruleLabel(r.rule), 'state')],
      acts: [i > 0 ? bareBtn('IconMoveUp', 'Nach oben, die erste passende Regel gilt', `data-up="${i}"`) : '', i < n - 1 ? bareBtn('IconMoveDown', 'Nach unten', `data-down="${i}"`) : '', bareBtn('IconPencil', 'Bearbeiten', `data-open="${i}"`), bareBtn('IconTrash', 'Entfernen', `data-del="${i}"`)],
    }));
    if (!DATA.users.some(u => u.name === probe.who)) probe.who = DATA.users[0].name;
    const user = DATA.users.find(u => u.name === probe.who);
    const [word, tone, why] = verdict(user, probe.path);
    host.innerHTML = CC.card('Regeln', list(rows, ['IconShieldOn', 'Keine Regeln, jeder sieht seinen ganzen Bereich']) + foot(btn('Regel anlegen', 'IconAdd', 'data-add')))
      + CC.card('Regel prüfen',
        frow('Wer', dropdown(DATA.users.map(u => [u.name, u.full]), probe.who, 'id="a-pwho"'))
        + frow('Pfad', pathField(probe.path, 'a-ppath'))
        + frow('Ergebnis', `<span class="a-ctl" id="a-pres">${pill(word, tone)}${info(why)}</span>`, 'Was diese Person an diesem Pfad sieht, nach ihrem Bereich, ihrer Rolle und den Regeln.'));
    const move = (i, d) => { const r = DATA.rules.splice(i, 1)[0]; DATA.rules.splice(i + d, 0, r); A.redraw(); };
    clicks(host, {
      up: (i) => move(+i, -1),
      down: (i) => move(+i, 1),
      del: (i) => {
        const r = DATA.rules.splice(+i, 1)[0];
        A.redraw();
        CC.toast('Regel entfernt', 'ok', { label: 'Rückgängig', run: () => { DATA.rules.splice(+i, 0, r); A.redraw(); } });
      },
      add: () => ruleWin(null),
      open: (i) => ruleWin(DATA.rules[+i]),
    });
    enterOpens(host);
    const update = () => {
      probe = { who: A.ddVal($('#a-pwho', host)), path: $('#a-ppath', host).value.trim() };
      const u = DATA.users.find(x => x.name === probe.who);
      const [w2, t2, y2] = FS.nodeAt(probe.path) ? verdict(u, probe.path) : ['Gibt es nicht', 'warn', 'Diesen Pfad gibt es auf dem Server nicht.'];
      $('#a-pres', host).innerHTML = pill(w2, t2) + info(y2);
    };
    host.addEventListener('change', e => { if (e.target.closest('#a-pwho')) update(); });
    $('#a-ppath', host).addEventListener('input', update);
  } });

  /* Freigaben */
  const shareUrl = (s) => `https://cannon.tower.lan/s/${s.slug}`;
  const shareName = (s) => s.name || FS.split(s.path).pop();
  const EXPIRY = [['keep', 'Wie bisher'], ['1', '1 Tag ab jetzt'], ['7', '7 Tage ab jetzt'], ['30', '30 Tage ab jetzt'], ['never', 'Nie']];

  function shareWin(s) {
    let pwOn = s.password, pwNew = false;
    const w = A.stepWin({ title: 'Freigabe' });
    const pwRow = () => frow('Passwort verlangen', sw(pwOn, 'id="a-shpw"'), 'Wer den Link öffnet, muss erst das Passwort eingeben.')
      + (!pwOn ? '' : s.password && !pwNew ? frow('Passwort', `<span class="a-ctl">${pill('Gesetzt')}${btn('Neu setzen', 'IconKey', 'id="a-shpwnew"')}</span>`) : frow('Passwort', A.pwField('id="a-shpw1"', 'm'), 'Mindestens 6 Zeichen.'));
    w.set(`<div class="well">${frow('Link', A.copyField(shareUrl(s)))}${frow('Pfad', `<span class="pill mono">${esc(s.path)}</span>`)}${frow('Erstellt', pill(`${fmtDate(s.created)} von ${s.by}`))}${frow('Aufrufe', pill(String(s.views), 'glim-num'))}</div>
      <div class="well">${frow('Nachricht an die Gegenseite', `<textarea class="field w" id="a-shmsg" rows="2" maxlength="2000" placeholder="etwa: Hier sind die Fotos vom Wochenende">${esc(s.message || '')}</textarea>`, 'Steht auf der Seite des Links über den Dateien, etwa ein Gruß oder ein Hinweis. Darf leer bleiben.')}${frow('Läuft ab', dropdown(EXPIRY.map(([v, l]) => [v, v === 'keep' ? (s.expires ? `${l}, ${fmtDate(s.expires)}` : `${l}, nie`) : l]), 'keep', 'id="a-shexp"'))}<div id="a-shpwbox">${pwRow()}</div></div>`,
      [cancel, { label: 'Speichern', g: 'IconSave', primary: true, run: (el) => {
        const pw1 = $('#a-shpw1', el);
        if (pw1 && pw1.value.length < 6) return fail('Das Passwort braucht mindestens 6 Zeichen', pw1);
        const exp = A.ddVal($('#a-shexp', el));
        if (exp === 'never') s.expires = null;
        else if (exp !== 'keep') s.expires = A.now() + Number(exp) * DAY;
        s.password = pwOn;
        s.message = $('#a-shmsg', el).value.trim();
        A.redraw();
        CC.toast('Gespeichert');
      } }]);
    const box = $('#a-shpwbox', w.el);
    box.addEventListener('change', e => { if (e.target.id === 'a-shpw') { pwOn = swVal(e.target); box.innerHTML = pwRow(); wireControls(box); } });
    box.addEventListener('click', e => { if (e.target.closest('#a-shpwnew')) { pwNew = true; box.innerHTML = pwRow(); wireControls(box); $('#a-shpw1', box).focus(); } });
  }

  CC.settingsTab({ id: 'shares', label: 'Freigaben', g: 'IconLink', order: 80, render(host) {
    const rows = DATA.shares.map((s, i) => {
      const left = s.expires ? s.expires - A.now() : null;
      return row({
        tile: g(s.items ? 'IconCollector' : FS.nodeAt(s.path)?.dir ? 'IconFolder' : 'IconFile'), name: shareName(s), tip: s.path, attr: `data-open="${i}"`,
        badges: [left == null ? pill('Läuft nicht ab') : left < 0 ? pill('Abgelaufen', 'state') : pill(A.ago(s.expires), left < 3 * DAY ? 'warn' : ''), s.password ? pill('Passwort') : '', s.limit ? pill(`höchstens ${s.limit} Downloads`, 'glim-num') : '', pill(s.views === 1 ? '1 Aufruf' : `${s.views} Aufrufe`, 'glim-num')],
        acts: [bareBtn('IconCopy', 'Link kopieren', `data-copy="${esc(shareUrl(s))}"`), bareBtn('IconPencil', 'Bearbeiten', `data-open="${i}"`), bareBtn('IconUnlink', 'Widerrufen', `data-revoke="${i}"`)],
      });
    });
    host.innerHTML = CC.card('Freigaben', list(rows, ['IconLink', 'Keine Freigaben. Teilen kannst du über Datei, Freigeben.']));
    clicks(host, {
      revoke: (i) => {
        const s = DATA.shares.splice(+i, 1)[0];
        A.redraw();
        CC.toast(`Der Link zu ${shareName(s)} gilt nicht mehr`, 'ok', { label: 'Rückgängig', run: () => { DATA.shares.splice(+i, 0, s); A.redraw(); } });
      },
      open: (i) => shareWin(DATA.shares[+i]),
    });
    enterOpens(host);
  } });

  CC.css(`
.pill.run { background: var(--accent); color: var(--accent-contrast); }
.a-pickhead { display: flex; justify-content: flex-end; }
.a-tiles { display: grid; grid-template-columns: repeat(auto-fill, minmax(13rem, 1fr)); gap: .8125rem; align-content: start; }
.a-tile { display: flex; align-items: center; gap: .7rem; height: 3.4rem; padding: 0 .9rem 0 .6rem; border-radius: var(--radius-pill); background: var(--carbon-surface2); text-align: left; }
.a-tile:hover, .a-tile:focus-visible { background: var(--accent); color: var(--accent-contrast); }
.a-tile:hover .tile, .a-tile:focus-visible .tile { background: color-mix(in srgb, var(--accent-contrast) 14%, transparent); }
.a-tile:hover .tile svg, .a-tile:focus-visible .tile svg { color: var(--accent-contrast); }
.a-tile .tile { background: var(--carbon-surface3); }
.a-tile .tile svg { color: var(--accent-glyph); }
.a-tile b { display: block; font-weight: 600; }
.a-tile small { display: block; font-size: 12px; opacity: .75; }
.a-tiles .empty { grid-column: 1 / -1; }
`);
})();
