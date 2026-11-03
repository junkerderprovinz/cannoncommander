// Symbolische Verknüpfung and Eigenschaften und Rechte.
(() => {
  const { sw, frow, swVal, cancel } = CC.ui;
  const { dropdown, ddVal } = CC.admin;
  const bk = CC.bk;

  CC.action('symlink', { label: 'Symbolische Verknüpfung', g: 'IconLink', needs: 'sel', run: () => {
    const dir = bk.here(), n = bk.selNodes()[0];
    const target0 = FS.inArchive(dir) ? '' : FS.join(dir, n.name);
    const where0 = CC.prefs.dual && !FS.inArchive(bk.otherDir()) ? bk.otherDir() : FS.inArchive(dir) ? '/mnt/user' : dir;
    const w = win({
      title: 'Symbolische Verknüpfung', cls: 'bk',
      body: `<div class="well bk-form">
          ${frow('Zeigt auf', `<input class="field grow mono" id="l-target" value="${esc(target0)}" spellcheck="false" autocomplete="off">`, 'Der volle Pfad der Datei oder des Ordners. Eine Verknüpfung darf auch auf etwas zeigen, das es noch nicht gibt.')}
          ${frow('Liegt in', bk.pathField('l-where', where0))}
          ${frow('Name', `<input class="field grow" id="l-name" value="${esc(n.name)}" spellcheck="false" autocomplete="off">`)}
          ${frow('Ergibt', '<span class="bk-finalw" id="l-final"></span>')}
        </div>`,
      foot: [{ start: ' ' }, cancel, { label: 'Anlegen', g: 'IconLink', primary: true, run: () => make() }],
    });
    bk.wirePicks(w);
    const st = {};
    const draw = () => {
      st.target = $('#l-target', w).value.trim(); st.where = $('#l-where', w).value.trim(); st.name = $('#l-name', w).value.trim();
      const okDir = bk.isDir(st.where), taken = okDir && bk.nodeIn(st.where, st.name), self = FS.join(st.where, st.name) === st.target;
      st.tnode = FS.nodeAt(st.target);
      st.ok = okDir && st.name && !taken && !self && st.target.startsWith('/') && !/\//.test(st.name);
      $('#l-final', w).innerHTML = !okDir ? '<span class="pill fault">Diesen Ordner gibt es nicht</span>'
        : !st.name ? '<span class="pill fault">Der Name fehlt</span>'
        : taken ? '<span class="pill fault">Den Namen gibt es dort schon</span>'
        : self ? '<span class="pill fault">Eine Verknüpfung kann nicht auf sich selbst zeigen</span>'
        : `<span class="bk-path mono bk-final" title="${esc(FS.join(st.where, st.name))}"><bdi>${esc(FS.join(st.where, st.name))}</bdi></span>`;
      bk.setStart(w, st.target && !st.tnode ? 'Das Ziel gibt es noch nicht' : '');
      bk.primary(w).disabled = !st.ok;
    };
    function make() {
      if (!st.ok) return false;
      const t = st.tnode;
      FS.nodeAt(st.where).children.push(t?.dir ? bk.dirNode(st.name, [], { link: st.target, mode: '0777' }) : bk.fileNode(st.name, t ? t.size : st.target.length, { link: st.target, mode: '0777' }));
      bk.log('verknüpft', `${FS.join(st.where, st.name)} mit ${st.target}`);
      bk.reveal(st.where, st.name);
      bk.doneToast(`Verknüpfung angelegt: ${FS.join(st.where, st.name)}`, FS.join(st.where, st.name));
    }
    w.addEventListener('input', draw);
    w.addEventListener('keydown', e => { if (e.key === 'Enter' && e.target.closest('input')) { e.preventDefault(); bk.primary(w).click(); } });
    draw();
    setTimeout(() => { const i = $('#l-name', w); i.focus(); i.select(); }, 0);
  } });

  const UIDS = { root: 0, nobody: 99, jdp: 1000, anna: 1001, gast: 1002 };
  const GIDS = { root: 0, users: 100, admins: 1000, familie: 1001 };
  const KIND = { dir: 'Ordner', img: 'Bild', video: 'Video', audio: 'Musik', arch: 'Archiv', disk: 'Abbild', pdf: 'PDF-Dokument', office: 'Office-Dokument', text: 'Textdatei', code: 'Quelltext', file: 'Datei' };
  const typeOf = (n) => n.link ? `Verknüpfung auf ${n.dir ? 'einen Ordner' : 'eine Datei'}` : KIND[FS.kindOf(n)] + (n.dir || !FS.ext(n.name) ? '' : `, ${FS.ext(n.name).toUpperCase()}`);
  const counts = (n) => { let f = 0, d = 0; FS.walk(k => { k.dir ? d++ : f++; }, n); return [f, d]; };
  const symbolic = (mode, dir) => (dir ? 'd' : '-') + [...mode.slice(-3)].map(c => { const v = +c; return (v & 4 ? 'r' : '-') + (v & 2 ? 'w' : '-') + (v & 1 ? 'x' : '-'); }).join('');
  const bytes = (b) => `${Math.round(b).toLocaleString('de-AT')} Bytes`;
  const created = (n) => n.created ?? n.time - Math.floor(bk.rng(n.name)() * 400) * DAY;
  const WHO = [['Besitzer', 0], ['Gruppe', 1], ['Andere', 2]], BITS = [['Lesen', 4], ['Schreiben', 2], ['Ausführen', 1]];

  CC.action('props', { label: 'Eigenschaften und Rechte', g: 'IconSliders', key: 'Alt+Eingabe', run: () => {
    if (CC.view !== 'files') CC.go('files');
    const dir = bk.here(), sel = bk.selNodes();
    const self = !sel.length;
    const nodes = self ? [FS.nodeAt(dir)] : sel;
    const paths = self ? [dir] : nodes.map(n => FS.join(dir, n.name));
    const one = nodes.length === 1 ? nodes[0] : null;
    const ro = FS.inArchive(dir) || (dir === '/' && self);
    const size = nodes.reduce((a, n) => a + FS.totalSize(n), 0);
    const [fc, dc] = nodes.reduce(([f, d], n) => { if (!n.dir) return [f + 1, d]; const [a, b] = counts(n); return [f + a, d + b + (one ? 0 : 1)]; }, [0, 0]);
    const modes = [...new Set(nodes.map(n => (n.mode || (n.dir ? '0775' : '0664')).padStart(4, '0')))];
    const owners = [...new Set(nodes.map(n => n.owner || 'nobody'))], groups = [...new Set(nodes.map(n => n.group || 'users'))];
    const anyDir = nodes.some(n => n.dir && !n.link);
    // A selection with mixed owners keeps each one's own until a name is picked.
    const idPick = (known, cur, id) => {
      const names = [...new Set([...Object.keys(known), ...cur])].map(n => [n, n]), mixed = cur.length > 1;
      return dropdown(mixed ? [['', 'Wie bisher'], ...names] : names, mixed ? '' : cur[0], `id="${id}" ${ro ? 'disabled' : ''}`);
    };
    const general = one ? `
        ${frow('Art', `<span class="bk-val">${esc(typeOf(one))}</span>`)}
        ${frow('Ort', `<span class="bk-val bk-pathv"><span class="bk-path mono" title="${esc(paths[0])}"><bdi>${esc(paths[0])}</bdi></span>${bk.copyBtn(paths[0], 'Pfad kopieren')}</span>`)}
        ${one.link ? frow('Zeigt auf', `<span class="bk-val bk-pathv"><span class="bk-path mono"><bdi>${esc(one.link)}</bdi></span>${bk.copyBtn(one.link, 'Ziel kopieren')}</span>`) : ''}
        ${frow('Größe', `<span class="bk-val glim-num" data-tip="${bytes(size)}">${fmtSize(size) || '0 B'}${one.dir ? `, ${bk.plural(fc, 'Datei', 'Dateien')}${dc ? ` in ${bk.plural(dc, 'Ordner', 'Ordnern')}` : ''}` : ''}</span>`)}
        ${frow('Geändert', `<span class="bk-val glim-num">${fmtTime(one.time)}</span>`)}
        ${frow('Angelegt', `<span class="bk-val glim-num">${fmtTime(created(one))}</span>`)}`
      : `
        ${frow('Auswahl', `<span class="bk-val">${bk.plural(nodes.length, 'Objekt', 'Objekte')}</span>`)}
        ${frow('Ort', `<span class="bk-val bk-pathv"><span class="bk-path mono"><bdi>${esc(dir)}</bdi></span>${bk.copyBtn(paths.join('\n'), 'Pfade kopieren')}</span>`)}
        ${frow('Größe', `<span class="bk-val glim-num" data-tip="${bytes(size)}">${fmtSize(size) || '0 B'}, ${bk.plural(fc, 'Datei', 'Dateien')}${dc ? ` und ${bk.plural(dc, 'Ordner', 'Ordner')}` : ''}</span>`)}
        ${frow('Zuletzt geändert', `<span class="bk-val glim-num">${fmtTime(Math.max(...nodes.map(n => n.time)))}</span>`)}`;
    const title = one ? (self ? (FS.LABEL[dir] || FS.split(dir).pop() || 'Server').replace('::', '') : one.name) : bk.plural(nodes.length, 'Objekt', 'Objekte');
    const w = win({
      title: 'Eigenschaften und Rechte', wide: true, cls: 'bk bk-props',
      body: `<div class="bk-phead">${one ? bk.tile(one) : `<span class="bk-tile">${g('IconFiles')}</span>`}<span class="bk-name">${esc(title)}</span></div>
        <div class="bk-grid2 bk-top">
          <div class="well bk-form"><div class="eyebrow">Allgemein</div>${general}</div>
          <div class="well bk-form"><div class="eyebrow">Besitz</div>
            ${frow('Besitzer', `<span class="bk-idf">${idPick(UIDS, owners, 'r-owner')}<span class="pill glim-num" id="r-uid"></span></span>`, 'Unraid-Freigaben gehören nobody (99) und der Gruppe users (100). So kommen Docker-Container und Netzfreigaben an alles heran.')}
            ${frow('Gruppe', `<span class="bk-idf">${idPick(GIDS, groups, 'r-group')}<span class="pill glim-num" id="r-gid"></span></span>`)}
            ${frow('Auch für alles darin', sw(false, `id="r-rec" ${anyDir && !ro ? '' : 'disabled'}`), anyDir ? 'Setzt Besitz und Rechte auch für jede Datei und jeden Ordner darunter.' : 'Gilt nur für Ordner.')}
          </div>
        </div>
        <div class="well bk-form"><div class="bk-tools"><span class="eyebrow">Rechte</span>${modes.length > 1 ? '<span class="pill warn" data-tip="Die Objekte haben unterschiedliche Rechte. Speichern gibt allen die Rechte, die hier stehen.">verschieden</span>' : ''}</div>
          <div class="bk-perms">
            <span></span>${BITS.map(([l]) => `<span class="bk-ph">${l}</span>`).join('')}
            ${WHO.map(([l, wi]) => `<span class="bk-pl">${l}</span>${BITS.map(([bl, bit]) => `<span class="bk-pc"><button class="col-check" type="button" role="checkbox" aria-checked="false" data-bk-perm="${wi}:${bit}" aria-label="${l}: ${bl}" ${ro ? 'disabled' : ''}><span class="cbox">${g('IconCheck')}</span></button></span>`).join('')}`).join('')}
          </div>
          ${frow('Als Zahl', `<span class="bk-idf"><span class="pill mono" id="r-sym"></span><input class="field s mono glim-num" id="r-oct" maxlength="4" inputmode="numeric" ${ro ? 'disabled' : ''}></span>`, 'Die drei Ziffern stehen für Besitzer, Gruppe und Andere. Lesen zählt 4, Schreiben 2, Ausführen 1. 775 heißt: Besitzer und Gruppe dürfen alles, Andere lesen und öffnen.')}
        </div>`,
      foot: [{ start: ro ? 'Hier geht nur Ansehen' : ' ' }, cancel, { label: 'Speichern', g: 'IconSave', primary: true, run: () => save() }],
    });
    let mode = modes[0];
    const setMode = (m, fromField) => {
      mode = m.padStart(4, '0');
      $$('[data-bk-perm]', w).forEach(b => {
        const [wi, bit] = b.dataset.bkPerm.split(':').map(Number), on = !!(+mode[1 + wi] & bit);
        b.setAttribute('aria-checked', String(on));
        b.firstElementChild.classList.toggle('on', on);
      });
      if (!fromField) $('#r-oct', w).value = mode.slice(1);
      $('#r-sym', w).textContent = symbolic(mode, one?.dir);
    };
    const ids = () => {
      const o = ddVal($('#r-owner', w)), gr = ddVal($('#r-group', w));
      $('#r-uid', w).textContent = o ? `UID ${UIDS[o] ?? '?'}` : 'UID';
      $('#r-gid', w).textContent = gr ? `GID ${GIDS[gr] ?? '?'}` : 'GID';
    };
    w.addEventListener('click', e => {
      const b = e.target.closest('[data-bk-perm]');
      if (!b) return;
      const [wi, bit] = b.dataset.bkPerm.split(':').map(Number), digits = [...mode];
      digits[1 + wi] = String(+digits[1 + wi] ^ bit);
      setMode(digits.join(''));
    });
    w.addEventListener('change', ids);
    w.addEventListener('input', e => {
      if (e.target.id === 'r-oct') { const v = e.target.value.trim(); if (/^[0-7]{3,4}$/.test(v)) setMode(v.length === 3 ? '0' + v : v, true); }
    });
    function save() {
      if (ro) return false;
      const owner = ddVal($('#r-owner', w)), group = ddVal($('#r-group', w)), rec = swVal($('#r-rec', w));
      const v = $('#r-oct', w).value.trim();
      if (!/^[0-7]{3,4}$/.test(v)) { CC.toast('Die Rechte als Zahl sind drei Ziffern von 0 bis 7', 'warn'); return false; }
      let n = 0;
      const set = (k) => { if (owner) k.owner = owner; if (group) k.group = group; k.mode = mode; n++; };
      nodes.forEach(k => { set(k); if (rec && k.dir) FS.walk(set, k); });
      bk.log('Rechte geändert', `${bk.plural(n, 'Objekt', 'Objekte')} in ${dir}`);
      CC.render();
      CC.toast(`Gespeichert für ${bk.plural(n, 'Objekt', 'Objekte')}: ${owner || 'Besitzer bleibt'}:${group || 'Gruppe bleibt'}, ${mode.slice(1)}`);
    }
    w.addEventListener('keydown', e => { if (e.key === 'Enter' && e.target.closest('input')) { e.preventDefault(); bk.primary(w).click(); } });
    setMode(mode);
    ids();
    bk.primary(w).disabled = ro;
  } });

  CC.css(`
.bk-props .win-body { gap: .9rem; }
.bk-phead { display: flex; align-items: center; gap: .6rem; padding: 0 .1rem; }
.bk-phead .bk-name { font-size: 16px; font-weight: 600; }
.bk-val { margin-left: auto; text-align: right; color: var(--carbon-text-sub); min-width: 0; }
.bk-pathv { display: flex; align-items: center; gap: .2rem; max-width: 70%; }
.bk-pathv .bk-path { font-size: 12.5px; }
.bk-idf { display: flex; gap: .4rem; align-items: center; margin-left: auto; flex: 1; justify-content: flex-end; min-width: 0; }
.bk-idf .field { flex: 1; min-width: 0; max-width: 12rem; }
.bk-idf .field.s { flex: none; }
.bk-idf .pill { min-width: 4.4rem; justify-content: center; }
.bk-perms { display: grid; grid-template-columns: 8.5rem repeat(3, minmax(0, 7rem)); align-items: center; row-gap: .35rem; }
.bk-ph { font-size: 12px; color: var(--carbon-text-muted); text-align: center; }
.bk-pc { display: flex; justify-content: center; height: var(--btn-h); align-items: center; }
@media (max-width: 760px) { .bk-pathv { max-width: 75%; } .bk-perms { grid-template-columns: 5.5rem repeat(3, 1fr); } }
`);
})();
