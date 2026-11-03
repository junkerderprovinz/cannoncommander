// Suche nach Metadaten: words like breite:>=3840, codec:hevc or interpret:"Queen" match the values the list's columns show.
(() => {
  const norm = (s) => s.toLowerCase().replace(/ß/g, 'ss').normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/[^a-z0-9]/g, '');
  const UNITS = { b: 1, k: KB, kb: KB, m: MB, mb: MB, g: GB, gb: GB, t: TB, tb: TB };
  const AGE = { h: 3600e3, d: DAY, t: DAY, w: 7 * DAY, m: 30 * DAY, mo: 30 * DAY, y: 365 * DAY, j: 365 * DAY };
  const kindOf = (k) => k.kind ? k : { ...k, kind: FS.kindOf(k) };
  const side = (k, i) => { const m = COL.dimensions.value(kindOf(k)).match(/(\d+)\s*×\s*(\d+)/); return m ? +m[i] : NaN; };

  // A few fields read better than their column names; every column also answers to its label and its id.
  // Sizes compare files only, as in Explorer, since a folder's total would match almost every search.
  const SPECIAL = {
    size: { names: ['größe', 'groesse', 'size'], kind: 'bytes', num: (k) => k.dir ? NaN : k.size || 0 },
    time: { names: ['geändert', 'geaendert', 'datum', 'modified'], kind: 'time', num: (k) => k.time },
    width: { names: ['breite', 'width'], kind: 'num', num: (k) => side(k, 1) },
    height: { names: ['höhe', 'hoehe', 'height'], kind: 'num', num: (k) => side(k, 2) },
    duration: { names: ['dauer', 'duration', 'länge', 'laenge'], kind: 'secs', num: (k) => COL.duration.num(kindOf(k)) },
  };
  const ALIAS = { codec: 'vcodec', kamera: 'camera', interpret: 'artist', kuenstler: 'artist', kunstler: 'artist', jahr: 'year', seiten: 'pages', typ: 'type', art: 'kind', endung: 'ext', ordner: 'path', besitzer: 'owner', titel: 'title', objektiv: 'lens', blende: 'aperture', aufloesung: 'dimensions', bildrate: 'framerate', tonspuren: 'audiotracks' };
  const FIELDS = new Map();
  for (const c of COLUMNS) if (c.value) { FIELDS.set(norm(c.label), { col: c }); FIELDS.set(norm(c.id), { col: c }); }
  for (const [from, id] of Object.entries(ALIAS)) if (COL[id]) FIELDS.set(from, { col: COL[id] });
  FIELDS.set(norm('auflösung'), { col: COL.dimensions });
  for (const s of Object.values(SPECIAL)) for (const n of s.names) FIELDS.set(norm(n), { special: s });

  const unquote = (v) => v.replace(/^"(.*)"$/, '$1');
  const bytes = (v) => { const m = v.toLowerCase().replace(',', '.').match(/^([\d.]+)\s*([kmgt]?b?)$/); return m ? parseFloat(m[1]) * (UNITS[m[2]] || MB) : NaN; };
  const secs = (v) => { const m = v.toLowerCase().replace(',', '.').match(/^([\d.]+)\s*(h|std|min|m|s)?$/); return m ? parseFloat(m[1]) * ({ h: 3600, std: 3600, s: 1 }[m[2]] || 60) : NaN; };
  // A time is either an age such as 30d, 2w or 1y, or a date such as 2026-01-31 or 31.01.2026.
  const when = (v) => {
    const age = v.toLowerCase().match(/^(\d+)\s*(h|d|t|w|mo|m|y|j)$/);
    if (age) return { age: +age[1] * AGE[age[2]] };
    const iso = v.match(/^(\d{4})(?:-(\d{1,2})(?:-(\d{1,2}))?)?$/), de = v.match(/^(\d{1,2})\.(\d{1,2})\.(\d{4})$/);
    const [y, mo, d] = iso ? [+iso[1], +(iso[2] || 1), +(iso[3] || 1)] : de ? [+de[3], +de[2], +de[1]] : [];
    return y ? { at: new Date(y, mo - 1, d).getTime(), span: iso && !iso[2] ? 365 * DAY : iso && !iso[3] ? 30 * DAY : DAY } : null;
  };
  const cmp = (a, op, b) => ({ '>': a > b, '<': a < b, '>=': a >= b, '<=': a <= b, '=': a === b, '!=': a !== b }[op]);

  function test(field, op, raw) {
    const v = unquote(raw);
    if (field.special) {
      const s = field.special;
      if (s.kind === 'time') {
        const w = when(v);
        if (!w) return () => false;
        // An age turns the comparison round: geändert:<30d means changed less than 30 days ago.
        if (w.age != null) return (k) => cmp(NOW - k.time, op || '<=', w.age);
        if (!op || op === '=') return (k) => k.time >= w.at && k.time < w.at + w.span;
        return (k) => cmp(k.time, op, w.at);
      }
      const want = s.kind === 'bytes' ? bytes(v) : s.kind === 'secs' ? secs(v) : parseFloat(v.replace(',', '.'));
      if (isNaN(want)) return () => false;
      return (k) => { const n = s.num(k); return !isNaN(n) && n >= 0 && cmp(n, op || '=', want); };
    }
    const c = field.col, low = v.toLowerCase();
    const text = (k) => String(c.value(kindOf(k), k.at) || '');
    if (['<', '>', '<=', '>='].includes(op)) {
      const want = parseFloat(v.replace(',', '.'));
      return (k) => { const n = c.num ? c.num(kindOf(k)) : parseFloat(text(k).replace(',', '.')); return !isNaN(n) && n >= 0 && cmp(n, op, want); };
    }
    if (op === '=') return (k) => text(k).toLowerCase() === low;
    if (op === '!=') return (k) => !text(k).toLowerCase().includes(low);
    return (k) => text(k).toLowerCase().includes(low);
  }

  // Splits a query into field words and the rest, which the plain name search keeps handling.
  function parse(query = '') {
    const tests = [], unknown = [], rest = [];
    for (const m of query.matchAll(/([\p{L}][\p{L}\d.-]*):(>=|<=|!=|>|<|=)?("[^"]*"?|\S*)|"[^"]*"?|\S+/gu)) {
      if (!m[1]) { rest.push(m[0]); continue; }
      const field = FIELDS.get(norm(m[1]));
      if (!field) { unknown.push(m[1]); continue; }
      if (m[3] && m[3] !== '"') tests.push(test(field, m[2], m[3]));
    }
    return { fields: tests.length, unknown, rest: rest.join(' '), test: (k) => tests.every(t => t(k)) };
  }
  CC.meta = { parse, examples: ['breite:>=3840', 'codec:hevc', 'interpret:"Queen"', 'kamera:X100', 'größe:>1GB', 'geändert:<30d'] };

  const plain = nameMatcher;
  nameMatcher = (query) => {
    const q = parse(query);
    if (!q.fields && !q.unknown.length) return plain(query);
    if (q.unknown.length) return () => false;
    const name = q.rest.trim() ? plain(q.rest) : () => true;
    return (k) => q.test(k) && name(k);
  };

  // A field nobody knows would just find nothing, so the panel says which words it does understand.
  function decorate() {
    for (let i = 0; i < (CC.prefs.dual ? 2 : 1); i++) {
      const t = Files.T(i), panel = $(`.panel[data-panel="${i}"]`);
      if (!t.qf || !t.filter || !panel) continue;
      const { unknown } = parse(t.filter);
      if (!unknown.length) continue;
      $('.crumbs', panel)?.insertAdjacentHTML('afterend', `<div class="smart-bar meta-warn">${g('IconInfo')}<span>${unknown.map(u => `„${esc(u)}:“`).join(', ')} kenne ich nicht. Ich verstehe etwa breite, höhe, codec, interpret, kamera, größe, geändert, dauer und jeden Spaltennamen wie Objektiv oder Jahr</span></div>`);
    }
  }
  const render = CC.views.files.render;
  CC.views.files.render = (host) => { render(host); decorate(); };

  CC.css(`
.meta-warn { background: color-mix(in srgb, var(--status-warn-solid, #FF832B) 16%, var(--carbon-surface2)); }
.meta-warn > svg { color: var(--status-warn-solid, #FF832B); }
.meta-warn span { white-space: normal; }
`);
})();
