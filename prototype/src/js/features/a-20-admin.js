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
  const srcGlyph = (s) => s.path === '/boot' ? 'IconFlash' : s.path.startsWith('/remotes') ? 'IconCloud' : 'IconDatabase';

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

  CC.settingsTab({ id: 'sources', label: 'Quellen', g: 'IconTabStorage', order: 50, render(host) {
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
  const RT = REMOTE_TEXTS.config;
  const say = (text, vars = {}) => text.replace(/\{(\w+)\}/g, (_, k) => vars[k] ?? '');
  const GROUPS = ['cloud', 'storage', 'protocol'];
  const WORD_NAMES = new Set(['smb', 's3', 'crypt', 'alias', 'archive', 'cache', 'chunker', 'combine', 'compress', 'hasher', 'union', 'memory']);
  const pName = (p) => (WORD_NAMES.has(p.id) ? RT.p[p.id] : p.name);
  const nameAndKind = (label) => { const at = label.indexOf(' / '); return at < 0 ? { name: label } : { name: label.slice(0, at), sub: label.slice(at + 3) }; };
  const HINTS = { smb: 'smb', sftp: 'sftp', webdav: 'webdav', ftp: 'ftp', http: 'http', crypt: 'crypt', s3: 's3', hidrive: 'hidrive', icloud: 'icloud',
    googlephotos: 'googlephotos', minio: 'selfHosted', seaweedfs: 'selfHosted', ceph: 'selfHosted', garage: 'selfHosted', doi: 'doi', alias: 'alias',
    archive: 'archive', cache: 'cache', chunker: 'chunker', combine: 'combine', compress: 'compress', hasher: 'hasher', union: 'union', memory: 'memory' };
  const pHint = (id) => (HINTS[id] ? RT.ph[HINTS[id]] : '');
  const markOf = (mark) => (BRANDS.marks[mark] ? { svg: BRANDS.marks[mark], tile: BRANDS.tiles[mark] } : { glyph: mark && GLYPHS[mark] ? mark : 'IconCloud' });
  const markHtml = (m) => (m.svg ? `<span class="glim-brand-svg" aria-hidden="true">${m.svg}</span>` : g(m.glyph));
  const LABELS = { user: 'user', username: 'user', pass: 'pass', password: 'pass', host: 'host', url: 'url', port: 'port', domain: 'domain', vendor: 'vendor',
    provider: 'provider', region: 'region', endpoint: 'endpoint', account: 'account', key: 'key', key_file: 'keyFile', access_key_id: 'accessKey',
    secret_access_key: 'secretKey', access_grant: 'accessGrant', client_id: 'clientId', client_secret: 'clientSecret', remote: 'remote', library: 'library',
    hostname: 'hostname', scope: 'scope', '2fa': 'twoFactor', share_name: 'shareName', upstreams: 'upstreams', access_token: 'accessToken', api_key: 'apiKey',
    api_url: 'apiUrl', api_password: 'apiPassword', passphrase: 'passphrase', tls: 'tls', explicit_tls: 'explicitTls', host_keys: 'hostKeys' };
  const optLabel = (n) => (LABELS[n] ? RT.o[LABELS[n]] : n);
  const OWN = { url: 'url', user: 'user', username: 'user', pass: 'pass', password: 'pass', host: 'host', port: 'port', access_key_id: 'accessKey',
    secret_access_key: 'secretKey', endpoint: 'endpoint', region: 'region', vendor: 'vendor', key_file: 'keyFile', remote: 'remote', upstreams: 'upstreams',
    passphrase: 'passphrase', host_keys: 'hostKeys' };
  const SECRET_FIELDS = new Set(['pass', 'password', 'secret_access_key', 'key', 'api_key', 'token', 'access_token', 'private_key']);
  const AUTH = { apppassword: 'authAppPassword', accesskey: 'authAccessKey', apikey: 'authApiKey', login: 'authLogin', logintoken: 'authLoginToken' };
  function optHint(o, p, type) {
    let own = OWN[o.name] ? RT.oh[OWN[o.name]] : '';
    if ((o.name === 'pass' || o.name === 'password') && type === 'webdav') own = RT.oh.passCloud;
    const about = [];
    if (o.name === 'url' && p?.urlHint) about.push(say(RT.oh.shape, { shape: p.urlHint }));
    if (p?.defaults?.[o.name]) about.push(say(RT.oh.serverDefault, { value: p.defaults[o.name] }));
    if (SECRET_FIELDS.has(o.name) && p?.auth && AUTH[p.auth] && type !== 'webdav') { about.push(RT.oh[AUTH[p.auth]]); own = ''; }
    if (SECRET_FIELDS.has(o.name) && p?.authUrl) about.push(say(RT.oh.authWhere, { url: p.authUrl }));
    const named = LABELS[o.name] ? say(RT.oh.rcloneName, { name: o.name }) : '';
    return own || about.length || named ? [own, ...about, named].filter(Boolean).join(' ') : o.help || '';
  }
  const applies = (filter, value) => {
    if (!filter) return true;
    const not = filter.startsWith('!');
    const hit = !!value && (not ? filter.slice(1) : filter).split(',').includes(value);
    return not ? !hit : hit;
  };
  const suggest = (product, taken) => {
    const base = product.toLowerCase().normalize('NFD').replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || 'remote';
    let n = base;
    for (let k = 2; taken.includes(n); k++) n = `${base}-${k}`;
    return n;
  };
  const kindOf = (r) => { const p = REMOTES.providers.find(x => x.id === r.provider); return p ? pName(p) : r.type; };
  const remoteMark = (r) => markOf(REMOTES.providers.find(x => x.id === r.provider)?.mark);
  const PICKED = new Set(['vendor', 'provider']), WIDE = new Set(['url', 'endpoint', 'host_keys', 'remote', 'upstreams']);
  const SEED = { gdrive: {}, 'nas-keller': { host: 'keller', user: 'jdp', domain: '' } };
  DATA.remotes.forEach(r => { r.cfg = r.cfg || { ...(SEED[r.name] || {}) }; });

  function remoteWin(r) {
    const isNew = !r;
    const w = A.stepWin({ title: isNew ? RT.add : RT.edit, wide: true });
    const taken = DATA.remotes.filter(x => x !== r).map(x => x.name);
    let provider = r ? REMOTES.providers.find(p => p.id === r.provider) || null : null;
    let typeId = r ? r.type : '';
    const values = r ? { ...r.cfg } : {};
    let name = r?.name || '', mount = r?.mount || '/remotes/', path = r?.path || '';
    let nameTouched = !!r, mountTouched = !!r, signedIn = !!r && r.type === 'drive', signing = false;
    let result = null, testing = false, more = false, shares = [];
    const type = () => REMOTES.types.find(t => t.id === typeId);
    const who = () => (provider ? pName(provider) : type()?.description || typeId);

    const tile = (p) => {
      const m = markOf(p.mark), lit = m.tile, { name: n, sub } = nameAndKind(pName(p)), hint = pHint(p.id), bubble = p.group === 'protocol' ? hint : '';
      return `<li data-type="${p.id}" ${!bubble && hint ? `title="${esc(hint)}"` : ''} ${lit ? `style="--provider-tile:${lit.tile};--provider-ink:${lit.ink}"` : ''}>
        <div class="glim-readme-btn-unit group glim-tile-house"><button type="button" class="glim-readme-btn glim-brand-tile" aria-label="${esc(sub ? n + ' ' + sub : n)}">
        <span class="glim-readme-btn-mark ${m.svg ? '' : 'a-pglyph'}" aria-hidden="true">${markHtml(m)}</span><span class="glim-readme-btn-text"><span class="glim-readme-btn-name">${esc(n)}</span>${sub ? `<span class="glim-readme-btn-sub">${esc(sub)}</span>` : ''}</span></button>
        <span class="glim-readme-btn-sheen" aria-hidden="true"></span>${bubble ? `<span class="glim-readme-btn-hint">${info(bubble)}</span>` : ''}</div></li>`;
    };
    const pick = (q = '') => {
      q = q.trim().toLowerCase();
      const hit = (words) => !q || words.some(x => x.toLowerCase().includes(q));
      const sorted = [...REMOTES.providers].sort((a, b) => pName(a).localeCompare(pName(b), undefined, { sensitivity: 'base' }));
      const groups = GROUPS.map(gr => ({ gr, ps: sorted.filter(p => p.group === gr && hit([pName(p), p.name, p.backend])), un: gr === 'protocol' ? REMOTES.unlisted.filter(b => hit([b])) : [] }))
        .filter(x => x.ps.length + x.un.length);
      if (!groups.length) {
        const nm = (id, fb) => { const p = REMOTES.providers.find(x => x.id === id); return p ? nameAndKind(pName(p)).name : fb; };
        return `<p class="a-pnone">${esc(say(RT.searchNone, { query: q, s3: nm('s3', 'S3'), webdav: nm('webdav', 'WebDAV') }))}</p>`;
      }
      return groups.map(x => `<section class="a-pgroup" data-group="${x.gr}"><h3 class="eyebrow">${esc(RT.g[x.gr])}</h3><ul class="glim-provider-pick">${x.ps.map(tile).join('')}</ul>
        ${x.un.length ? `<h4 class="a-pmore">${esc(say(RT.pickMore, { count: x.un.length }))}</h4><ul class="a-ppills">${x.un.map(b => `<li><button type="button" class="a-ppill" data-type="${esc(b)}">${esc(b)}</button></li>`).join('')}</ul>` : ''}</section>`).join('');
    };
    const showPick = () => {
      typeId = '';
      w.set(`<div class="a-pickhead"><input class="field m" id="a-rq" placeholder="${esc(RT.search)}" aria-label="${esc(RT.search)}"></div><div class="a-picks" id="a-picks">${pick()}</div>`, [cancel]);
      const picks = $('#a-picks', w.el);
      picks.style.minHeight = picks.offsetHeight + 'px';
      $('#a-rq', w.el).addEventListener('input', e => { picks.innerHTML = pick(e.target.value); });
      $('#a-rq', w.el).addEventListener('keydown', e => { if (e.key === 'Escape' && e.target.value) { e.stopPropagation(); e.target.value = ''; picks.innerHTML = pick(); } });
    };

    const split = () => {
      const t = type(), preset = provider?.preset || {};
      const shown = t.options.filter(o => applies(o.provider, values.provider) && !(t.oauth && o.name === 'token'));
      const first = shown.filter(o => (o.required || o.essential) && preset[o.name] === undefined);
      return { first, rest: shown.filter(o => !first.includes(o)) };
    };
    const control = (o) => {
      const ch = (o.examples || []).filter(e => applies(e.provider, values.provider)).map(e => [e.value, e.help || e.value]);
      const ph = provider?.defaults?.[o.name] || o.default || ch[0]?.[0] || '';
      if (o.type === 'bool') return sw((values[o.name] || o.default) === 'true', `data-rf="${o.name}"`);
      if ((o.exclusive || PICKED.has(o.name)) && ch.length) return dropdown([['', ph], ...ch].filter((c, k) => k || !ch.some(x => x[0] === '')), values[o.name] || '', `data-rf="${o.name}"`);
      if (o.secret) return A.pwField(`data-rf="${o.name}" value="${esc(values[o.name] || '')}" placeholder="${esc(r?.secrets?.includes(o.name) ? RT.unchanged : ph)}"`, 'm mono');
      return field(values[o.name] || '', `data-rf="${o.name}" placeholder="${esc(ph)}" spellcheck="false"`, WIDE.has(o.name) ? 'w mono' : 'm mono');
    };
    const optRow = (o) => frow(esc(optLabel(o.name)), control(o), optHint(o, provider, typeId));
    const verdict = () => {
      if (!result) return '';
      if (result.ok) return result.free != null ? say(RT.okFree, { free: fmtSize(result.free) }) : RT.ok;
      return say(RT.fail.other, { error: result.error || '' });
    };
    const read = () => {
      $$('[data-rf]', w.el).forEach(i => {
        const v = i.classList.contains('a-dd') ? i.dataset.val : i.classList.contains('sw') ? String(i.getAttribute('aria-checked') === 'true') : i.value.trim();
        if (i.dataset.rf === 'path') path = v; else values[i.dataset.rf] = v;
      });
      if ($('#a-rname', w.el)) { name = $('#a-rname', w.el).value.trim(); mount = $('#a-rmount', w.el).value.trim(); }
    };
    const missing = () => {
      for (const o of split().first) {
        if (!o.required || values[o.name] || o.default || (o.secret && r?.secrets?.includes(o.name))) continue;
        return [say(RT.missing, { field: optLabel(o.name) }), $(`[data-rf="${o.name}"]`, w.el)];
      }
      if (type().oauth && !signedIn) return [say(RT.signInFirst, { who: who() }), $('#a-oauth', w.el)];
      return null;
    };
    const showForm = () => {
      const t = type(), { first, rest } = split(), word = t.pathHint;
      const account = t.oauth ? frow(RT.account, signedIn ? `<span class="a-ctl">${pill(RT.signedInPill, 'ok')}${btn(RT.otherAccount, 'IconSignIn', 'id="a-oauth"')}</span>` : btn(say(RT.signInAt, { who: who() }), 'IconSignIn', 'id="a-oauth"'), RT.accountTip)
        + (signing ? `${frow(RT.onYourPc, A.copyField(t.authorize || `rclone authorize "${t.id}"`), RT.onYourPcTip)}${frow(RT.token, field('', `id="a-rtoken" placeholder="${esc(RT.tokenPh)}" spellcheck="false"`, 'w') + btn(RT.takeToken, 'IconCheck', 'id="a-rtake"'))}` : '') : '';
      const pathCtl = shares.length ? dropdown(shares.map(s => [s, s]), path || shares[0], 'data-rf="path"') : field(path, `data-rf="path" placeholder="${word === 'folder' ? '/' : ''}" spellcheck="false"`, 'm mono');
      const tested = { label: testing ? 'Prüfe' : !result ? 'Testen' : result.ok ? RT.connected : RT.notConnected, g: testing ? 'IconRefresh' : !result ? 'IconHealth' : result.ok ? 'IconCheck' : 'IconClose',
        id: 'a-rtest', tone: result ? (result.ok ? 'ok' : 'danger') : '', run: () => { test(); return false; } };
      w.set(`<div class="a-rform"><div class="well"><div class="eyebrow">${esc(who())}${provider && pHint(provider.id) ? ' ' + info(pHint(provider.id)) : ''}</div>${account}${first.map(optRow).join('')}
          ${frow(`${esc(RT.path[word])} <small>optional</small>`, pathCtl, RT.pathTip[word])}
          ${result?.hostKey ? frow(RT.hostKey, `<span class="a-hostkey mono" data-hostkey>${esc(result.hostKey.fingerprint)}</span>`, RT.hostKeyTip) : ''}</div>
        ${rest.length ? `<div class="well"><div class="eyebrow">${esc(RT.more)}</div>${frow(esc(say(RT.moreCount, { count: rest.length })), btn(more ? RT.moreHide : RT.moreShow, more ? 'IconCollapse' : 'IconExpand', 'id="a-rmore"'), RT.moreTip)}${more ? rest.map(optRow).join('') : ''}</div>` : ''}
        <div class="well">${frow(RT.name, field(name, `id="a-rname" placeholder="${esc(RT.namePh)}"`, 'm'), RT.nameTip)}${frow(RT.mountAt, pathField(mount, 'a-rmount'), RT.mountAtTip)}</div></div>`,
        [{ start: esc(verdict()) }, ...(isNew ? [{ label: 'Zurück', g: 'IconBack', run: () => { read(); showPick(); return false; } }] : []), cancel, tested,
          { label: isNew ? 'Hinzufügen' : 'Speichern', g: isNew ? 'IconAdd' : 'IconSave', primary: true, run: save }]);
      $('#a-rname', w.el).addEventListener('input', e => { nameTouched = true; if (!mountTouched) $('#a-rmount', w.el).value = '/remotes/' + e.target.value.trim().toLowerCase(); });
      $('#a-rmount', w.el).addEventListener('input', () => { mountTouched = true; });
      w.el.oninput = w.el.onchange = (e) => {
        if (e.target.id === 'a-rq' || !result) return;
        result = null;
        const b = $('#a-rtest', w.el);
        b.className = 'btn';
        b.innerHTML = g('IconHealth') + 'Testen';
        $('.win-foot .start', w.el).textContent = '';
      };
    };
    function choose(p, backend) {
      provider = p;
      typeId = backend;
      for (const k of Object.keys(values)) delete values[k];
      Object.assign(values, p?.preset || {});
      if (!nameTouched) { name = suggest(p?.name || backend, taken); if (!mountTouched) mount = '/remotes/' + name; }
      result = null; shares = []; more = false; signedIn = false; signing = false;
      showForm();
    }
    function test() {
      read();
      const m = missing();
      if (m) return fail(...m);
      testing = true;
      result = null;
      showForm();
      setTimeout(() => {
        testing = false;
        result = values.host === 'nirgends' ? { ok: false, error: 'dial tcp: no such host' } : { ok: true, free: 3 * TB, ...(typeId === 'sftp' ? { hostKey: { fingerprint: 'SHA256:47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU' } } : {}) };
        if (typeId === 'smb' && !path && result.ok) shares = ['archiv', 'fotos', 'backup'];
        showForm();
        if (!result.ok) A.shake($('#a-rtest', w.el));
      }, 700);
    }
    const save = (el) => {
      read();
      if (!/^[a-z0-9][a-z0-9_-]*$/.test(name)) return fail(name ? 'Nur Kleinbuchstaben, Ziffern, _ und -' : RT.needName, $('#a-rname', el));
      if (taken.includes(name)) return fail('Diesen Namen gibt es schon', $('#a-rname', el));
      if (!/^\/remotes\/.+/.test(mount)) return fail(say(RT.underRemotes, { name }), $('#a-rmount', el));
      const m = missing();
      if (m) return fail(...m);
      const remoteLabel = typeId === 'smb' ? `SMB: //${values.host}/${path}` : `rclone: ${name}`;
      const parent = FS.nodeAt('/remotes');
      if (r) {
        const node = FS.kids(parent).find(k => k.name === FS.split(r.mount).pop());
        if (node) Object.assign(node, { name: FS.split(mount).pop(), remote: remoteLabel });
        Object.assign(r, { name, mount, path, cfg: { ...values } });
      } else {
        DATA.remotes.push({ name, type: typeId, provider: provider?.id, mount, path, state: 'verbunden', cfg: { ...values } });
        if (!FS.kids(parent).some(k => k.name === FS.split(mount).pop())) parent.children.push({ name: FS.split(mount).pop(), dir: true, children: [], time: A.now(), owner: 'nobody', group: 'users', mode: '0775', remote: remoteLabel });
      }
      A.redraw();
      CC.toast(isNew ? say(RT.addedAt, { name, mount }) : 'Gespeichert');
    };
    w.el.addEventListener('click', e => {
      const t = e.target.closest('[data-type]');
      if (t && !typeId) {
        const p = REMOTES.providers.find(x => x.id === t.dataset.type);
        choose(p || null, p ? p.backend : t.dataset.type);
        return;
      }
      if (e.target.closest('#a-oauth')) { read(); signing = true; showForm(); return; }
      if (e.target.closest('#a-rtake')) {
        const tok = $('#a-rtoken', w.el).value.trim();
        if (!tok) return fail(RT.needToken, $('#a-rtoken', w.el));
        if (!tok.startsWith('{')) return fail(RT.tokenRejected, $('#a-rtoken', w.el));
        read(); signedIn = true; signing = false; showForm();
        CC.toast(say(RT.signedIn, { who: who() }));
        return;
      }
      if (e.target.closest('#a-rmore')) { read(); more = !more; showForm(); }
    });
    typeId ? showForm() : showPick();
  }

  CC.settingsTab({ id: 'remotes', label: 'Fernziele', g: 'IconCloud', order: 60, render(host) {
    const rc = DATA.rclone;
    const rows = DATA.remotes.map((r, i) => row({
      tile: markHtml(remoteMark(r)), name: r.name, attr: `data-open="${i}"`,
      badges: [pill(kindOf(r)), pill(r.mount, 'mono'), r.state !== 'verbunden' ? pill('Ausgehängt', 'state') : ''],
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
            const parent = FS.nodeAt('/remotes');
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

  CC.settingsTab({ id: 'users', label: 'Benutzer & Gruppen', g: 'IconAccounts', order: 70, render(host) {
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
  CC.settingsTab({ id: 'rules', label: 'Zugriffsregeln', g: 'IconShieldOn', order: 80, render(host) {
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

  CC.settingsTab({ id: 'shares', label: 'Freigaben', g: 'IconLink', order: 90, render(host) {
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
/* A picker window is at most 85 % of the screen tall, as GlimStone's are. */
.win:has(.a-picks) { max-height: 85vh; }
.a-picks { display: flex; flex-direction: column; gap: 1.5rem; }
.a-pgroup { display: flex; flex-direction: column; gap: .75rem; }
.a-pgroup > .eyebrow { margin: 0; padding: 0; font-weight: 400; }
.a-pglyph { color: var(--accent-glyph); }
.glim-readme-btn-unit:is(:hover, :focus-within) .a-pglyph { color: inherit; }
.a-pmore { margin: .25rem 0 0; font-size: 12px; font-weight: 400; color: var(--carbon-text-muted); }
.a-ppills { display: grid; grid-template-columns: repeat(auto-fill, minmax(9.5rem, 1fr)); gap: .5rem; margin: 0; padding: 0; list-style: none; }
.a-ppill { width: 100%; padding: .5rem .75rem; overflow: hidden; border-radius: var(--radius-pill); background: var(--carbon-surface2); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.a-ppill:hover, .a-ppill:focus-visible { background: var(--carbon-surface3); }
.a-pnone { padding: 1.5rem 0; text-align: center; color: var(--carbon-text-muted); }
.a-hostkey { overflow-wrap: anywhere; }
.a-rform { display: flex; flex-direction: column; gap: .8rem; }
`);
})();
