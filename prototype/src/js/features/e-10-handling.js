// Everyday handling: jump by typing, an undo history, folders ranked by use, the folder in the address,
// files from the clipboard, and duplicating or gathering the selection into a new folder.
(() => {
  const { frow, cancel, closeBtn } = CC.ui;
  const store = (key, fallback) => ({
    get() { try { return JSON.parse(localStorage.getItem(key)) || fallback; } catch { return fallback; } },
    set(v) { try { localStorage.setItem(key, JSON.stringify(v)); } catch {} },
  });
  const ago = (t) => { const m = Math.round((Date.now() - t) / 60000); return m < 1 ? 'gerade eben' : m < 60 ? `vor ${m} Min.` : `vor ${Math.round(m / 60)} Std.`; };

  // Undo: every step that can be taken back lands here, whether it came with a toast or not.
  const History = {
    list: [],
    push(label, undo, glyph = 'IconUndo') {
      const item = { label, undo, glyph, at: Date.now(), done: false };
      History.list.unshift(item);
      History.list.length = Math.min(History.list.length, 30);
      return item;
    },
    take(item) {
      if (!item || item.done) return;
      item.done = true;
      History.list = History.list.filter(x => x !== item);
      $$('#toasts .toast').forEach(t => { if (t.children[1]?.textContent === item.label) t.remove(); });
      item.undo();
      CC.render();
    },
  };
  CC.history = History;
  // A toast's own Rückgängig and Strg+Z share one step, so it can only be taken back once.
  const plainToast = CC.toast;
  CC.toast = (text, kind, act) => {
    if (act && act.label === 'Rückgängig') {
      const item = History.push(text, act.run);
      return plainToast(text, kind, { ...act, run: () => History.take(item) });
    }
    return plainToast(text, kind, act);
  };
  CC.action('undo', { label: 'Rückgängig', g: 'IconUndo', key: 'Strg+Z', run: () => {
    const item = History.list[0];
    if (!item) return CC.toast('Hier gibt es nichts zurückzunehmen', 'warn');
    History.take(item);
    plainToast(`Zurückgenommen: ${item.label}`);
  } });
  CC.action('undoList', { label: 'Letzte Aktionen', g: 'IconLog', run: () => {
    const r = $('[data-menu="1"]')?.getBoundingClientRect() || { left: 200, bottom: 80 };
    const items = History.list.slice(0, 10);
    menu(r.left, r.bottom + 6, items.length
      ? [{ head: 'Zurücknehmen' }, ...items.map(it => ({ label: it.label, g: it.glyph, count: ago(it.at), run: () => { History.take(it); plainToast(`Zurückgenommen: ${it.label}`); } }))]
      : [{ label: 'Noch nichts, das sich zurücknehmen ließe', g: 'IconInfo', run: () => {} }]);
  } });

  // Typing in the list jumps to the first name that starts with the letters typed so far, as in Krusader.
  const Jump = { text: '', timer: null };
  const jumpEnd = () => { Jump.text = ''; clearTimeout(Jump.timer); $$('.tjump').forEach(x => x.remove()); };
  const fold = (s) => s.toLocaleLowerCase('de');
  function jumpTo(from, dir) {
    const list = Files.entries(), t = Files.T(), n = list.length;
    for (let s = 0; s < n; s++) {
      const i = (from + s * dir + n * 2) % n;
      if (!list[i].up && fold(list[i].name).startsWith(fold(Jump.text))) { t.cursor = i; CC.render(); $('.panel.active .row.cursor')?.scrollIntoView({ block: 'nearest' }); return true; }
    }
    CC.render();
    return false;
  }
  const showJump = (found = true) => {
    $$('.tjump').forEach(x => x.remove());
    if (!Jump.text) return;
    $('.panel.active')?.insertAdjacentHTML('beforeend', `<div class="tjump glim-card ${found ? '' : 'miss'}" role="status">${g('IconSearch')}<span class="mono">${esc(Jump.text)}</span><small>${found ? 'Pfeile: nächster Treffer' : 'kein Name beginnt so'}</small></div>`);
  };
  const plainKey = CC.views.files.key;
  CC.views.files.key = (e, combo) => {
    const typed = e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey && (e.key !== ' ' || Jump.text);
    if (typed) {
      e.preventDefault();
      Jump.text += e.key;
      clearTimeout(Jump.timer);
      Jump.timer = setTimeout(jumpEnd, 4000);
      const found = jumpTo(Files.T().cursor, 1);
      return showJump(found);
    }
    if (Jump.text) {
      if (combo === 'Escape') { e.preventDefault(); return jumpEnd(); }
      if (combo === 'Backspace') { e.preventDefault(); Jump.text = Jump.text.slice(0, -1); if (!Jump.text) return jumpEnd(); jumpTo(0, 1); return showJump(); }
      if (combo === 'ArrowDown' || combo === 'ArrowUp') {
        e.preventDefault();
        clearTimeout(Jump.timer);
        Jump.timer = setTimeout(jumpEnd, 4000);
        const dir = combo === 'ArrowDown' ? 1 : -1;
        jumpTo(Files.T().cursor + dir, dir);
        return showJump();
      }
      jumpEnd();
    }
    plainKey(e, combo);
  };
  document.addEventListener('mousedown', () => Jump.text && jumpEnd());
  CC.action('typeJump', { label: 'Zur Datei springen', g: 'IconSearch', key: 'Buchstaben tippen', run: () => CC.toast('Tipp in der Liste einfach den Anfang eines Namens, der Cursor springt hin') });

  // Folders ranked like zoxide: how often, weighted by how recently.
  const visits = store('cannoncommander-frecent', {});
  const rank = (v) => { const h = (Date.now() - v.t) / 3600000; return v.n * (h < 1 ? 4 : h < 24 ? 2 : h < 168 ? .5 : .25); };
  CC.topFolders = (n = 8, skip = '') => Object.entries(visits.get()).filter(([p]) => p !== skip && FS.nodeAt(p)?.dir).sort((a, b) => rank(b[1]) - rank(a[1])).slice(0, n).map(([p]) => p);
  const plainGo = Files.go;
  Files.go = (path, i = Files.active, record) => {
    const out = plainGo(path, i, record);
    if (!FS.inArchive(path) && FS.nodeAt(path)?.dir) {
      const all = visits.get(), v = all[path] || { n: 0, t: 0 };
      all[path] = { n: v.n + 1, t: Date.now() };
      visits.set(all);
    }
    return out;
  };
  CC.paletteSources?.push({
    head: 'Häufige Ordner',
    list: () => CC.topFolders(12).map(p => ({ name: p, g: 'IconFolder', run: () => Files.go(p) })),
  });
  CC.action('history', { label: 'Ordnerverlauf', g: 'IconLog', key: 'Alt+Runter', run: () => {
    const r = $('.panel.active .pathbar')?.getBoundingClientRect() || { left: 300, bottom: 120 };
    const here = Files.T().path, top = CC.topFolders(5, here);
    const recent = Files.visited.filter(p => p !== here && !top.includes(p));
    const row = (p) => ({ label: p, g: 'IconFolder', run: () => Files.go(p) });
    menu(r.left, r.bottom + 6, top.length || recent.length
      ? [...(top.length ? [{ head: 'Häufig' }, ...top.map(row)] : []), ...(recent.length ? [{ head: 'Zuletzt besucht' }, ...recent.map(row)] : [])]
      : [{ head: 'Noch kein Verlauf' }]);
  } });

  // The open folder lives in the address after #, so a reload or a bookmark comes back to it, even from a file:// page.
  const pathOfHash = () => { try { return decodeURIComponent(location.hash.slice(1)); } catch { return ''; } };
  const hashOf = (p) => '#' + encodeURI(p);
  const syncHash = () => {
    if (CC.view !== 'files') return;
    const want = hashOf(Files.T().path);
    if (location.hash !== want) history.replaceState(history.state, '', want);
  };
  const plainRender = CC.views.files.render;
  CC.views.files.render = (host) => { plainRender(host); syncHash(); if (Jump.text) showJump(); };
  let fromHistory = false;
  // Stepping through our own entries also changes the hash; only a hash typed or opened from outside should move the panel.
  addEventListener('popstate', (e) => { if (e.state?.ccNav === undefined) return; fromHistory = true; setTimeout(() => { fromHistory = false; }, 0); });
  addEventListener('hashchange', () => {
    const p = pathOfHash();
    if (fromHistory || !p || p === Files.T().path) return;
    if (FS.nodeAt(p)?.dir) Files.go(p); else CC.toast('Diesen Ordner gibt es nicht', 'warn');
  });
  // The first render writes the start folder into the address, so the folder asked for is read before that.
  const asked = pathOfHash();
  document.addEventListener('DOMContentLoaded', () => { if (asked && asked !== Files.T().path && FS.nodeAt(asked)?.dir) Files.go(asked, 0, false); });
  CC.copyInternalLink = (path) => {
    const url = location.href.split('#')[0] + hashOf(path);
    navigator.clipboard?.writeText(url).catch(() => {});
    CC.toast('Interner Link kopiert. Wer ihn öffnet, sieht den Ordner mit seinen eigenen Rechten');
  };
  CC.action('internalLink', { label: 'Internen Link kopieren', g: 'IconLink', needs: 'sel', when: () => Files.sel().length === 1 && Files.sel()[0].dir, run: () => {
    const k = Files.sel()[0];
    CC.copyInternalLink(FS.join(k.at || Files.T().path, k.name));
  } });

  // The clipboard can hold a picture or text instead of copied files; pasting then makes a new file of it.
  delete CC.keymap['Ctrl+v'];
  const stamp = () => { const d = new Date(), p = (n) => String(n).padStart(2, '0'); return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}-${p(d.getMinutes())}`; };
  document.addEventListener('paste', e => {
    if (CC.view !== 'files' || winStack.length || e.target.closest?.('input, textarea, select, [contenteditable]')) return;
    if (Files.clip) { e.preventDefault(); return CC.run('clipPaste'); }
    const data = e.clipboardData;
    const pic = [...(data?.files || [])].find(f => f.type.startsWith('image/'));
    const text = data?.getData('text/plain');
    if (!pic && !text) return CC.toast('Die Ablage ist leer', 'warn');
    e.preventDefault();
    if (FS.inArchive(Files.T().path)) return CC.toast('Im Archiv geht nur Ansehen und Entpacken', 'warn');
    const folder = Files.T().path, dir = FS.nodeAt(folder);
    const ext = pic ? (pic.type.split('/')[1] || 'png').replace('jpeg', 'jpg') : 'txt';
    const want = pic ? `Screenshot ${stamp()}.${ext}` : 'Notiz.txt';
    const suggest = FS.kids(dir).some(c => c.name === want) ? Files.freeName(dir, want) : want;
    const url = pic && URL.createObjectURL(pic);
    const preview = pic ? `<img class="pz-img" src="${url}" alt="">` : `<pre class="pz-text">${esc(text.slice(0, 600))}</pre>`;
    win({ title: pic ? 'Bild als Datei einfügen' : 'Text als Datei einfügen',
      body: `<div class="well pz-prev">${preview}</div><div class="well">${frow('Name', `<input class="field w" id="pz-name" value="${esc(suggest)}" autocomplete="off">`, `Die Datei kommt nach ${esc(folder)}.`)}</div>`,
      foot: [cancel, { label: 'Anlegen', g: 'IconPaste', primary: true, run: (w) => {
        const name = $('#pz-name', w).value.trim();
        if (!name) return false;
        if (FS.kids(dir).some(c => c.name === name)) { CC.toast('Diesen Namen gibt es hier schon', 'warn'); return false; }
        const n = { name, size: pic ? pic.size : new Blob([text]).size, time: Date.now(), owner: 'nobody', group: 'users', mode: '0664', ...(pic ? {} : { text }) };
        dir.children.push(n);
        CC.render();
        const idx = Files.entries().findIndex(k => k.name === name);
        if (idx >= 0) { Files.T().cursor = idx; CC.render(); }
        CC.toast(`„${name}“ angelegt`, 'ok', { label: 'Rückgängig', run: () => { dir.children = dir.children.filter(c => c !== n); CC.render(); } });
      } }],
      onOpen: (w) => { const i = $('#pz-name', w), dot = suggest.lastIndexOf('.'); setTimeout(() => i.setSelectionRange(0, dot > 0 ? dot : suggest.length), 0); i.onkeydown = (ev) => { if (ev.key === 'Enter') $('.win-foot .btn.primary', w).click(); }; },
      onClose: () => url && URL.revokeObjectURL(url) });
  });

  // Duplicating keeps the copy next to its original under a free "Kopie" name.
  const copyName = (dir, name) => {
    const dot = name.lastIndexOf('.'), ext = dot > 0 ? name.slice(dot) : '', base = (dot > 0 ? name.slice(0, dot) : name).replace(/ Kopie( d+)?$/, '');
    for (let n = 1; ; n++) { const c = `${base} Kopie${n > 1 ? ' ' + n : ''}${ext}`; if (!FS.kids(dir).some(k => k.name === c)) return c; }
  };
  CC.action('duplicate', { label: 'Hier duplizieren', g: 'IconCopy', key: 'Strg+D', needs: ['sel', 'writable'], run: () => {
    const made = [];
    for (const k of Files.sel()) {
      const dir = FS.nodeAt(k.at || Files.T().path), orig = FS.kids(dir).find(c => c.name === k.name);
      const copy = { ...JSON.parse(JSON.stringify(orig)), name: copyName(dir, k.name), time: Date.now() };
      dir.children.push(copy);
      made.push([dir, copy]);
    }
    Files.T().marked = new Set();
    CC.render();
    const one = made.length === 1 && made[0][1].name;
    if (one) { const idx = Files.entries().findIndex(e => e.name === one); if (idx >= 0) { Files.T().cursor = idx; CC.render(); } }
    CC.toast(one ? `„${one}“ angelegt` : `${made.length} Kopien angelegt`, 'ok', { label: 'Rückgängig', run: () => { for (const [dir, c] of made) dir.children = dir.children.filter(x => x !== c); CC.render(); } });
  } });

  CC.action('folderWithSel', { label: 'Neuer Ordner mit Auswahl', g: 'IconFolderAdd', key: 'Umschalt+F7', needs: ['sel', 'writable'], run: () => {
    const folder = Files.T().path, dir = FS.nodeAt(folder);
    const picked = Files.sel().filter(k => !k.at || k.at === folder).map(k => FS.kids(dir).find(c => c.name === k.name)).filter(Boolean);
    if (!picked.length) return CC.toast('Das geht nur mit Dateien aus diesem Ordner', 'warn');
    const suggest = FS.kids(dir).some(c => c.name === 'Neuer Ordner') ? Files.freeName(dir, 'Neuer Ordner') : 'Neuer Ordner';
    win({ title: 'Neuer Ordner mit Auswahl',
      body: `<div class="well">${frow('Name', `<input class="field w" id="fws-name" value="${esc(suggest)}" autocomplete="off">`, `${picked.length === 1 ? '1 Objekt kommt' : `${picked.length} Objekte kommen`} gleich hinein.`)}</div>`,
      foot: [cancel, { label: 'Anlegen', g: 'IconFolderAdd', primary: true, run: (w) => {
        const name = $('#fws-name', w).value.trim();
        if (!name) return false;
        if (FS.kids(dir).some(c => c.name === name)) { CC.toast('Diesen Namen gibt es hier schon', 'warn'); return false; }
        // Moving hands over copies, as a transfer does, so the Unraid view of disks and shares follows the change.
        const clone = (n) => JSON.parse(JSON.stringify(n));
        const box = { name, dir: true, children: picked.map(clone), time: Date.now(), owner: 'nobody', group: 'users', mode: '0775' };
        dir.children = [...dir.children.filter(c => !picked.includes(c)), box];
        Files.T().marked = new Set();
        CC.render();
        const idx = Files.entries().findIndex(e => e.name === name);
        if (idx >= 0) { Files.T().cursor = idx; CC.render(); }
        CC.toast(`„${name}“ mit ${picked.length === 1 ? '1 Objekt' : `${picked.length} Objekten`} angelegt`, 'ok', { label: 'Rückgängig', run: () => { dir.children = [...dir.children.filter(c => c !== box), ...box.children.map(clone)]; CC.render(); } });
      } }],
      onOpen: (w) => { const i = $('#fws-name', w); setTimeout(() => i.select(), 0); i.onkeydown = (ev) => { if (ev.key === 'Enter') $('.win-foot .btn.primary', w).click(); }; } });
  } });

  CC.bindKey('Ctrl+z', 'undo');
  CC.bindKey('Ctrl+d', 'duplicate');
  CC.bindKey('Shift+F7', 'folderWithSel');

  const edit = MENUS.find(m => m[0] === 'Bearbeiten')[1];
  edit.unshift('undo', 'undoList', '-');
  edit.splice(edit.indexOf('rename') + 1, 0, 'duplicate');
  edit.splice(edit.indexOf('copyPath') + 1, 0, 'internalLink');
  const file = MENUS.find(m => m[0] === 'Datei')[1];
  file.splice(file.indexOf('newFolder') + 1, 0, 'folderWithSel');

  CC.css(`
.tjump { position: absolute; left: 50%; bottom: 3.2rem; transform: translateX(-50%); z-index: 4; display: flex; align-items: center; gap: .55rem; height: 2.1rem; padding: 0 .9rem; border-radius: var(--radius-pill); font-size: 13px; pointer-events: none; animation: tjump-in .12s ease-out; }
.tjump svg { width: .9rem; height: .9rem; color: var(--accent-glyph); }
.tjump .mono { font-weight: 600; color: var(--carbon-text); }
.tjump small { color: var(--carbon-text-muted); font-size: 12px; }
.tjump.miss svg, .tjump.miss .mono { color: var(--status-fail-text, #ff8389); }
@keyframes tjump-in { from { opacity: 0; translate: 0 .3rem; } }
@media (prefers-reduced-motion: reduce) { .tjump { animation: none; } }
.pz-prev { display: grid; place-items: center; max-height: 16rem; overflow: hidden; }
.pz-img { max-width: 100%; max-height: 14rem; border-radius: var(--radius-control); }
.pz-text { margin: 0; width: 100%; max-height: 14rem; overflow: auto; font-size: 12px; white-space: pre-wrap; color: var(--carbon-text-sub); }
`);
})();
