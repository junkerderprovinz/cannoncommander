// Konto und Sicherheit: password, two-factor, passkeys, sessions and API tokens of the signed-in person.
(() => {
  const A = CC.admin;
  const { pill, bareBtn, row, list, foot, btn, field, fail } = A;

  DATA.account = DATA.account || { pwChanged: NOW - 41 * DAY, stayDays: 30, recovery: 8 };
  DATA.passkeys = DATA.passkeys || [
    { name: 'Windows Hello', origin: 'cannon.tower.lan', created: NOW - 60 * DAY, last: NOW - 3 * 60000 },
    { name: 'Handy', origin: '192.168.10.5:8443', created: NOW - 120 * DAY, last: NOW - 50 * DAY },
  ];
  const me = () => DATA.users.find(u => u.name === CC.user.name) || DATA.users[0];
  const redraw = A.redraw;

  const B32 = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  const randomOf = (alphabet, n) => Array.from({ length: n }, () => alphabet[Math.floor(Math.random() * alphabet.length)]).join('');
  const group4 = (s) => s.match(/.{1,4}/g).join(' ');

  // A made-up QR code: the three finder squares and a pattern seeded from the secret, enough to read as one.
  function qr(secret) {
    const n = 25;
    let s = [...secret].reduce((a, c) => (a * 31 + c.charCodeAt(0)) % 2147483647, 7);
    const next = () => (s = (s * 16807) % 2147483647) / 2147483647;
    const finder = (x, y) => (x < 7 && y < 7) || (x >= n - 7 && y < 7) || (x < 7 && y >= n - 7);
    const inFinder = (x, y) => {
      const fx = x >= n - 7 ? x - (n - 7) : x, fy = y >= n - 7 ? y - (n - 7) : y;
      return fx === 0 || fx === 6 || fy === 0 || fy === 6 || (fx >= 2 && fx <= 4 && fy >= 2 && fy <= 4);
    };
    let rects = '';
    for (let y = 0; y < n; y++) for (let x = 0; x < n; x++) {
      const on = finder(x, y) ? inFinder(x, y) : (x === 7 || y === 7 || x === n - 8 || y === n - 8) && (finder(x - 1, y) || finder(x, y - 1) || finder(x + 1, y) || finder(x, y + 1)) ? false : next() > .52;
      if (on) rects += `<rect x="${x + 2}" y="${y + 2}" width="1" height="1"/>`;
    }
    return `<svg viewBox="0 0 ${n + 4} ${n + 4}" shape-rendering="crispEdges" role="img" aria-label="QR-Code"><rect width="${n + 4}" height="${n + 4}" fill="#fff"/><g fill="#161616">${rects}</g></svg>`;
  }

  function showCodes(w, done) {
    const codes = Array.from({ length: 8 }, () => `${randomOf('ABCDEFGHJKLMNPQRSTUVWXYZ23456789', 4)}-${randomOf('ABCDEFGHJKLMNPQRSTUVWXYZ23456789', 4)}`);
    w.lock(true);
    w.set(`<div class="well a-text"><p>Diese acht Codes siehst du nur jetzt. Leg sie an einen sicheren Ort, etwa in deinen Passwortmanager.</p><p>Fehlt einmal das Handy, meldet dich jeder Code genau einmal an.</p></div>
      <div class="well"><div class="a-codes mono glim-num">${codes.map(c => `<span>${c}</span>`).join('')}</div>${foot(btn('Alle kopieren', 'IconCopy', `data-copy="${codes.join('\n')}"`))}</div>`,
      [{ label: 'Habe ich gesichert', g: 'IconCheck', primary: true, run: () => { w.lock(false); DATA.account.recovery = 8; done(); } }]);
  }

  function totpSetup() {
    const secret = randomOf(B32, 32);
    const w = A.stepWin({ title: 'Zwei-Faktor-Authentifizierung einrichten', wide: true });
    w.set(`<div class="well a-totp"><div class="a-qr">${qr(secret)}</div>
        <div class="a-totp-side"><div class="eyebrow">Mit der App scannen ${info('Jede App für Einmalcodes geht, etwa Aegis, 2FAS, Google Authenticator oder dein Passwortmanager.')}</div>
          ${frow('Oder abtippen', A.copyField(group4(secret)))}
          ${frow('Code aus der App', field('', 'id="a-totp" inputmode="numeric" maxlength="6" autocomplete="one-time-code" placeholder="123456"', 's mono'), 'Die sechs Ziffern, die die App gerade zeigt. Sie wechseln alle 30 Sekunden.')}</div></div>`,
      [cancel, { label: 'Weiter', g: 'IconForward', primary: true, run: (el) => {
        const code = $('#a-totp', el).value.replace(/\s/g, '');
        if (!/^\d{6}$/.test(code)) return fail('Gib die sechs Ziffern aus der App ein', $('#a-totp', el));
        if (code === '000000') return fail('Der Code passt nicht. Stimmt die Uhrzeit am Handy?', $('#a-totp', el));
        showCodes(w, () => { me().twofa = true; redraw(); CC.toast('Zwei-Faktor-Authentifizierung ist an'); });
        return false;
      } }]);
    $('#a-totp', w.el).addEventListener('keydown', e => { if (e.key === 'Enter') $('.win-foot .btn.primary', w.el).click(); });
  }

  function codeWin({ title, verb, glyph, done }) {
    win({ title,
      body: `<div class="well">${frow('Code aus der App', field('', 'id="a-code" inputmode="numeric" maxlength="9" autocomplete="one-time-code" placeholder="123456"', 's mono'), 'Ein Code aus der App oder einer deiner Wiederherstellungscodes.')}</div>`,
      foot: [cancel, { label: verb, g: glyph, primary: true, run: (w) => {
        const c = $('#a-code', w).value.trim();
        if (!/^\d{6}$/.test(c) && !/^[A-Z0-9]{4}-[A-Z0-9]{4}$/i.test(c)) return fail('Gib einen Code aus der App ein', $('#a-code', w));
        done();
      } }] });
  }

  function passwordWin() {
    win({ title: 'Passwort ändern',
      body: `<div class="well">${frow('Aktuelles Passwort', A.pwField('id="a-pw0" autocomplete="current-password"', 'm'))}
        ${frow('Neues Passwort', A.pwField('id="a-pw1"', 'm'), 'Mindestens 12 Zeichen. Ein neues Passwort meldet dich auf allen anderen Geräten ab, diese Sitzung bleibt angemeldet.')}
        ${frow('Wiederholen', A.pwField('id="a-pw2"', 'm'))}</div>`,
      foot: [cancel, { label: 'Passwort ändern', g: 'IconKey', primary: true, run: (w) => {
        const p0 = $('#a-pw0', w).value, p1 = $('#a-pw1', w).value, p2 = $('#a-pw2', w).value;
        if (!p0) return fail('Das aktuelle Passwort fehlt noch', $('#a-pw0', w));
        if (p1.length < 12) return fail('Das neue Passwort braucht mindestens 12 Zeichen', $('#a-pw1', w));
        if (p1 === p0) return fail('Das neue Passwort ist dasselbe wie das alte', $('#a-pw1', w));
        if (p1 !== p2) return fail('Die beiden neuen Passwörter sind nicht gleich', $('#a-pw2', w));
        const others = DATA.sessions.filter(s => !s.current && !s.device.startsWith('API-Token'));
        DATA.sessions = DATA.sessions.filter(s => !others.includes(s));
        DATA.account.pwChanged = A.now();
        redraw();
        CC.toast(others.length ? `Passwort geändert, ${others.length === 1 ? 'ein anderes Gerät ist' : `${others.length} andere Geräte sind`} abgemeldet` : 'Passwort geändert');
      } }] });
  }

  function passkeyWin() {
    const w = A.stepWin({ title: 'Passkey hinzufügen' });
    w.set(`<div class="well">${frow('Name', field('', 'id="a-pkname" placeholder="etwa Laptop"', 'm'), 'Damit erkennst du den Passkey in der Liste wieder.')}
        ${frow('Aktuelles Passwort', A.pwField('id="a-pkpw" autocomplete="current-password"', 'm'), 'CannonCommander fragt bei jedem neuen Zugang danach, damit nur du einen anlegen kannst.')}</div>`,
      [cancel, { label: 'Weiter', g: 'IconForward', primary: true, run: (el) => {
        const name = $('#a-pkname', el).value.trim();
        if (!name) return fail('Gib dem Passkey einen Namen', $('#a-pkname', el));
        if (DATA.passkeys.some(p => p.name === name)) return fail('Diesen Namen gibt es schon', $('#a-pkname', el));
        if (!$('#a-pkpw', el).value) return fail('Das aktuelle Passwort fehlt noch', $('#a-pkpw', el));
        w.set(`<div class="empty">${g('IconKey')}<div>Bestätige den Passkey im Fenster deines Browsers</div></div>`, [cancel]);
        setTimeout(() => {
          if (!w.el.isConnected) return;
          DATA.passkeys.push({ name, origin: location.hostname || 'cannon.tower.lan', created: A.now(), last: null });
          w.close();
          redraw();
          CC.toast(`Passkey ${name} hinzugefügt`);
        }, 1200);
        return false;
      } }]);
  }

  function tokenWin() {
    const w = A.stepWin({ title: 'Token erstellen' });
    w.set(`<div class="well">${frow('Name', field('', 'id="a-tkname" placeholder="etwa backup-skript"', 'm'), 'Wofür das Token da ist. Es steht später in der Liste der Sitzungen.')}
        ${frow('Rechte', seg([['r', 'Lesen'], ['rw', 'Lesen und schreiben']], 'r', 'id="a-tkrights"'), 'Das Token darf nie mehr als du selbst.')}
        ${frow('Läuft ab', seg([['30', '30 Tage'], ['365', '1 Jahr'], ['never', 'Nie']], '365', 'id="a-tkexp"'))}
        ${frow('Aktuelles Passwort', A.pwField('id="a-tkpw" autocomplete="current-password"', 'm'), 'CannonCommander fragt bei jedem neuen Zugang danach, damit nur du einen anlegen kannst.')}</div>`,
      [cancel, { label: 'Erstellen', g: 'IconKey', primary: true, run: (el) => {
        const name = $('#a-tkname', el).value.trim();
        if (!name) return fail('Gib dem Token einen Namen', $('#a-tkname', el));
        if (DATA.tokens.some(t => t.name === name)) return fail('Diesen Namen gibt es schon', $('#a-tkname', el));
        if (!$('#a-tkpw', el).value) return fail('Das aktuelle Passwort fehlt noch', $('#a-tkpw', el));
        const rights = segVal($('#a-tkrights', el)) === 'rw' ? 'lesen, schreiben' : 'lesen';
        const exp = segVal($('#a-tkexp', el));
        const token = 'cc_' + randomOf('ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789', 40);
        DATA.tokens.push({ name, created: A.now(), last: null, rights, expires: exp === 'never' ? null : A.now() + Number(exp) * DAY });
        w.lock(true);
        w.set(`<div class="well a-text"><p>Kopier das Token jetzt. CannonCommander zeigt es danach nie wieder, du kannst nur ein neues erstellen.</p></div>
          <div class="well">${frow(esc(name), A.copyField(token))}</div>`,
          [{ label: 'Fertig', g: 'IconCheck', primary: true, run: () => { w.lock(false); redraw(); CC.toast(`Token ${name} erstellt`); } }]);
        return false;
      } }]);
  }

  const sessionGlyph = (s) => s.device.startsWith('API-Token') ? 'IconKey' : /Android|iPhone|Handy/.test(s.device) ? 'IconPhone' : 'IconBrowser';

  CC.settingsTab({ id: 'security', label: 'Konto & Sicherheit', g: 'IconTabSecurity', order: 90, render(host) {
    const acc = DATA.account, u = me();
    const others = DATA.sessions.filter(s => !s.current && !s.device.startsWith('API-Token'));
    host.innerHTML = CC.card('Anmeldung',
      frow('Passwort', `<span class="a-ctl">${pill('Geändert ' + A.ago(acc.pwChanged))}${btn('Ändern', 'IconKey', 'data-pw')}</span>`, 'Ein neues Passwort meldet dich auf allen anderen Geräten ab. Diese Sitzung bleibt angemeldet.')
      + frow('Angemeldet bleiben', seg([[1, '1 Tag'], [7, '7 Tage'], [30, '30 Tage']], acc.stayDays, 'id="a-stay"'), 'Danach fragt CannonCommander wieder nach dem Passwort.'))
      + CC.card('Zwei-Faktor-Authentifizierung', u.twofa
        ? frow('Status', `<span class="a-ctl">${pill('An', 'ok')}${btn('Ausschalten', 'IconShieldOff', 'data-2off')}</span>`, 'Nach dem Passwort fragt CannonCommander nach einem Code aus deiner App.')
          + frow('Wiederherstellungscodes', `<span class="a-ctl">${pill(`${acc.recovery} übrig`, acc.recovery < 3 ? 'warn' : '')}${btn('Neu erzeugen', 'IconRefresh', 'data-2codes')}</span>`, 'Jeder Code meldet dich einmal an, falls das Handy fehlt. Neue Codes machen die alten ungültig.')
        : frow('Status', `<span class="a-ctl">${pill('Aus', 'state')}${btn('Einrichten', 'IconShieldOn', 'data-2on')}</span>`, 'Nach dem Passwort fragt CannonCommander zusätzlich nach einem Code aus einer App auf deinem Handy.'))
      + CC.card('Passkeys', list(DATA.passkeys.map((p, i) => row({
          tile: g('IconKey'), name: p.name, tip: `Gilt nur unter ${p.origin}. Das Passwort gilt daneben weiter.`,
          badges: [pill(p.origin, 'mono'), pill(p.last ? 'Benutzt ' + A.ago(p.last) : 'Noch nie benutzt')],
          acts: [bareBtn('IconTrash', 'Entfernen', `data-pkdel="${i}"`)],
        })), ['IconKey', 'Noch kein Passkey']) + foot(btn('Passkey hinzufügen', 'IconAdd', 'data-pkadd')))
      + CC.card('Sitzungen', list(DATA.sessions.map((s, i) => row({
          tile: g(sessionGlyph(s)), name: s.device,
          badges: [pill(s.where, 'mono'), s.current ? pill('Diese Sitzung', 'state') : pill(A.ago(s.last))],
          acts: [s.current ? '' : bareBtn('IconSignOut', 'Abmelden', `data-sout="${i}"`)],
        })), ['IconBrowser', 'Keine Sitzungen']) + (others.length ? foot(btn('Alle anderen abmelden', 'IconSignOut', 'data-soutall')) : ''))
      + CC.card('API-Tokens', list(DATA.tokens.map((t, i) => row({
          tile: g('IconKey'), name: t.name, tip: `Erstellt am ${fmtDate(t.created)}${t.expires ? `, läuft am ${fmtDate(t.expires)} ab` : ', läuft nicht ab'}.`,
          badges: [pill(t.rights), pill(t.last ? 'Benutzt ' + A.ago(t.last) : 'Noch nie benutzt')],
          acts: [bareBtn('IconKeyRevoke', 'Widerrufen', `data-trev="${i}"`)],
        })), ['IconKey', 'Noch kein Token. Skripte und andere Programme melden sich damit an.']) + foot(btn('Token erstellen', 'IconAdd', 'data-tadd')));

    $('#a-stay', host).addEventListener('change', e => { acc.stayDays = +segVal(e.target); CC.toast('Gespeichert'); });
    host.addEventListener('click', e => {
      const t = e.target.closest('button');
      if (!t || t.closest('[data-copy]')) return;
      const d = t.dataset;
      if ('pw' in d) passwordWin();
      else if ('2on' in d) totpSetup();
      else if ('2off' in d) codeWin({ title: 'Zwei-Faktor-Authentifizierung ausschalten', verb: 'Ausschalten', glyph: 'IconShieldOff', done: () => { u.twofa = false; redraw(); CC.toast('Zwei-Faktor-Authentifizierung ist aus. Das Passwort allein meldet dich jetzt an'); } });
      else if ('2codes' in d) codeWin({ title: 'Neue Wiederherstellungscodes', verb: 'Neu erzeugen', glyph: 'IconRefresh', done: () => setTimeout(() => showCodes(A.stepWin({ title: 'Neue Wiederherstellungscodes' }), () => { redraw(); CC.toast('Neue Codes gelten, die alten nicht mehr'); }), 0) });
      else if ('pkadd' in d) passkeyWin();
      else if ('pkdel' in d) {
        const p = DATA.passkeys[+d.pkdel];
        A.confirmWin({ title: 'Passkey entfernen', verb: 'Entfernen', glyph: 'IconTrash', text: `<p>${esc(p.name)} meldet dich danach nicht mehr an. Das Passwort gilt weiter.</p>`,
          run: () => { DATA.passkeys = DATA.passkeys.filter(x => x !== p); redraw(); CC.toast(`Passkey ${p.name} entfernt`); } });
      } else if ('sout' in d) {
        const s = DATA.sessions[+d.sout];
        DATA.sessions = DATA.sessions.filter(x => x !== s);
        redraw();
        CC.toast(`${s.device} ist abgemeldet`);
      } else if ('soutall' in d) {
        A.confirmWin({ title: 'Alle anderen abmelden', verb: 'Abmelden', glyph: 'IconSignOut',
          text: `<p>${others.length === 1 ? 'Ein anderes Gerät muss' : `${others.length} andere Geräte müssen`} sich neu anmelden. Diese Sitzung bleibt.</p><p>API-Tokens arbeiten weiter, die widerrufst du unten einzeln.</p>`,
          run: () => { DATA.sessions = DATA.sessions.filter(s => !others.includes(s)); redraw(); CC.toast('Alle anderen Geräte sind abgemeldet'); } });
      } else if ('tadd' in d) tokenWin();
      else if ('trev' in d) {
        const tk = DATA.tokens[+d.trev];
        A.confirmWin({ title: 'Token widerrufen', verb: 'Widerrufen', glyph: 'IconKeyRevoke', text: `<p>Was ${esc(tk.name)} benutzt, bekommt danach keinen Zugriff mehr.</p>`,
          run: () => {
            DATA.tokens = DATA.tokens.filter(x => x !== tk);
            DATA.sessions = DATA.sessions.filter(s => !s.device.includes(`„${tk.name}“`));
            redraw();
            CC.toast(`Token ${tk.name} widerrufen`);
          } });
      }
    });
  } });

  CC.css(`
.a-totp { flex-direction: row; gap: 1.2rem; align-items: center; }
.a-qr { width: 10rem; height: 10rem; flex: none; border-radius: var(--radius-control); overflow: hidden; }
.a-qr svg { width: 100%; height: 100%; }
.a-totp-side { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: .6rem; }
.a-totp-side .field.w { width: 18rem; }
.a-codes { display: grid; grid-template-columns: repeat(4, 1fr); gap: .5rem; }
.a-codes span { height: 2rem; display: grid; place-items: center; border-radius: var(--radius-control); background: var(--carbon-surface3); letter-spacing: .06em; }
@media (max-width: 760px) { .a-totp { flex-direction: column; align-items: stretch; } .a-qr { align-self: center; } .a-codes { grid-template-columns: repeat(2, 1fr); } }
`);
})();
