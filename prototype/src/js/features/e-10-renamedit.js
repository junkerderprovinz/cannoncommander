// Im Editor umbenennen: every name on a line of its own, edited as text, and each changed line renames its file.
(() => {
  const { sw, swVal, cancel } = CC.ui;
  const bk = CC.bk;

  CC.action('renameEditor', { label: 'Im Editor umbenennen', g: 'IconCode', needs: ['sel', 'writable'], run: () => {
    const dir = bk.here(), nodes = bk.selNodes(), before = nodes.map(n => n.name);
    let rows = [], lines = before.length;
    const w = win({
      title: 'Im Editor umbenennen', wide: true, cls: 'bk re-win',
      body: `<div class="well re-find">
          <input class="field grow mono" id="re-q" placeholder="Suchen" aria-label="Suchen" spellcheck="false" autocomplete="off">
          <input class="field grow mono" id="re-r" placeholder="Ersetzen durch" aria-label="Ersetzen durch" spellcheck="false" autocomplete="off">
          <span class="re-sw">Ausdruck ${sw(false, 'id="re-regex" data-tip="Suchen als regulären Ausdruck lesen, im Ersetzen stehen $1 und $2 für die Gruppen"')}</span>
          <span class="pill glim-num" id="re-n"></span>
          <button class="btn" id="re-all">${g('IconSync')}Alle ersetzen</button>
        </div>
        <div class="re-main">
          <div class="well re-edit"><div class="re-gutter mono" aria-hidden="true"></div><textarea class="re-ta mono" spellcheck="false" autocapitalize="off" wrap="off" aria-label="Namen, einer pro Zeile">${esc(before.join('\n'))}</textarea></div>
          <div class="well bk-fill"><div class="bk-tools"><span class="eyebrow" id="re-sum">Vorschau</span></div><div class="re-warn" id="re-warn" hidden></div><div class="bk-list" id="re-list"></div></div>
        </div>`,
      foot: [{ start: ' ' }, { label: 'Mit Mustern', g: 'IconPencil', run: () => { setTimeout(() => CC.run('multiRename')); } }, cancel, { label: 'Umbenennen', g: 'IconPencil', primary: true, run: () => apply() }],
    });
    const ta = $('.re-ta', w), gutter = $('.re-gutter', w), q = $('#re-q', w), r = $('#re-r', w);
    const pattern = () => {
      if (!q.value) return null;
      try { return swVal($('#re-regex', w)) ? new RegExp(q.value, 'g') : new RegExp(q.value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'gi'); } catch { return undefined; }
    };
    const draw = () => {
      // One empty line at the end is what an editor leaves behind and does not count as a name.
      const names = ta.value.replace(/\n$/, '').split('\n');
      lines = names.length;
      gutter.innerHTML = names.map((_, i) => `<span class="${i >= before.length ? 'extra' : ''}">${i + 1}</span>`).join('');
      gutter.scrollTop = ta.scrollTop;
      const moving = new Set(nodes), taken = new Set(bk.kidsOf(dir).filter(k => !moving.has(k)).map(k => k.name.toLowerCase()));
      const count = new Map();
      names.forEach(n => count.set(n.trim().toLowerCase(), (count.get(n.trim().toLowerCase()) || 0) + 1));
      rows = nodes.map((node, i) => {
        const out = (names[i] ?? '').trim();
        const err = !out ? 'leer' : /[/\0]/.test(out) ? 'unzulässig' : out !== node.name && taken.has(out.toLowerCase()) ? 'gibt es schon' : count.get(out.toLowerCase()) > 1 ? 'doppelt' : '';
        return { node, old: node.name, out, err };
      });
      const changed = rows.filter(x => x.out !== x.old).length, errs = rows.filter(x => x.err).length;
      const warn = $('#re-warn', w);
      warn.hidden = lines === before.length;
      warn.textContent = lines > before.length ? `Da stehen ${lines} Zeilen, gewählt sind aber ${before.length} Namen. Jede Zeile gehört zu genau einer Datei, bitte ${lines - before.length === 1 ? 'eine Zeile' : `${lines - before.length} Zeilen`} entfernen.`
        : `Da stehen nur ${lines} Zeilen für ${before.length} Namen. Jede Datei braucht ihre Zeile, auch wenn ihr Name gleich bleibt.`;
      $('#re-sum', w).textContent = `${bk.plural(before.length, 'Objekt', 'Objekte')}, ${changed} ${changed === 1 ? 'Name ändert' : 'Namen ändern'} sich`;
      bk.setStart(w, errs ? `${bk.plural(errs, 'Name passt', 'Namen passen')} nicht` : '');
      $('#re-list', w).innerHTML = rows.map(x => `<div class="bk-mrow ${x.out === x.old ? 'same' : ''}">
          <span class="bk-name mono">${esc(x.old)}</span><span class="bk-arrow sm">${g('IconForward')}</span>
          <span class="bk-name mono ${x.err ? '' : 'new'}">${esc(x.out) || '&nbsp;'}</span>${x.err ? `<span class="pill fault">${x.err}</span>` : '<span></span>'}</div>`).join('');
      const re = pattern();
      $('#re-n', w).textContent = re === undefined ? 'Ausdruck ungültig' : re ? bk.plural((ta.value.match(re) || []).length, 'Treffer', 'Treffer') : '';
      $('#re-all', w).disabled = !re;
      bk.primary(w).disabled = lines !== before.length || !!errs || !changed;
    };
    function apply() {
      if (lines !== before.length || rows.some(x => x.err)) return false;
      const todo = rows.filter(x => x.out !== x.old);
      if (!todo.length) return false;
      const t = Files.T();
      todo.forEach(x => { x.node.name = x.out; if (t.marked.delete(x.old)) t.marked.add(x.out); });
      bk.log('umbenannt', `${bk.plural(todo.length, 'Objekt', 'Objekte')} in ${dir}`);
      CC.render();
      CC.toast(`${bk.plural(todo.length, 'Name', 'Namen')} geändert`, 'ok', { label: 'Rückgängig', run: () => {
        todo.forEach(x => { x.node.name = x.old; if (t.marked.delete(x.out)) t.marked.add(x.old); });
        CC.render();
      } });
    }
    ta.addEventListener('input', draw);
    ta.addEventListener('scroll', () => { gutter.scrollTop = ta.scrollTop; });
    q.addEventListener('input', draw);
    w.addEventListener('change', e => { if (e.target.id === 're-regex') draw(); });
    $('#re-all', w).addEventListener('click', () => {
      const re = pattern();
      if (!re) return;
      ta.value = ta.value.replace(re, r.value);
      draw();
      ta.focus();
    });
    draw();
    setTimeout(() => ta.focus(), 0);
  } });

  const edit = MENUS.find(m => m[0] === 'Bearbeiten')[1];
  edit.splice(edit.indexOf('multiRename') + 1, 0, 'renameEditor');

  CC.css(`
.re-win { height: min(44rem, calc(100vh - 3rem)); }
.re-find { flex-direction: row; align-items: center; gap: .5rem; flex-wrap: wrap; flex: none; }
.re-find .field { flex: 1 1 12rem; min-width: 0; height: var(--btn-h); }
.re-sw { display: inline-flex; align-items: center; gap: .45rem; font-size: 13px; color: var(--carbon-text-sub); }
.re-main { flex: 1; min-height: 0; display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 1rem; }
.re-edit { flex-direction: row; gap: 0; padding: 0; overflow: hidden; min-height: 0; }
.re-gutter { flex: none; width: 3rem; padding: .7rem .5rem .7rem 0; overflow: hidden; text-align: right; color: var(--carbon-text-muted); font-size: 13px; line-height: 1.6; background: color-mix(in srgb, var(--carbon-text) 4%, transparent); user-select: none; }
.re-gutter span { display: block; }
.re-gutter span.extra { color: var(--status-fail-text, #ff8389); }
.re-ta { flex: 1; min-width: 0; resize: none; border: 0; outline: none; background: transparent; color: var(--carbon-text); padding: .7rem .8rem; font-size: 13px; line-height: 1.6; white-space: pre; overflow: auto; }
.re-warn { margin: 0 0 .5rem; padding: .45rem .7rem; border-radius: var(--radius-control); background: color-mix(in srgb, var(--status-fail-solid, #ff6b72) 16%, transparent); font-size: 13px; line-height: 1.45; }
@media (max-width: 900px) { .re-main { grid-template-columns: 1fr; grid-template-rows: minmax(12rem, 1fr) minmax(0, 1fr); } }
`);
})();
