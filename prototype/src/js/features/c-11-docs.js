// Renderers for text, code, markdown, documents, archives, disk images and everything else (a hex view).
(() => {
  const { rng, plural, viewKind, textFor, md, highlight, R } = CC.ck;
  const LANG = { yml: 'YAML', yaml: 'YAML', go: 'Go', json: 'JSON', xml: 'XML', md: 'Markdown', sh: 'Shell', js: 'JavaScript', ts: 'TypeScript', conf: 'Nginx', cfg: 'Konfiguration', log: 'Log', 1: 'Log', srt: 'Untertitel', sha256: 'Prüfsumme', txt: 'Text' };
  const langOf = CC.ck.langOf = (n) => LANG[FS.ext(n.name)] || (n.name === 'go' ? 'Shell' : 'Text');

  let wrap = false;
  R.text = R.code = (host, n, ctx) => {
    const all = textFor(n).replace(/\n$/, '').split('\n');
    const shown = ctx.compact ? all.slice(0, 160) : all;
    const kind = viewKind(n), ext = FS.ext(n.name);
    const info = `${langOf(n)}, ${plural(all.length, 'Zeile', 'Zeilen')}`;
    host.innerHTML = `${ctx.compact ? '' : `<div class="cv-bar"><span class="grow"></span><span class="cv-swl">Zeilen umbrechen ${sw(wrap, 'id="cv-wrap"')}</span></div>`}
      <div class="cv-code ${wrap || ctx.compact ? 'wrap' : ''}"><div class="cv-lines">${shown.map(l => `<div class="ln">${highlight(l, kind, ext) || ' '}</div>`).join('')}</div></div>`;
    $('#cv-wrap', host)?.addEventListener('change', (e) => { wrap = swVal(e.target); $('.cv-code', host).classList.toggle('wrap', wrap); });
    return { info };
  };

  let mdView = 'md';
  R.md = (host, n, ctx) => {
    const text = textFor(n);
    const info = `Markdown, ${plural(text.replace(/\n$/, '').split('\n').length, 'Zeile', 'Zeilen')}`;
    if (ctx.compact) { host.innerHTML = `<article class="cv-md compact">${md(text)}</article>`; return { info }; }
    const paint = () => {
      host.innerHTML = `<div class="cv-bar"><span class="grow"></span>${seg([['md', 'Gerendert'], ['src', 'Quelltext']], mdView, 'id="cv-mdv"')}</div>
        <div class="cv-scroll">${mdView === 'md' ? `<article class="cv-md">${md(text)}</article>` : `<div class="cv-code wrap"><div class="cv-lines">${text.replace(/\n$/, '').split('\n').map(l => `<div class="ln">${esc(l) || ' '}</div>`).join('')}</div></div>`}</div>`;
      wireControls(host);
      $('#cv-mdv', host).addEventListener('change', (e) => { mdView = segVal(e.currentTarget); paint(); });
    };
    paint();
    return { info };
  };

  // Paper pages: their type is sized in container units, so a thumbnail is the same page, only smaller.
  const POOL = [
    'Die Parteien vereinbaren die nachstehenden Bedingungen und bestätigen mit ihrer Unterschrift, eine Ausfertigung dieses Vertrags erhalten zu haben.',
    'Änderungen und Ergänzungen bedürfen der Schriftform. Mündliche Nebenabreden wurden nicht getroffen.',
    'Sollte eine Bestimmung unwirksam sein, bleibt die Gültigkeit der übrigen Bestimmungen davon unberührt. An ihre Stelle tritt eine Regelung, die dem wirtschaftlichen Zweck am nächsten kommt.',
    'Die Abrechnung erfolgt jährlich bis spätestens 30. Juni des Folgejahres. Guthaben werden mit der nächsten Zahlung verrechnet, Nachforderungen sind binnen 14 Tagen fällig.',
    'Der Zugang zu den Gemeinschaftsflächen steht allen Bewohnerinnen und Bewohnern offen. Fahrräder sind im dafür vorgesehenen Raum im Erdgeschoß abzustellen.',
    'Schäden sind unverzüglich zu melden. Kleine Reparaturen bis zu einem Betrag von 100 Euro im Einzelfall übernimmt die Mieterin oder der Mieter selbst.',
  ];
  const para = (r, n = 2) => Array.from({ length: n }, () => `<p>${POOL[Math.floor(r() * POOL.length)]} ${POOL[Math.floor(r() * POOL.length)]}</p>`).join('');
  const foot = (k, total) => `<div class="pf">Seite ${k + 1} von ${total}</div>`;

  function docPages(n) {
    const r = rng(n.name);
    if (n.name === 'Rechnung-Oktober.pdf') return [`
      <div class="lh"><b>Kabelwerk Netz GmbH</b><span>Industriestraße 12, 4020 Linz</span></div>
      <div class="addr">Junker der Provinz<br>Hauptplatz 1<br>4600 Wels</div>
      <dl class="meta"><dt>Rechnungsnummer</dt><dd>2026-10-4471</dd><dt>Datum</dt><dd>31.10.2026</dd><dt>Kundennummer</dt><dd>K-208815</dd></dl>
      <h1>Rechnung Oktober 2026</h1>
      <table><tr><th>Leistung</th><th>Zeitraum</th><th class="r">Betrag</th></tr>
        <tr><td>Glasfaser 500/100</td><td>01.10. bis 31.10.</td><td class="r">49,90 €</td></tr>
        <tr><td>Feste IPv4-Adresse</td><td>01.10. bis 31.10.</td><td class="r">5,00 €</td></tr>
        <tr><td>Router-Miete</td><td>01.10. bis 31.10.</td><td class="r">3,00 €</td></tr>
        <tr class="sum"><td>Summe netto</td><td></td><td class="r">48,25 €</td></tr>
        <tr><td>USt. 20 %</td><td></td><td class="r">9,65 €</td></tr>
        <tr class="total"><td>Gesamt</td><td></td><td class="r">57,90 €</td></tr></table>
      <p>Wir buchen den Betrag am 15.11.2026 von deinem Konto AT61 1904 3002 3457 3201 ab. Danke, dass du bei uns bist.</p>${foot(0, 1)}`];
    if (n.name === 'Mietvertrag.pdf') {
      const secs = [['§ 1 Mietgegenstand', 'Vermietet wird die Wohnung Top 7 im zweiten Stock des Hauses Hauptplatz 1, 4600 Wels, bestehend aus drei Zimmern, Küche, Bad, WC und Abstellraum, insgesamt 78 m².'],
        ['§ 2 Mietdauer', 'Das Mietverhältnis beginnt am 1. März 2024 und wird auf fünf Jahre abgeschlossen. Es endet am 28. Februar 2029, ohne dass es einer Kündigung bedarf.'],
        ['§ 3 Miete', 'Die monatliche Hauptmiete beträgt 690 Euro. Sie ist jeweils am Fünften eines Monats im Voraus fällig.'],
        ['§ 4 Betriebskosten', 'Zusätzlich zur Hauptmiete wird eine Akontozahlung für Betriebskosten von 140 Euro monatlich vereinbart.'],
        ['§ 5 Kaution', 'Die Mieterin oder der Mieter leistet eine Kaution in Höhe von drei Bruttomonatsmieten.'],
        ['§ 6 Erhaltung', 'Die Wohnung ist pfleglich zu behandeln und bei Rückgabe in dem Zustand zu übergeben, in dem sie übernommen wurde.'],
        ['§ 7 Haustiere', 'Die Haltung von Kleintieren ist gestattet. Für Hunde und Katzen ist die Zustimmung der Vermieterin einzuholen.'],
        ['§ 8 Untervermietung', 'Eine gänzliche oder teilweise Untervermietung ist ohne schriftliche Zustimmung nicht gestattet.'],
        ['§ 9 Schlussbestimmungen', 'Gerichtsstand ist Wels. Es gilt österreichisches Recht.']];
      const pages = [];
      for (let k = 0; k < 5; k++) {
        const own = secs.slice(k * 2, k * 2 + 2);
        pages.push(`${k === 0 ? '<h1>Mietvertrag über Wohnräume</h1><p class="lead">abgeschlossen zwischen Maria Huber, Vermieterin, und Junker der Provinz, Mieter.</p>' : ''}
          ${own.map(([h, t]) => `<h2>${h}</h2><p>${t}</p>${para(r, 1)}`).join('')}
          ${k === 4 ? '<div class="sig"><span>Wels, am 12.02.2024</span><span>Vermieterin</span><span>Mieter</span></div>' : ''}${foot(k, 5)}`);
      }
      return pages;
    }
    if (n.name === 'Lebenslauf.pdf') return [`<h1>Lebenslauf</h1><p class="lead">Junker der Provinz, Wels</p>
        <h2>Berufserfahrung</h2><dl class="cv"><dt>seit 2021</dt><dd>Systemadministration, Stadtwerke Wels</dd><dt>2016 bis 2021</dt><dd>IT-Support, Ordination Dr. Berger</dd><dt>2014 bis 2016</dt><dd>Lehre als Informationstechniker</dd></dl>
        <h2>Ausbildung</h2><dl class="cv"><dt>2014</dt><dd>HTL Wels, Elektronik</dd></dl>${foot(0, 2)}`,
      `<h2>Kenntnisse</h2><dl class="cv"><dt>Systeme</dt><dd>Linux, Unraid, Proxmox, Windows Server</dd><dt>Netzwerk</dt><dd>VLANs, WireGuard, pfSense</dd><dt>Sprachen</dt><dd>Deutsch, Englisch</dd></dl><h2>Interessen</h2><p>Heimserver, Fotografie, Bergwandern.</p>${foot(1, 2)}`];
    if (n.name === 'Bewerbung.docx') return [`
      <div class="addr r">Junker der Provinz<br>Hauptplatz 1<br>4600 Wels</div>
      <div class="addr">Linz AG<br>Personalabteilung<br>Wiener Straße 151<br>4021 Linz</div>
      <p class="date">Wels, 28.10.2026</p>
      <h1>Bewerbung als Systemadministrator</h1>
      <p>Sehr geehrte Damen und Herren,</p>
      <p>mit großem Interesse habe ich Ihre Ausschreibung gelesen. Seit fünf Jahren betreue ich die Server und das Netzwerk der Stadtwerke Wels und möchte diese Erfahrung nun in einem größeren Team einbringen.</p>
      <p>Besonders reizt mich der Ausbau Ihrer Speicherlandschaft. Zuhause betreibe ich selbst einen Unraid-Server mit Backups auf ein zweites Gerät im Keller, die Grundlagen dafür sind mir also vertraut.</p>
      <p>Über eine Einladung zu einem Gespräch freue ich mich.</p>
      <p>Mit freundlichen Grüßen<br><br>Junker der Provinz</p>${foot(0, 1)}`];
    const count = Math.max(1, Math.min(8, Math.round(n.size / (150 * KB))));
    return Array.from({ length: count }, (_, k) => `${k === 0 ? `<h1>${esc(n.name.replace(/\.[^.]+$/, ''))}</h1>` : ''}<h2>Abschnitt ${k + 1}</h2>${para(r, 3)}${foot(k, count)}`);
  }

  function pager(host, pages, ctx) {
    const info = plural(pages.length, 'Seite', 'Seiten');
    if (ctx.compact) { host.innerHTML = `<div class="cv-paper"><div class="pg">${pages[0]}</div></div>`; return { info }; }
    host.innerHTML = `<div class="cv-bar"><button class="btn icon" data-pg="-1" data-tip="Vorherige Seite (Bild auf)" aria-label="Vorherige Seite">${g('IconBack')}</button>
        <span class="pill glim-num" id="cv-pgn">Seite 1 von ${pages.length}</span>
        <button class="btn icon" data-pg="1" data-tip="Nächste Seite (Bild ab)" aria-label="Nächste Seite">${g('IconForward')}</button></div>
      <div class="cv-doc">${pages.length > 1 ? `<div class="cv-pthumbs">${pages.map((p, k) => `<button class="cv-pthumb ${k ? '' : 'cur'}" data-pk="${k}" aria-label="Seite ${k + 1}"><span class="cv-paper"><span class="pg">${p}</span></span><span class="glim-num">${k + 1}</span></button>`).join('')}</div>` : ''}
        <div class="cv-pages">${pages.map((p, k) => `<div class="cv-paper" data-page="${k}"><div class="pg">${p}</div></div>`).join('')}</div></div>`;
    const box = $('.cv-pages', host);
    let cur = 0;
    const mark = (k) => {
      cur = k;
      $('#cv-pgn', host).textContent = `Seite ${k + 1} von ${pages.length}`;
      $$('.cv-pthumb', host).forEach((t, i) => t.classList.toggle('cur', i === k));
      $('.cv-pthumb.cur', host)?.scrollIntoView({ block: 'nearest' });
    };
    const go = (k) => { k = Math.max(0, Math.min(pages.length - 1, k)); box.querySelector(`[data-page="${k}"]`).scrollIntoView({ block: 'start', behavior: 'smooth' }); mark(k); };
    box.addEventListener('scroll', () => {
      const mid = box.getBoundingClientRect().top + box.clientHeight * .35;
      const k = $$('[data-page]', box).findLastIndex(p => p.getBoundingClientRect().top <= mid);
      if (k >= 0 && k !== cur) mark(k);
    });
    host.addEventListener('click', (e) => {
      const b = e.target.closest('[data-pg]'), t = e.target.closest('[data-pk]');
      if (b) go(cur + +b.dataset.pg);
      if (t) go(+t.dataset.pk);
    });
    return {
      info,
      keys: (e, combo) => {
        if (['PageDown', 'ArrowRight'].includes(combo)) return go(cur + 1), true;
        if (['PageUp', 'ArrowLeft'].includes(combo)) return go(cur - 1), true;
        if (combo === 'Home') return go(0), true;
        if (combo === 'End') return go(pages.length - 1), true;
      },
    };
  }
  R.pdf = (host, n, ctx) => pager(host, docPages(n), ctx);

  const MONTHS = ['Jänner', 'Februar', 'März', 'April', 'Mai', 'Juni', 'Juli', 'August', 'September', 'Oktober', 'November', 'Dezember'];
  const eur = (v) => v.toLocaleString('de-AT', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  function sheetData(n, tab) {
    const r = rng(n.name + tab);
    if (tab === 1) {
      const rows = [['Datum', 'Beleg', 'Kategorie', 'Betrag']];
      const cats = ['Arbeitsmittel', 'Fahrtkosten', 'Fortbildung', 'Spenden', 'Internet'];
      for (let k = 0; k < 18; k++) rows.push([`${String(1 + Math.floor(r() * 28)).padStart(2, '0')}.${String(1 + Math.floor(k * 12 / 18)).padStart(2, '0')}.2025`, `B-${String(k + 1).padStart(3, '0')}`, cats[Math.floor(r() * cats.length)], eur(12 + r() * 380)]);
      return rows;
    }
    const rows = [['Monat', 'Einnahmen', 'Ausgaben', 'Saldo', 'Belege']];
    let a = 0, b = 0;
    MONTHS.forEach(m => { const x = 3100 + Math.round(r() * 400), y = 2200 + Math.round(r() * 900); a += x; b += y; rows.push([m, eur(x), eur(y), eur(x - y), String(2 + Math.floor(r() * 9))]); });
    rows.push(['Summe', eur(a), eur(b), eur(a - b), '']);
    return rows;
  }
  R.office = (host, n, ctx) => {
    const e = FS.ext(n.name);
    if (e === 'xlsx') return sheet(host, n, ctx);
    if (e === 'pptx') return slides(host, n, ctx);
    return pager(host, docPages(n), ctx);
  };
  function sheet(host, n, ctx) {
    const tabs = ['Übersicht', 'Belege', 'Notizen'];
    let tab = 0, sel = [0, 0];
    const cols = 'ABCDEFG'.split('');
    const formula = (rows, rr, cc) => rows[rr]?.[0] === 'Summe' && cc > 0 && cc < 4 ? `=SUMME(${cols[cc]}2:${cols[cc]}13)` : cc === 3 && rr > 0 && tab === 0 ? `=B${rr + 1}-C${rr + 1}` : rows[rr]?.[cc] ?? '';
    const paint = () => {
      const rows = tab === 2 ? [['Steuerberatung am 14.03. um 10 Uhr'], ['Pendlerpauschale nicht vergessen']] : sheetData(n, tab);
      const nRows = ctx.compact ? 12 : Math.max(rows.length + 6, 24);
      host.innerHTML = `${ctx.compact ? '' : `<div class="cv-bar"><span class="pill glim-num cv-ref">${cols[sel[1]]}${sel[0] + 1}</span><span class="cv-fx mono">${esc(formula(rows, sel[0], sel[1]))}</span></div>`}
        <div class="cv-sheet"><table><thead><tr><th></th>${cols.map(c => `<th>${c}</th>`).join('')}</tr></thead><tbody>
        ${Array.from({ length: nRows }, (_, rr) => `<tr><th class="glim-num">${rr + 1}</th>${cols.map((c, cc) => {
          const v = rows[rr]?.[cc] ?? '';
          return `<td class="${rr === 0 || rows[rr]?.[0] === 'Summe' ? 'b' : ''} ${/^-?[\d\s.]+,\d\d$|^\d+$/.test(v) ? 'r glim-num' : ''} ${!ctx.compact && rr === sel[0] && cc === sel[1] ? 'sel' : ''}" data-rc="${rr},${cc}">${esc(v)}</td>`;
        }).join('')}</tr>`).join('')}</tbody></table></div>
        ${ctx.compact ? '' : `<div class="cv-tabs">${tabs.map((t, k) => `<button class="tab ${k === tab ? 'cur' : ''}" data-st="${k}">${t}</button>`).join('')}</div>`}`;
    };
    paint();
    host.addEventListener('click', (e) => {
      const c = e.target.closest('[data-rc]'), t = e.target.closest('[data-st]');
      if (c && !ctx.compact) { sel = c.dataset.rc.split(',').map(Number); paint(); }
      if (t) { tab = +t.dataset.st; sel = [0, 0]; paint(); }
    });
    return {
      info: 'Tabelle, 3 Blätter',
      keys: (e, combo) => {
        const d = { ArrowDown: [1, 0], ArrowUp: [-1, 0], ArrowRight: [0, 1], ArrowLeft: [0, -1] }[combo];
        if (d) { sel = [Math.max(0, sel[0] + d[0]), Math.max(0, Math.min(6, sel[1] + d[1]))]; paint(); return true; }
      },
    };
  }
  const SLIDES = [
    { t: 'Heimserver 2026', s: 'Was im Keller so läuft', title: true },
    { t: 'Was läuft', b: ['Plex für Filme und Musik', 'CannonCommander für die Dateien', 'Nginx Proxy Manager davor', 'Backups jede Nacht um 3 Uhr'] },
    { t: 'Speicher', chart: [['Filme', 62], ['Fotos', 18], ['Backups', 14], ['Rest', 6]] },
    { t: 'Backups', b: ['appdata täglich, 14 Stände', 'Flash wöchentlich', 'Zweite Kopie im Keller-NAS', 'Einmal im Monat testweise zurückspielen'] },
    { t: 'Nächste Schritte', b: ['USV-Akku tauschen', 'Disk 2 beobachten', 'Fotos nach Jahr sortieren'] },
  ];
  const slideHtml = (s) => s.title ? `<div class="sl title"><h1>${s.t}</h1><p>${s.s}</p></div>`
    : `<div class="sl"><h2>${s.t}</h2>${s.b ? `<ul>${s.b.map(x => `<li>${x}</li>`).join('')}</ul>` : `<div class="bars">${s.chart.map(([l, v]) => `<div><span>${l}</span><i style="width:${v}%"></i><b>${v} %</b></div>`).join('')}</div>`}</div>`;
  function slides(host, n, ctx) {
    const info = plural(SLIDES.length, 'Folie', 'Folien');
    if (ctx.compact) { host.innerHTML = `<div class="cv-slide">${slideHtml(SLIDES[0])}</div>`; return { info }; }
    let cur = 0;
    const paint = () => {
      host.innerHTML = `<div class="cv-bar"><button class="btn icon" data-sl="-1" data-tip="Vorherige Folie" aria-label="Vorherige Folie">${g('IconBack')}</button><span class="pill glim-num">Folie ${cur + 1} von ${SLIDES.length}</span><button class="btn icon" data-sl="1" data-tip="Nächste Folie" aria-label="Nächste Folie">${g('IconForward')}</button></div>
        <div class="cv-doc"><div class="cv-pthumbs">${SLIDES.map((s, k) => `<button class="cv-pthumb ${k === cur ? 'cur' : ''}" data-sk="${k}" aria-label="Folie ${k + 1}"><span class="cv-slide">${slideHtml(s)}</span><span class="glim-num">${k + 1}</span></button>`).join('')}</div>
        <div class="cv-stagebox"><div class="cv-slide big">${slideHtml(SLIDES[cur])}</div></div></div>`;
    };
    paint();
    const go = (k) => { cur = Math.max(0, Math.min(SLIDES.length - 1, k)); paint(); };
    host.addEventListener('click', (e) => {
      const b = e.target.closest('[data-sl]'), t = e.target.closest('[data-sk]');
      if (b) go(cur + +b.dataset.sl);
      if (t) go(+t.dataset.sk);
    });
    return { info, keys: (e, combo) => { if (['ArrowRight', 'PageDown', ' '].includes(combo)) return go(cur + 1), true; if (['ArrowLeft', 'PageUp'].includes(combo)) return go(cur - 1), true; } };
  }

  function archiveList(n, folder) {
    if (n.zip) {
      const out = [];
      const walk = (list, pre) => list.forEach(k => k.dir ? (out.push({ path: pre + k.name + '/', dir: true }), walk(k.children || [], pre + k.name + '/')) : out.push({ path: pre + k.name, size: k.size }));
      walk(n.zip, '');
      return out;
    }
    if (/^appdata-.*\.tar/.test(n.name)) {
      const out = [];
      FS.walk((k, p) => out.push({ path: 'appdata' + p + (k.dir ? '/' : ''), size: k.dir ? null : k.size, dir: k.dir }), FS.nodeAt('/mnt/user/appdata'));
      return out;
    }
    if (/^flash-/.test(n.name)) return ['bzimage', 'bzroot', 'bzmodules', 'bzfirmware', 'config/', 'config/ident.cfg', 'config/go', 'config/super.dat', 'config/disk.cfg', 'config/plugins/', 'syslinux/syslinux.cfg']
      .map(p => ({ path: p, dir: p.endsWith('/'), size: p.endsWith('/') ? null : rng(p)() * (p.startsWith('bz') ? 140 * MB : 8 * KB) }));
    const r = rng(n.name);
    return Array.from({ length: 6 }, (_, k) => ({ path: `datei-${k + 1}.bin`, size: Math.floor(r() * n.size / 4) }));
  }
  R.arch = (host, n, ctx) => {
    const list = archiveList(n, ctx.folder), files = list.filter(x => !x.dir);
    const unpacked = files.reduce((a, x) => a + x.size, 0);
    const info = `${FS.ext(n.name).toUpperCase()}, ${plural(files.length, 'Datei', 'Dateien')}, entpackt ${fmtSize(unpacked)}`;
    const shown = ctx.compact ? list.slice(0, 10) : list;
    host.innerHTML = `${ctx.compact || !n.zip || FS.inArchive(ctx.folder) ? '' : `<div class="cv-bar"><span class="grow"></span><button class="btn" id="cv-browse">${g('IconFolderOpen')}Wie einen Ordner öffnen</button></div>`}
      <div class="cv-scroll"><div class="cv-mini">${shown.map(x => `<div class="cv-mrow ${x.dir ? 'dir' : ''}"><span class="tile">${g(x.dir ? 'IconFolder' : FS.glyphOf({ name: x.path }))}</span><span class="t mono">${esc(x.path)}</span>${x.dir ? '<span></span>' : `<span class="pill glim-num">${fmtSize(Math.round(x.size))}</span>`}</div>`).join('')}</div>
      ${list.length > shown.length ? `<div class="muted cv-more">und ${plural(list.length - shown.length, 'weiterer Eintrag', 'weitere Einträge')}</div>` : ''}</div>`;
    $('#cv-browse', host)?.addEventListener('click', () => { CC.closeWin(); Files.go(FS.join(ctx.folder, n.name + '::')); });
    return { info };
  };

  const ISO = {
    'ubuntu-24.04.3-desktop-amd64.iso': ['Ubuntu 24.04.3 LTS amd64', ['casper/', 'boot/', 'EFI/', 'dists/', 'pool/', 'md5sum.txt']],
    'Win11_24H2_German_x64.iso': ['CCCOMA_X64FRE_DE-DE_DV9', ['boot/', 'efi/', 'sources/', 'support/', 'autorun.inf', 'setup.exe']],
    'debian-13.1.0-amd64-netinst.iso': ['Debian 13.1.0 amd64 n', ['boot/', 'dists/', 'EFI/', 'install.amd/', 'pool/', 'md5sum.txt']],
  };
  R.disk = (host, n, ctx) => {
    const iso = FS.ext(n.name) === 'iso', known = ISO[n.name];
    const hasSum = FS.kids(FS.nodeAt(ctx.folder)).some(k => k.name === n.name + '.sha256');
    const parts = iso ? null : n.name.startsWith('vm-') ? [['EFI', 100 * MB], ['MSR', 16 * MB], ['Windows, NTFS', n.size - 866 * MB], ['Wiederherstellung', 750 * MB]] : [[n.name === 'docker.img' ? 'Docker, Btrfs' : 'libvirt, Btrfs', n.size]];
    const info = iso ? 'ISO 9660 mit UDF' : 'Rohes Disk-Image';
    const row = (l, v, tip) => frow(l, `<span class="pill glim-num">${esc(v)}</span>`, tip);
    host.innerHTML = `<div class="cv-scroll cv-disk"><div class="well"><div class="eyebrow">Abbild</div>
        ${row('Format', info)}
        ${iso ? row('Bezeichnung', known ? known[0] : n.name.replace(/\.iso$/, '').toUpperCase()) : ''}
        ${iso ? row('Startet mit', 'BIOS und UEFI') : row('Belegt', fmtSize(n.size * .41), 'Das Abbild ist dünn angelegt: Es belegt auf der Platte nur so viel, wie wirklich darin steht.')}
        ${frow('Prüfsumme', `<span class="pill ${hasSum ? 'ok' : ''} glim-num">${hasSum ? 'liegt daneben' : 'keine'}</span>`, hasSum ? 'Neben dem Abbild liegt eine .sha256-Datei. Unter Datei, Prüfsumme prüfen vergleichst du beide.' : '')}
      </div>
      ${parts ? `<div class="well"><div class="eyebrow">Partitionen</div><div class="cv-parts">${parts.map(([l, s], k) => `<i style="flex:${Math.max(s / n.size, .03)}" class="p${k}"></i>`).join('')}</div>
        ${parts.map(([l, s], k) => frow(`<span class="cv-key p${k}"></span>${esc(l)}`, `<span class="pill glim-num">${fmtSize(s)}</span>`)).join('')}</div>`
      : `<div class="well"><div class="eyebrow">Inhalt</div><div class="cv-mini">${(known ? known[1] : ['boot/', 'EFI/']).map(p => `<div class="cv-mrow"><span class="tile">${g(p.endsWith('/') ? 'IconFolder' : 'IconFile')}</span><span class="t mono">${p}</span><span></span></div>`).join('')}</div></div>`}</div>`;
    return { info };
  };

  // Everything without a better view gets the start of its bytes, as a lister would show it.
  R.file = (host, n, ctx) => {
    const r = rng(n.name);
    const magic = /\.(db|sqlite)$/.test(n.name) ? 'SQLite format 3\0' : n.name === 'bzimage' ? 'MZ\x90\0\x03\0\0\0' : n.name === 'bzroot' ? '070701' : '';
    const bytes = Array.from({ length: ctx.compact ? 192 : 512 }, (_, k) => k < magic.length ? magic.charCodeAt(k) : r() < .3 ? 0 : Math.floor(r() * 256));
    const rows = [];
    for (let o = 0; o < bytes.length; o += 16) {
      const b = bytes.slice(o, o + 16);
      rows.push(`<div class="hx"><span class="o">${o.toString(16).padStart(8, '0')}</span><span class="h">${b.map(x => x.toString(16).padStart(2, '0')).join(' ')}</span><span class="a">${esc(b.map(x => x >= 32 && x < 127 ? String.fromCharCode(x) : '.').join(''))}</span></div>`);
    }
    host.innerHTML = `${ctx.compact ? '' : `<div class="cv-bar"><span class="pill">Erste ${bytes.length} Byte</span>${info('Für diesen Dateityp gibt es keine Vorschau. Die Hexansicht zeigt den Anfang der Datei, so wie er auf der Platte steht.')}</div>`}
      <div class="cv-scroll cv-hex mono">${rows.join('')}</div>`;
    return { info: magic.startsWith('SQLite') ? 'SQLite-Datenbank' : 'Binärdatei' };
  };

  CC.css(`
.cv-scroll { flex: 1; min-height: 0; overflow: auto; }
.cv-swl { display: inline-flex; align-items: center; gap: .6rem; }
.cv-code { tab-size: 4; flex: 1; min-height: 0; overflow: auto; border-radius: var(--radius-card); background: var(--carbon-surface2); padding: .7rem 0; font-family: ui-monospace, "Cascadia Code", Menlo, monospace; font-size: 13px; line-height: 1.6; }
.cv-lines { counter-reset: ln; min-width: max-content; }
.cv-code.wrap .cv-lines { min-width: 0; }
.cv-code .ln { position: relative; padding: 0 1rem 0 4.2rem; white-space: pre; min-height: 1.6em; }
.cv-code.wrap .ln { white-space: pre-wrap; overflow-wrap: anywhere; }
.cv-code .ln::before { counter-increment: ln; content: counter(ln); position: absolute; left: 0; width: 3.1rem; text-align: right; color: var(--carbon-text-muted); font-variant-numeric: tabular-nums; user-select: none; }
.cv-code .ln:hover { background: var(--carbon-hover); }
.cv-compact .cv-code { max-height: 60vh; flex: none; }
i.c { font-style: normal; color: var(--carbon-text-muted); }
i.s { font-style: normal; color: var(--carbon-text-sub); }
i.k { font-style: normal; color: var(--accent-ink); }

.cv-md { max-width: 46rem; margin: 0 auto; padding: .5rem 1rem 2rem; font-size: 15px; line-height: 1.65; }
.cv-md.compact { padding: 0 .25rem; font-size: 13px; }
.cv-md h1 { font-size: 1.75em; margin: .4em 0 .5em; letter-spacing: -.01em; }
.cv-md h2 { font-size: 1.3em; margin: 1.4em 0 .4em; }
.cv-md h3, .cv-md h4 { font-size: 1.05em; margin: 1.2em 0 .3em; }
.cv-md p { margin: .6em 0; }
.cv-md a { color: var(--accent-ink); text-decoration: underline; text-underline-offset: 2px; }
.cv-md code { font-family: ui-monospace, "Cascadia Code", Menlo, monospace; font-size: .88em; padding: .1em .4em; border-radius: var(--radius-control); background: var(--carbon-surface2); }
.cv-md pre { padding: .8em 1em; border-radius: var(--radius-control); background: var(--carbon-surface2); overflow: auto; }
.cv-md pre code { padding: 0; background: none; }
.cv-md blockquote { margin: 1em 0; padding: .7em 1em; border-radius: var(--radius-control); background: var(--carbon-surface2); color: var(--carbon-text-sub); }
.cv-md ul, .cv-md ol { padding-left: 1.4em; }
.cv-md li { margin: .2em 0; }
.cv-md li.task { list-style: none; margin-left: -1.4em; display: flex; align-items: center; gap: .55em; }
.cv-md li.task .mark { width: 1.05em; height: 1.05em; border-radius: var(--radius-pill); background: var(--carbon-surface3); display: grid; place-items: center; flex: none; }
.cv-md li.task.done .mark { background: var(--accent); color: var(--accent-contrast); }
.cv-md li.task .mark svg { width: .7em; height: .7em; }
.cv-md li.task.done { color: var(--carbon-text-muted); text-decoration: line-through; }
.cv-md table { border-collapse: collapse; margin: 1em 0; font-size: .92em; }
.cv-md th, .cv-md td { padding: .4em .8em; text-align: left; }
.cv-md th { color: var(--carbon-text-muted); font-weight: 500; font-size: .9em; }
.cv-md tr + tr td, .cv-md tbody tr:first-child td { box-shadow: inset 0 1px 0 color-mix(in srgb, var(--carbon-text) 8%, transparent); }
.cv-md hr { border: 0; height: 1px; background: color-mix(in srgb, var(--carbon-text) 10%, transparent); margin: 1.5em 0; }

.cv-doc { flex: 1; min-height: 0; display: flex; gap: 1rem; }
.cv-pthumbs { width: 8.5rem; flex: none; overflow: auto; display: flex; flex-direction: column; gap: .8rem; padding: .3rem .4rem .3rem .2rem; }
.cv-pthumb { display: flex; flex-direction: column; align-items: center; gap: .3rem; font-size: 12px; color: var(--carbon-text-muted); border-radius: var(--radius-control); padding: .35rem; }
.cv-pthumb:hover { background: var(--carbon-hover); }
.cv-pthumb.cur { background: var(--accent); color: var(--accent-contrast); }
.cv-pthumb .cv-paper, .cv-pthumb .cv-slide { width: 100%; pointer-events: none; box-shadow: 0 1px 3px rgba(0,0,0,.3); }
.cv-pages { flex: 1; min-width: 0; overflow: auto; border-radius: var(--radius-card); background: var(--carbon-surface2); padding: 1.5rem; display: flex; flex-direction: column; align-items: center; gap: 1.5rem; }
.cv-pages .cv-paper { width: min(100%, 48rem); flex: none; box-shadow: var(--elevation); }
.cv-paper { container-type: inline-size; aspect-ratio: 1 / 1.414; background: #fff; color: #1d1d1f; border-radius: 4px; overflow: hidden; position: relative; display: block; }
.cv-paper .pg { display: block; height: 100%; padding: 8cqw 9cqw; font-size: 1.85cqw; line-height: 1.55; font-family: "Noto Serif", Georgia, serif; text-align: left; }
.cv-paper h1 { font-family: var(--font-sans); font-size: 2.2em; margin: .2em 0 .7em; letter-spacing: -.01em; }
.cv-paper h2 { font-family: var(--font-sans); font-size: 1.25em; margin: 1.3em 0 .35em; }
.cv-paper p { margin: .55em 0; }
.cv-paper .lead { color: #555; }
.cv-paper .lh { display: flex; justify-content: space-between; align-items: baseline; padding-bottom: 1em; margin-bottom: 2em; box-shadow: inset 0 -1px 0 #ddd; font-family: var(--font-sans); }
.cv-paper .lh b { font-size: 1.3em; } .cv-paper .lh span { color: #777; font-size: .85em; }
.cv-paper .addr { margin: 1em 0 1.5em; font-family: var(--font-sans); font-size: .95em; }
.cv-paper .addr.r { text-align: right; color: #555; }
.cv-paper .date { text-align: right; }
.cv-paper .meta, .cv-paper .cv { display: grid; grid-template-columns: auto 1fr; gap: .25em 1.5em; margin: 1em 0; font-family: var(--font-sans); font-size: .92em; }
.cv-paper dt { color: #777; } .cv-paper dd { margin: 0; }
.cv-paper table { width: 100%; border-collapse: collapse; font-family: var(--font-sans); font-size: .92em; margin: 1em 0 1.5em; }
.cv-paper th { text-align: left; color: #777; font-weight: 500; padding: .45em 0; box-shadow: inset 0 -1px 0 #ccc; }
.cv-paper td { padding: .45em 0; box-shadow: inset 0 -1px 0 #eee; }
.cv-paper .r { text-align: right; }
.cv-paper tr.sum td { padding-top: 1em; } .cv-paper tr.total td { font-weight: 700; box-shadow: inset 0 -2px 0 #1d1d1f; }
.cv-paper .sig { display: grid; grid-template-columns: 1fr 1fr; gap: 3em 2em; margin-top: 3em; font-family: var(--font-sans); font-size: .9em; }
.cv-paper .sig span:first-child { grid-column: 1 / -1; }
.cv-paper .sig span + span { padding-top: .5em; box-shadow: inset 0 1px 0 #999; color: #777; }
.cv-paper .pf { position: absolute; bottom: 4cqw; left: 0; right: 0; text-align: center; color: #999; font-size: 1.5cqw; font-family: var(--font-sans); }
.cv-compact .cv-paper { box-shadow: var(--elevation); }

.cv-sheet { flex: 1; min-height: 0; overflow: auto; border-radius: var(--radius-control); background: var(--carbon-surface); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--carbon-text) 8%, transparent); }
.cv-sheet table { border-collapse: collapse; font-size: 13px; min-width: 100%; }
.cv-sheet th { position: sticky; top: 0; background: var(--carbon-surface2); color: var(--carbon-text-muted); font-weight: 500; font-size: 12px; height: 1.7rem; min-width: 2.6rem; }
.cv-sheet tbody th { position: sticky; left: 0; top: auto; }
.cv-sheet td { height: 1.75rem; min-width: 7.5rem; padding: 0 .55rem; box-shadow: inset -1px -1px 0 color-mix(in srgb, var(--carbon-text) 8%, transparent); white-space: nowrap; cursor: cell; }
.cv-sheet td.r { text-align: right; } .cv-sheet td.b { font-weight: 600; }
.cv-sheet td.sel { box-shadow: inset 0 0 0 2px var(--accent-ink); background: var(--accent-soft); }
.cv-ref { min-width: 3.5rem; justify-content: center; }
.cv-fx { flex: 1; height: var(--btn-h); border-radius: var(--radius-control); background: var(--carbon-surface2); display: flex; align-items: center; padding: 0 .7rem; }
.cv-tabs { display: flex; gap: .35rem; flex: none; }

.cv-slide { container-type: inline-size; aspect-ratio: 16 / 9; background: #fff; color: #1d1d1f; border-radius: 4px; overflow: hidden; display: block; }
.cv-slide .sl { height: 100%; text-align: left; padding: 6cqw 7cqw; font-size: 2.6cqw; font-family: var(--font-sans); display: flex; flex-direction: column; }
.cv-slide .sl.title { justify-content: center; background: linear-gradient(135deg, #1d2b53, #3b5fa8); color: #fff; }
.cv-slide .sl.title h1 { font-size: 3em; margin: 0 0 .3em; } .cv-slide .sl.title p { opacity: .8; margin: 0; font-size: 1.2em; }
.cv-slide h2 { font-size: 1.8em; margin: 0 0 .8em; }
.cv-slide ul { margin: 0; padding-left: 1.2em; line-height: 1.7; }
.cv-slide .bars { display: flex; flex-direction: column; gap: .7em; }
.cv-slide .bars div { display: grid; grid-template-columns: 6em 1fr 3em; align-items: center; gap: .8em; }
.cv-slide .bars i { height: 1.1em; border-radius: .3em; background: #3b5fa8; }
.cv-slide .bars b { font-weight: 600; }
.cv-stagebox { flex: 1; min-width: 0; border-radius: var(--radius-card); background: var(--carbon-surface2); display: grid; place-items: center; padding: 1.5rem; container-type: size; }
.cv-slide.big { width: min(100cqw - 3rem, (100cqh - 3rem) * 16 / 9); box-shadow: var(--elevation); }

.cv-mrow.dir .tile svg { color: var(--accent-glyph); }
.cv-mrow .t.mono { font-size: 12.5px; }
.cv-disk { display: flex; flex-direction: column; gap: 1rem; max-width: 46rem; width: 100%; margin: 0 auto; }
.cv-parts { display: flex; gap: 3px; height: .9rem; border-radius: var(--radius-pill); overflow: hidden; }
.cv-parts i { min-width: .5rem; }
.p0 { background: var(--carbon-text-muted); } .p1 { background: var(--carbon-surface3); } .p2 { background: var(--accent); } .p3 { background: var(--carbon-text-sub); }
.cv-key { width: .7rem; height: .7rem; border-radius: 50%; display: inline-block; margin-right: .2rem; }
.cv-hex { border-radius: var(--radius-card); background: var(--carbon-surface2); padding: .7rem 1rem; font-size: 12.5px; line-height: 1.7; }
.cv-hex .hx { display: flex; gap: 1.6rem; white-space: pre; }
.cv-hex .o { color: var(--carbon-text-muted); } .cv-hex .a { color: var(--carbon-text-sub); }
.cv-compact .cv-hex .a { display: none; }

@media (max-width: 760px) {
  .cv-pthumbs { display: none; }
  .cv-pages { padding: .75rem; }
  .cv-hex .a { display: none; }
}
`);
})();
