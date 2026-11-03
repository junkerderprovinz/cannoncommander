// Prüfsummen erstellen und prüfen, Datei teilen und Zusammenfügen.
(() => {
  const { sw, seg, frow, segVal, swVal, cancel, closeBtn } = CC.ui;
  const bk = CC.bk;
  const ALGO = Object.fromEntries(bk.ALGOS.map(([id, label, len]) => [id, { id, label, len }]));
  const SUMFILE = /\.(md5|sha1|sha256|sha512|b3)$|^(SHA256SUMS|SHA1SUMS|MD5SUMS)$/i;
  const filesUnder = (dir, nodes) => nodes.flatMap(n => n.dir ? FS.kids(n).length ? filesUnder(FS.join(dir, n.name), FS.kids(n)).map(f => ({ ...f, rel: n.name + '/' + f.rel })) : [] : [{ node: n, dir, rel: n.name }]);
  // Reading speed of the made-up disks, so a big file visibly takes longer than a small one.
  const readMs = (size) => Math.min(1600, 120 + size / (900 * MB) * 1000);

  CC.action('checksum', { label: 'Prüfsummen erstellen', g: 'IconShieldOn', needs: 'sel', run: () => {
    const dir = bk.here(), nodes = bk.selNodes();
    const files = filesUnder(dir, nodes).filter(f => !SUMFILE.test(f.node.name));
    if (!files.length) return CC.toast('Hier gibt es keine Datei für eine Prüfsumme', 'warn');
    const base = nodes.length === 1 ? nodes[0].name : (FS.split(dir).pop() || 'pruefsummen').replace('::', '');
    let algo = 'sha256', timers = [], done = 0;
    const saveDir0 = FS.inArchive(dir) ? '/mnt/user/downloads' : dir;
    const w = win({
      title: 'Prüfsummen erstellen', wide: true, cls: 'bk bk-sums',
      body: `<div class="well bk-form">${frow('Verfahren', seg(bk.ALGOS.map(([id, l]) => [id, l]), algo, 'id="c-algo"'), 'SHA-256 steht neben fast jedem Download. MD5 und SHA-1 reichen, um Übertragungsfehler zu finden, aber nicht gegen Absicht. BLAKE3 ist am schnellsten.')}</div>
        <div class="well bk-fill"><div class="bk-tools"><span class="eyebrow" id="c-count"></span></div><div class="bk-list" id="c-list"></div></div>
        <div class="well bk-form"><div class="eyebrow">Speichern</div>
          ${frow('Ordner', bk.pathField('c-dir', saveDir0))}
          ${frow('Dateiname', `<input class="field grow mono" id="c-name" spellcheck="false" autocomplete="off">`)}
        </div>`,
      foot: [{ start: ' ' }, closeBtn, { label: 'Als .sha256 speichern', g: 'IconSave', primary: true, run: () => save() }],
      onClose: () => timers.forEach(clearTimeout),
    });
    bk.wirePicks(w);
    let nameTouched = false;
    const ext = () => algo;
    const compute = () => {
      timers.forEach(clearTimeout); timers = []; done = 0;
      $('#c-list', w).innerHTML = files.map((f, i) => `<div class="bk-sum" id="c-row-${i}">${bk.tile(f.node)}<span class="bk-name" title="${esc(f.rel)}">${esc(f.rel)}</span>
          <span class="bk-hash mono"><span class="bar"><i style="width:0%"></i></span></span><span></span></div>`).join('');
      let at = 0;
      files.forEach((f, i) => {
        const ms = readMs(f.node.size);
        const row = $(`#c-row-${i}`, w);
        timers.push(setTimeout(() => { row.querySelector('.bar i').style.transition = `width ${ms}ms linear`; row.querySelector('.bar i').style.width = '100%'; }, at + 20));
        at += ms;
        timers.push(setTimeout(() => {
          const h = bk.hashOf(algo, f.node);
          row.querySelector('.bk-hash').innerHTML = `<span title="${h}">${h}</span>`;
          row.lastElementChild.outerHTML = bk.copyBtn(h, 'Prüfsumme kopieren');
          done++; status();
        }, at));
      });
      status();
    };
    const status = () => {
      $('#c-count', w).textContent = `${bk.plural(files.length, 'Datei', 'Dateien')}, ${fmtSize(files.reduce((a, f) => a + f.node.size, 0))}`;
      bk.setStart(w, done < files.length ? `Lese ${done + 1} von ${files.length}` : `${ALGO[algo].label} für ${bk.plural(files.length, 'Datei', 'Dateien')} fertig`);
      const target = $('#c-dir', w).value.trim(), name = $('#c-name', w).value.trim();
      bk.setPrimary(w, `Als .${ext()} speichern`, 'IconSave', done < files.length || !bk.isDir(target) || !name);
    };
    const setName = () => { if (!nameTouched) $('#c-name', w).value = `${base}.${ext()}`; };
    function save() {
      const target = $('#c-dir', w).value.trim(), name = $('#c-name', w).value.trim();
      if (!bk.isDir(target) || !name) return false;
      // Paths in the list are relative to the folder the list is saved in, as sha256sum -c expects them.
      const prefix = target === dir ? '' : dir.startsWith(target + '/') ? dir.slice(target.length + 1) + '/' : dir + '/';
      const text = files.map(f => `${bk.hashOf(algo, f.node)}  ${prefix}${f.rel}`).join('\n') + '\n';
      const d = FS.nodeAt(target), old = bk.nodeIn(target, name);
      if (old) d.children = d.children.filter(k => k !== old);
      d.children.push(bk.fileNode(name, text.length, { text }));
      bk.reveal(target, name);
      bk.doneToast(`Gespeichert: ${FS.join(target, name)}`, FS.join(target, name));
    }
    w.addEventListener('change', e => { if (e.target.closest('#c-algo')) { algo = segVal($('#c-algo', w)); setName(); compute(); } });
    w.addEventListener('input', e => { if (e.target.id === 'c-name') nameTouched = true; status(); });
    setName();
    compute();
  } });

  // Both "hash  name" (sha256sum) and "SHA256 (name) = hash" (BSD) lines are read.
  function parse(text) {
    return text.split('\n').map(l => l.trim()).filter(l => l && !l.startsWith('#')).map(l => {
      let m = l.match(/^([0-9a-f]{32,128})\s+\*?(.+)$/i);
      if (m) return { hash: m[1].toLowerCase(), name: m[2] };
      m = l.match(/^(MD5|SHA1|SHA256|SHA512|BLAKE3)\s*\((.+)\)\s*=\s*([0-9a-f]+)$/i);
      if (m) return { hash: m[3].toLowerCase(), name: m[2], algo: { md5: 'md5', sha1: 'sha1', sha256: 'sha256', sha512: 'sha512', blake3: 'b3' }[m[1].toLowerCase()] };
      return { bad: l };
    });
  }
  const algoFor = (fileName, hash) => {
    const e = FS.ext(fileName || '');
    if (ALGO[e]) return e;
    return { 32: 'md5', 40: 'sha1', 64: 'sha256', 128: 'sha512' }[hash.length] || 'sha256';
  };

  CC.action('checksumVerify', { label: 'Prüfsummen prüfen', g: 'IconShieldOn', run: () => {
    const cur = CC.view === 'files' ? Files.sel().find(k => !k.dir && SUMFILE.test(k.name)) : null;
    const dir0 = CC.view === 'files' ? bk.here() : '/mnt/user/downloads';
    let mode = cur ? 'file' : 'paste', timers = [];
    const w = win({
      title: 'Prüfsummen prüfen', wide: true, cls: 'bk bk-verify',
      body: `<div class="well bk-form">
          ${frow('Prüfsummen', seg([['file', 'Aus einer Datei'], ['paste', 'Eingefügt']], mode, 'id="v-mode"'))}
          <div class="bk-swap">
            <div class="bk-swap-a" ${mode === 'file' ? '' : 'hidden'}>${frow('Datei', `<input class="field grow mono" id="v-file" value="${esc(cur ? FS.join(dir0, cur.name) : '')}" placeholder="/mnt/user/downloads/datei.sha256" spellcheck="false" autocomplete="off">`, 'Eine Liste wie .sha256, .md5 oder SHA256SUMS. Die Dateien werden in ihrem Ordner gesucht.')}</div>
            <div class="bk-swap-b" ${mode === 'paste' ? '' : 'hidden'}><textarea class="field mono" id="v-text" rows="3" placeholder="Prüfsumme und Dateiname, eine Zeile je Datei" spellcheck="false"></textarea>
              ${frow('Dateien liegen in', bk.pathField('v-dir', dir0))}</div>
          </div>
        </div>
        <div class="well bk-fill"><div class="bk-tools"><span class="eyebrow" id="v-count">Ergebnis</span></div><div class="bk-list" id="v-list"><div class="empty">${g('IconShieldOn')}<div>Füge Prüfsummen ein oder wähle eine Datei</div></div></div></div>`,
      foot: [{ start: ' ' }, closeBtn, { label: 'Prüfen', g: 'IconShieldOn', primary: true, run: () => { check(); return false; } }],
      onClose: () => timers.forEach(clearTimeout),
    });
    bk.wirePicks(w);
    function check() {
      timers.forEach(clearTimeout); timers = [];
      let lines, folder, sumName;
      if (mode === 'file') {
        const p = $('#v-file', w).value.trim(), n = FS.nodeAt(p);
        if (!n || n.dir) { $('#v-list', w).innerHTML = `<div class="empty">${g('IconFile')}<div>Diese Datei gibt es nicht</div></div>`; return; }
        folder = FS.parent(p); sumName = n.name;
        lines = parse(bk.textOf(folder, n));
      } else {
        folder = $('#v-dir', w).value.trim();
        lines = parse($('#v-text', w).value);
      }
      const rows = lines.map(l => {
        if (l.bad) return { name: l.bad, state: 'bad' };
        const path = l.name.startsWith('/') ? l.name : FS.join(folder, l.name), n = FS.nodeAt(path);
        const algo = l.algo || algoFor(sumName, l.hash);
        return { name: l.name, node: n, algo, state: !n || n.dir ? 'missing' : bk.hashOf(algo, n) === l.hash ? 'ok' : 'mismatch' };
      });
      if (!rows.length) { $('#v-list', w).innerHTML = `<div class="empty">${g('IconShieldOn')}<div>Keine Prüfsummen gefunden</div></div>`; $('#v-count', w).textContent = 'Ergebnis'; return; }
      const PILL = { ok: '<span class="bk-ok"><span class="bk-dot"></span>OK</span>', mismatch: '<span class="pill fault">Weicht ab</span>', missing: '<span class="pill warn">Fehlt</span>', bad: '<span class="pill warn">Nicht lesbar</span>' };
      $('#v-list', w).innerHTML = rows.map((r, i) => `<div class="bk-sum" id="v-row-${i}">${r.node && !r.node.dir ? bk.tile(r.node) : `<span class="bk-tile">${g('IconFile')}</span>`}<span class="bk-name mono">${esc(r.name)}</span>
          <span class="bk-hash">${r.node && !r.node.dir ? '<span class="bar"><i style="width:0%"></i></span>' : ''}</span><span class="bk-res"></span></div>`).join('');
      let at = 0, seen = 0;
      const tally = () => {
        const c = (s) => rows.slice(0, seen).filter(r => r.state === s).length;
        $('#v-count', w).textContent = `${bk.plural(rows.length, 'Datei', 'Dateien')}${seen ? `, ${c('ok')} OK` : ''}${c('mismatch') ? `, ${c('mismatch')} weichen ab` : ''}${c('missing') ? `, ${c('missing')} fehlen` : ''}`;
        bk.setStart(w, seen < rows.length ? `Prüfe ${seen + 1} von ${rows.length}` : c('mismatch') ? 'Eine Datei, die abweicht, ist beschädigt oder eine andere Version' : '');
      };
      rows.forEach((r, i) => {
        const row = $(`#v-row-${i}`, w), ms = r.node && !r.node.dir ? readMs(r.node.size) : 60;
        timers.push(setTimeout(() => { const b = row.querySelector('.bar i'); if (b) { b.style.transition = `width ${ms}ms linear`; b.style.width = '100%'; } }, at + 20));
        at += ms;
        timers.push(setTimeout(() => { row.querySelector('.bk-hash').innerHTML = r.algo ? `<span class="muted">${ALGO[r.algo].label}</span>` : ''; row.querySelector('.bk-res').innerHTML = PILL[r.state]; seen++; tally(); }, at));
      });
      tally();
    }
    w.addEventListener('change', e => {
      if (!e.target.closest('#v-mode')) return;
      mode = segVal($('#v-mode', w));
      $('.bk-swap-a', w).hidden = mode !== 'file';
      $('.bk-swap-b', w).hidden = mode !== 'paste';
      $(mode === 'file' ? '#v-file' : '#v-text', w).focus();
    });
    w.addEventListener('keydown', e => { if (e.key === 'Enter' && e.target.id === 'v-file') { e.preventDefault(); check(); } });
    if (cur) check();
  } });

  const PARTS = [['100', '100 MB'], ['1024', '1 GB'], ['4096', '4 GB'], ['custom', 'Eigene']];
  CC.action('split', { label: 'Datei teilen', g: 'IconCompressMax', needs: 'file', run: () => {
    const dir = bk.here(), n = bk.selNodes().find(k => !k.dir);
    const w = win({
      title: 'Datei teilen', cls: 'bk',
      body: `<div class="well"><div class="bk-filehead">${bk.tile(n)}<span class="bk-name">${esc(n.name)}</span><span class="pill glim-num">${fmtSize(n.size)}</span></div></div>
        <div class="well bk-form"><div class="eyebrow">Teile</div>
          ${frow('Größe je Teil', seg(PARTS, n.size > 4 * GB ? '4096' : n.size > GB ? '1024' : '100', 'id="t-size"'), '4 GB passt auf einen USB-Stick mit FAT32. 100 MB gehen durch die meisten Mail- und Upload-Grenzen.')}
          ${frow('Eigene Größe', `<span class="bk-range"><input class="field s glim-num" id="t-custom" value="700" inputmode="numeric" disabled><span class="muted">MB</span></span>`)}
          ${frow('Prüfsummendatei dazulegen', sw(true, 'id="t-sum"'), 'Legt eine .sha256 der ganzen Datei neben die Teile. Zusammenfügen prüft damit das Ergebnis.')}
        </div>
        <div class="well bk-form"><div class="eyebrow">Ziel</div>
          ${frow('Ordner', bk.pathField('t-dest', FS.inArchive(dir) ? '/mnt/user/downloads' : dir))}
          ${frow('Ergibt', `<span class="bk-finalw" id="t-final"></span>`)}
        </div>`,
      foot: [{ start: 'Läuft als Aufgabe im Hintergrund' }, cancel, { label: 'Teilen', g: 'IconCompressMax', primary: true, run: () => go() }],
    });
    bk.wirePicks(w);
    const st = {};
    const draw = () => {
      const v = segVal($('#t-size', w)), custom = $('#t-custom', w);
      custom.disabled = v !== 'custom';
      st.part = (v === 'custom' ? parseFloat(custom.value.replace(',', '.')) || 0 : +v) * MB;
      st.dest = $('#t-dest', w).value.trim();
      st.count = st.part > 0 ? Math.ceil(n.size / st.part) : 0;
      const ok = bk.isDir(st.dest), pad = Math.max(3, String(st.count).length);
      st.names = Array.from({ length: st.count }, (_, i) => `${n.name}.${String(i + 1).padStart(pad, '0')}`);
      $('#t-final', w).innerHTML = !ok ? '<span class="pill fault">Diesen Ordner gibt es nicht</span>'
        : st.count < 2 ? '<span class="pill warn">Die Datei ist kleiner als ein Teil</span>'
        : `<span class="pill glim-num">${st.count} Teile</span><span class="bk-path mono bk-final" title="${esc(FS.join(st.dest, st.names[0]))}"><bdi>${esc(st.names[0])} bis .${st.names.at(-1).split('.').pop()}</bdi></span>`;
      bk.primary(w).disabled = !ok || st.count < 2;
    };
    function go() {
      if (st.count < 2 || !bk.isDir(st.dest)) return false;
      const { dest, names, part } = st, withSum = swVal($('#t-sum', w));
      Jobs.start({ kind: 'split', label: `Teilen: ${n.name}`, total: n.size, done: () => {
        const d = FS.nodeAt(dest);
        names.forEach((nm, i) => {
          d.children = d.children.filter(k => k.name !== nm);
          d.children.push(bk.fileNode(nm, i < names.length - 1 ? part : n.size - part * (names.length - 1)));
        });
        if (withSum) {
          const text = `${bk.hashOf('sha256', n)}  ${n.name}\n`;
          d.children = d.children.filter(k => k.name !== n.name + '.sha256');
          d.children.push(bk.fileNode(n.name + '.sha256', text.length, { text }));
        }
        bk.reveal(dest, names[0]);
        bk.doneToast(`Geteilt in ${names.length} Teile in ${dest}`, FS.join(dest, names[0]));
      } });
      CC.toast(`Teilen: ${n.name} läuft`, 'ok', { label: 'Aufgaben', run: () => CC.run('jobs') });
    }
    w.addEventListener('input', draw);
    w.addEventListener('change', e => { draw(); if (e.target.closest('#t-size') && segVal($('#t-size', w)) === 'custom') $('#t-custom', w).focus(); });
    draw();
  } });

  CC.action('combine', { label: 'Zusammenfügen', g: 'IconMerge', needs: 'file', run: () => {
    const dir = bk.here(), pick = bk.selNodes().find(k => /\.\d{3,}$/.test(k.name));
    if (!pick) return CC.toast('Erst einen Teil wie datei.001 wählen', 'warn');
    const base = pick.name.replace(/\.\d{3,}$/, '');
    const parts = bk.kidsOf(dir).filter(k => !k.dir && k.name.startsWith(base + '.') && /^\d{3,}$/.test(k.name.slice(base.length + 1))).sort((a, b) => a.name.localeCompare(b.name, 'de', { numeric: true }));
    const nums = parts.map(p => +p.name.slice(base.length + 1));
    const missing = nums.length ? Array.from({ length: Math.max(...nums) }, (_, i) => i + 1).filter(i => !nums.includes(i)) : [];
    const total = parts.reduce((a, p) => a + p.size, 0);
    const sum = bk.nodeIn(dir, base + '.sha256');
    const w = win({
      title: 'Zusammenfügen', cls: 'bk',
      body: `<div class="well"><div class="eyebrow">${bk.plural(parts.length, 'Teil', 'Teile')}, ${fmtSize(total)}</div>
          <table class="tbl">${parts.map(p => `<tr><td class="mono">${esc(p.name)}</td><td class="glim-num r">${fmtSize(p.size)}</td></tr>`).join('')}
          ${missing.length ? `<tr><td colspan="2"><span class="pill fault">Es fehlt ${missing.map(i => '.' + String(i).padStart(3, '0')).join(', ')}</span></td></tr>` : ''}</table></div>
        <div class="well bk-form"><div class="eyebrow">Ergebnis</div>
          ${frow('Name', `<input class="field grow mono" id="j-name" value="${esc(base)}" spellcheck="false" autocomplete="off">`)}
          ${frow('Ordner', bk.pathField('j-dest', FS.inArchive(dir) ? '/mnt/user/downloads' : dir))}
          ${frow('Danach prüfen', sw(!!sum, `id="j-check" ${sum ? '' : 'disabled'}`), sum ? `Vergleicht das Ergebnis mit ${sum.name}.` : 'Dafür fehlt eine .sha256 mit dem Namen der ganzen Datei neben den Teilen.')}
          ${frow('Teile danach löschen', sw(false, 'id="j-drop"'), 'Die Teile landen im Papierkorb, sobald das Zusammenfügen geklappt hat.')}
          ${frow('Wird gespeichert als', `<span class="bk-finalw" id="j-final"></span>`)}
        </div>`,
      foot: [{ start: 'Läuft als Aufgabe im Hintergrund' }, cancel, { label: 'Zusammenfügen', g: 'IconMerge', primary: true, run: () => go() }],
    });
    bk.wirePicks(w);
    const st = {};
    const draw = () => {
      st.dest = $('#j-dest', w).value.trim(); st.name = $('#j-name', w).value.trim();
      const ok = bk.isDir(st.dest);
      st.file = ok && st.name ? (bk.nodeIn(st.dest, st.name) ? Files.freeName(FS.nodeAt(st.dest), st.name) : st.name) : '';
      $('#j-final', w).innerHTML = !ok ? '<span class="pill fault">Diesen Ordner gibt es nicht</span>' : !st.name ? '<span class="pill fault">Der Name fehlt</span>'
        : `<span class="bk-path mono bk-final" title="${esc(FS.join(st.dest, st.file))}"><bdi>${esc(FS.join(st.dest, st.file))}</bdi></span>` + (st.file !== st.name ? '<span class="pill warn" data-tip="Eine Datei mit diesem Namen liegt schon dort. Sie bleibt, die neue bekommt eine Nummer.">Name vergeben</span>' : '');
      bk.primary(w).disabled = !st.file || !!missing.length || parts.length < 2;
    };
    function go() {
      if (!st.file || missing.length) return false;
      const { dest, file } = st, check = swVal($('#j-check', w)), drop = swVal($('#j-drop', w));
      const expected = sum ? bk.textOf(dir, sum).trim().split(/\s+/)[0] : null;
      Jobs.start({ kind: 'combine', label: `Zusammenfügen: ${file}`, total, done: () => {
        const node = bk.fileNode(file, total);
        FS.nodeAt(dest).children.push(node);
        const good = !check || bk.hashOf('sha256', { name: base, size: total }) === expected;
        if (drop && good) {
          const d = FS.nodeAt(dir);
          d.children = d.children.filter(k => !parts.includes(k));
          parts.forEach(p => DATA.trash.unshift({ name: p.name, from: dir, size: p.size, deleted: Date.now(), by: CC.user.name, node: p }));
        }
        bk.log('zusammengefügt', `${parts.length} Teile zu ${FS.join(dest, file)}`);
        bk.reveal(dest, file);
        if (!good) CC.toast(`Die Prüfsumme von ${file} stimmt nicht. Die Teile bleiben liegen.`, 'fault');
        else bk.doneToast(`${check ? 'Zusammengefügt und geprüft' : 'Zusammengefügt'}: ${FS.join(dest, file)}`, FS.join(dest, file));
      } });
      CC.toast(`Zusammenfügen: ${file} läuft`, 'ok', { label: 'Aufgaben', run: () => CC.run('jobs') });
    }
    w.addEventListener('input', draw);
    w.addEventListener('change', draw);
    draw();
  } });

  CC.css(`
.bk-sums { height: min(44rem, calc(100vh - 3rem)); }
.bk-verify { height: min(42rem, calc(100vh - 3rem)); }
.bk-sum { display: grid; grid-template-columns: 1.75rem minmax(0, 1.3fr) minmax(0, 1fr) 5.5rem; gap: .7rem; align-items: center; height: 2.25rem; padding: 0 .45rem; border-radius: var(--radius-control); flex: none; }
.bk-sum:hover { background: var(--carbon-surface3); }
.bk-hash { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 12px; color: var(--carbon-text-sub); }
.bk-hash .bar { display: block; width: 100%; height: .35rem; max-width: 14rem; contain: size layout; }
.bk-sum > :last-child { justify-self: end; }
.bk-ok { display: inline-flex; align-items: center; gap: .4rem; font-size: 12px; color: var(--carbon-text-sub); }
.bk-swap { min-height: 7.6rem; display: flex; flex-direction: column; }
.bk-swap > div { display: flex; flex-direction: column; gap: .6rem; }
.bk-swap > [hidden] { display: none; }
.bk-swap textarea { width: 100%; min-height: 4.6rem; }
.bk-filehead { display: grid; grid-template-columns: 1.75rem minmax(0, 1fr) auto; gap: .6rem; align-items: center; }
.bk-filehead .bk-name { font-weight: 600; }
@media (max-width: 760px) { .bk-sum { grid-template-columns: 1.75rem minmax(0, 1fr) auto; } .bk-sum .bk-hash { display: none; } }
`);
})();
