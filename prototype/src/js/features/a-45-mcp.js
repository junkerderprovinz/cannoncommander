// The MCP server on the Anbindungen tab: the endpoint, the keys of the signed-in account and, for an admin, the keys of
// every other account and the sign-in through OAuth. A key shows once, when it is made or replaced.
(() => {
  const A = CC.admin;
  const { pill, row, list, foot, btn, bareBtn, field, fail, redraw } = A;
  const MIN = 60000, HOUR = 60 * MIN;

  function key(id, owner, label, client, hint, canWrite, created, last, calls) {
    return { id, owner, label, client, hint, canWrite, created, rotated: 0, last, calls };
  }
  // The keys the app's fixture answers /api/v1/mcp/keys with, so both draw the card in the same state.
  DATA.mcp = DATA.mcp || {
    endpoint: 'https://cannon.tower.lan/mcp',
    keys: [
      key('a1b2c3d4e5f60718293a4b5c6d7e8f90', 'jdp', 'Claude Code auf dem Laptop', 'claude-code', 'q7Xe', false, NOW - 9 * DAY, NOW - 12 * MIN, 14),
      key('0f1e2d3c4b5a69788796a5b4c3d2e1f0', 'jdp', 'Cursor', 'cursor', 'Lm3k', true, NOW - 30 * DAY, NOW - 2 * DAY, 0),
      key('9a8b7c6d5e4f30211203f4e5d6c7b8a9', 'anna', 'LM Studio', 'lmstudio', 'Zr8p', false, NOW - 6 * DAY, NOW - 3 * HOUR, 5),
    ],
    revoked: [{ ...key('5c4b3a29180716253443526170819203', 'jdp', 'Erster Versuch', '', 'Tt0a', false, NOW - 40 * DAY, NOW - 39 * DAY, 0), revoked: NOW - 38 * DAY }],
    events: [
      { at: NOW - 12 * MIN, tool: 'list_folder', outcome: 'ok' },
      { at: NOW - 13 * MIN, tool: 'move_items', outcome: 'read_only_key' },
      { at: NOW - 14 * MIN, tool: 'search_files', outcome: 'ok' },
    ],
    oauth: { enabled: false, issuer: '' },
  };

  const MARKS = {
    claudecode: '<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path style="fill:var(--mark-ink,#d97757)" d="M21 10.5h3v3h-3v3h-1.5v3H18v-3h-1.5v3H15v-3H9v3H7.5v-3H6v3H4.5v-3H3v-3H0v-3h3v-6h18Zm-15 0h1.5v-3H6Zm10.5 0H18v-3h-1.5z"/></svg>',
    cursor: '<svg fill="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path d="M11.503.131 1.891 5.678a.84.84 0 0 0-.42.726v11.188c0 .3.162.575.42.724l9.609 5.55a1 1 0 0 0 .998 0l9.61-5.55a.84.84 0 0 0 .42-.724V6.404a.84.84 0 0 0-.42-.726L12.497.131a1.01 1.01 0 0 0-.996 0M2.657 6.338h18.55c.263 0 .43.287.297.515L12.23 22.918c-.062.107-.229.064-.229-.06V12.335a.59.59 0 0 0-.295-.51l-9.11-5.257c-.109-.063-.064-.23.061-.23"/></svg>',
    lmstudio: '<svg fill="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path d="M14.025 0c3.492 0 5.237 0 6.571.68a6.24 6.24 0 0 1 2.725 2.724C24 4.738 24 6.484 24 9.975v4.05c0 3.492 0 5.237-.68 6.571a6.24 6.24 0 0 1-2.724 2.725c-1.334.679-3.08.679-6.571.679h-4.05c-3.492 0-5.237 0-6.571-.68A6.24 6.24 0 0 1 .68 20.597C0 19.262 0 17.516 0 14.025v-4.05c0-3.492 0-5.237.68-6.571A6.23 6.23 0 0 1 3.404.68C4.738 0 6.484 0 9.975 0zM7.688 16.313a1.313 1.313 0 0 0 0 2.625h11.625a1.313 1.313 0 0 0 0-2.625zm-3-3.75a1.313 1.313 0 0 0 0 2.624h11.625a1.313 1.313 0 0 0 0-2.624zm3-3.75a1.313 1.313 0 0 0 0 2.624h11.625a1.313 1.313 0 0 0 0-2.624zm-3-3.75a1.313 1.313 0 0 0 0 2.625h11.625a1.313 1.313 0 0 0 0-2.625z"/></svg>',
  };
  const CLIENTS = [
    { id: 'claude-code', name: 'Claude Code', kind: 'Terminal', mark: MARKS.claudecode },
    { id: 'cursor', name: 'Cursor', kind: 'Editor', mark: MARKS.cursor },
    { id: 'lmstudio', name: 'LM Studio', kind: 'Lokale Modelle', mark: MARKS.lmstudio },
    { id: '', name: 'Anderer Client', kind: 'Streamable HTTP' },
  ];
  const clientOf = (id) => CLIENTS.find(c => c.id && c.id === id);
  const markOf = (c, glyph) => (c?.mark ? `<span class="mcp-mark">${c.mark}</span>` : g(glyph));

  const OUTCOME = { ok: 'erledigt', not_permitted: 'abgelehnt, das darf der Schlüssel nicht', read_only_key: 'abgelehnt, der Schlüssel darf nur lesen', rate_limited: 'abgelehnt, zu viele Anfragen' };
  const LIMITS = { perCall: 25, perHour: 200 };
  let showRevoked = false, opening = false;

  const mine = (k) => k.owner === CC.user.name;
  const admin = () => CC.user.role === 'Admin';
  const find = (id) => DATA.mcp.keys.find(k => k.id === id) || DATA.mcp.revoked.find(k => k.id === id);
  const connector = () => DATA.mcp.oauth.issuer + '/mcp';
  const oauthActive = () => DATA.mcp.oauth.enabled && !!DATA.mcp.oauth.issuer;
  const calls = (n) => `${n} ${n === 1 ? 'Aufruf' : 'Aufrufe'} heute`;
  const used = (k) => (k.last ? 'Benutzt ' + A.ago(k.last) : 'Noch nie benutzt');
  const facts = (k) => [clientOf(k.client)?.name, k.rotated ? `Ersetzt am ${fmtDate(k.rotated)}.` : `Angelegt am ${fmtDate(k.created)}.`].filter(Boolean).join(' ');
  const newKey = () => 'ccmcp_' + Array.from({ length: 40 }, () => 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_'[Math.floor(Math.random() * 64)]).join('');
  const hinted = (title, hint, inner, extra) => CC.card(title, inner, extra).replace('</span><div class="card-body">', ` ${info(hint)}</span><div class="card-body">`);
  const writeSw = (k, refused) => `<label class="mcp-write" ${refused ? `data-tip="${esc(refused)}"` : ''}>Schreiben${sw(k.canWrite, `data-mwrite="${k.id}"`)}</label>`;
  const act = (glyph, tip, what, k) => bareBtn(glyph, tip, `data-mact="${what}" data-k="${k.id}"`);
  const connectorRow = () => frow('Connector-URL', A.copyField(connector(), '', true), 'Diese Adresse trägst du im Cloud-Assistenten ein. Die Anmeldeseite findet er von dort selbst.');

  function cards() {
    const m = DATA.mcp, own = m.keys.filter(mine), gone = m.revoked.filter(mine), others = m.keys.filter(k => !mine(k));
    const status = own.length ? `An, ${own.length} Schlüssel aktiv` : oauthActive() ? 'An, wartet auf die Anmeldung eines Clients' : 'Aus, noch kein Schlüssel';
    const rows = own.map(k => row({
      tile: markOf(clientOf(k.client), 'IconKey'), name: k.label, tip: facts(k), attr: `data-key="${k.id}"`,
      badges: [pill('…' + k.hint, 'mono'), pill(calls(k.calls)), pill(used(k))],
      acts: [writeSw(k), act('IconPencil', 'Umbenennen', 'rename', k), act('IconRetry', 'Ersetzen', 'replace', k), act('IconLog', 'Protokoll', 'log', k), act('IconKeyRevoke', 'Widerrufen', 'revoke', k)],
    }));
    const revoked = gone.length ? foot(btn(`${gone.length} ${gone.length === 1 ? 'widerrufener' : 'widerrufene'} Schlüssel`, showRevoked ? 'IconCollapse' : 'IconExpand', 'data-mrevoked'))
      + (showRevoked ? list(gone.map(k => row({
        tile: `<span class="mcp-muted">${markOf(clientOf(k.client), 'IconKey')}</span>`, name: k.label, tip: `Widerrufen am ${fmtDate(k.revoked)}`,
        acts: [act('IconLog', 'Protokoll', 'log', k), act('IconTrash', 'Entfernen', 'purge', k)],
      })), []) : '') : '';
    let html = hinted('MCP-Server', 'Damit arbeitet ein KI-Assistent wie Claude oder ChatGPT mit deinen Dateien, mit genau deinen Rechten und Zugriffsregeln. Ein Schlüssel liest nur, bis du ihm das Schreiben erlaubst.',
      frow('Status', pill(status, own.length ? 'ok' : '', 'data-mcp="status"'))
      + frow('Endpunkt', A.copyField(m.endpoint, 'data-mcp="endpoint"', true), 'Hier verbindet sich dein Assistent. Er spricht Streamable HTTP und schickt den Schlüssel im Header Authorization oder X-API-Key.')
      + (oauthActive() ? connectorRow() : '')
      + frow('Client verbinden', btn('Client wählen', 'IconLink', 'id="mcp-choose"'), 'Wähl deinen Client, dann bekommt er einen eigenen Schlüssel und du die passende Konfiguration zum Kopieren. Was der Assistent liest, etwa Ordner, Dateinamen oder den Inhalt einer Textdatei, geht an den KI-Anbieter dahinter.')
      + `<div class="well"><div class="eyebrow">Deine Schlüssel</div>${list(rows, ['IconKey', 'Noch kein Schlüssel. Verbinde einen Client, dann legt CannonCommander einen für ihn an.'])}${revoked}</div>`,
      'data-mcp="card"');
    if (!admin()) return html;
    const on = m.oauth.enabled || opening;
    html += hinted('Schlüssel aller Konten', 'Hier siehst du die Schlüssel der anderen Konten. Du kannst einem das Schreiben wieder wegnehmen oder ihn widerrufen. Erlauben kann es nur, wem der Schlüssel gehört.',
      frow('Anmeldung über OAuth', sw(on, 'data-mcp="oauth"'), 'Für ChatGPT, Claude auf claude.ai und andere Dienste, die keinen Schlüssel annehmen. Sie schicken dich auf eine Seite von CannonCommander, dort meldest du dich an und erlaubst den Zugriff. Ausschalten meldet jeden dieser Clients ab.')
      + (on ? frow('Öffentliche Adresse', `<span class="a-ctl">${field(m.oauth.issuer, 'id="mcp-issuer" placeholder="https://files.example.com"', 'w mono')}${btn('Speichern', 'IconSave', 'id="mcp-issuer-save"')}</span>`, 'Die https-Adresse ohne Pfad, unter der CannonCommander aus dem Internet erreichbar ist, etwa https://files.example.com. Ändert sie sich, meldet sich jeder Client neu an.') : '')
      + (oauthActive() ? connectorRow() : '')
      + list(others.map(k => row({
        tile: markOf(clientOf(k.client), 'IconKey'), name: k.label, tip: facts(k), attr: `data-key="${k.id}"`,
        badges: [pill(k.owner, 'state'), pill(calls(k.calls)), pill(used(k))],
        acts: [writeSw(k, k.canWrite ? '' : `Schreiben erlauben kann nur ${k.owner}`), act('IconLog', 'Protokoll', 'log', k), act('IconKeyRevoke', 'Widerrufen', 'revoke', k)],
      })), ['IconAccounts', 'Die anderen Konten haben noch keinen Schlüssel.']),
      'data-mcp="admin"');
    return html;
  }

  function showOnce(w, k, secret, verb) {
    w.set(`<div class="well a-text"><p>Kopier den Schlüssel jetzt. CannonCommander behält nur einen Fingerabdruck davon und kann ihn nicht noch einmal zeigen.</p></div>
      <div class="well">${frow(esc(k.label), A.copyField(secret))}${frow('Endpunkt', A.copyField(DATA.mcp.endpoint))}</div>`,
      [{ label: 'Fertig', g: 'IconCheck', primary: true, run: () => { redraw(); CC.toast(verb); } }]);
  }

  function connect(c) {
    const w = A.stepWin({ title: `${c.name} verbinden` });
    w.set(`<div class="well">${frow('Name', field(c.id ? c.name : '', 'id="mcp-name" placeholder="etwa Claude Code auf dem Laptop"', 'm'), 'Steht später in der Liste deiner Schlüssel.')}</div>`,
      [cancel, { label: 'Schlüssel anlegen', g: 'IconKey', primary: true, run: (el) => {
        const name = $('#mcp-name', el).value.trim();
        if (!name) return fail('Gib dem Schlüssel einen Namen', $('#mcp-name', el));
        if (DATA.mcp.keys.some(k => mine(k) && k.label === name)) return fail('Ein aktiver Schlüssel heißt schon so', $('#mcp-name', el));
        const secret = newKey();
        const k = key(Math.random().toString(16).slice(2).padEnd(32, '0'), CC.user.name, name, c.id, secret.slice(-4), false, A.now(), 0, 0);
        DATA.mcp.keys.push(k);
        showOnce(w, k, secret, `Schlüssel ${name} angelegt`);
        return false;
      } }]);
  }

  function choose() {
    const tile = (c, i) => `<button class="btn mcp-client" data-mclient="${i}">${markOf(c, 'IconLink')}<span><b>${esc(c.name)}</b><small>${esc(c.kind)}</small></span></button>`;
    let close;
    const el = win({ title: 'Client wählen', wide: true, body: `<div class="mcp-pick">${CLIENTS.map(tile).join('')}</div>`, foot: [cancel], onOpen: (_, c) => { close = c; } });
    el.addEventListener('click', e => {
      const b = e.target.closest('[data-mclient]');
      if (!b) return;
      close();
      connect(CLIENTS[+b.dataset.mclient]);
    });
  }

  function logWin(k) {
    const events = k.id === DATA.mcp.keys[0]?.id ? DATA.mcp.events : [];
    const body = events.length
      ? `<div class="well mcp-log">${events.map(e => `<div><span class="glim-num">${fmtTime(e.at)}</span><span class="mono">${esc(e.tool)}</span><span class="${e.outcome === 'ok' ? '' : 'warn'}">${esc(OUTCOME[e.outcome] || 'endete mit ' + e.outcome)}</span></div>`).join('')}</div>`
      : '<div class="well a-text"><p>Dieser Schlüssel hat CannonCommander noch nicht aufgerufen.</p></div>';
    win({ title: k.label, body: body + '<div class="a-text muted"><p>CannonCommander hebt die Aufrufe eines Schlüssels bis zu einem Monat auf. Was der Assistent geschickt hat, speichert es nie, den Schlüssel selbst auch nicht.</p></div>', foot: [closeBtn] });
  }

  function rename(k) {
    const w = A.stepWin({ title: 'Umbenennen' });
    w.set(`<div class="well">${frow('Name', field(k.label, 'id="mcp-rename"', 'm'))}</div>`, [cancel, { label: 'Speichern', g: 'IconSave', primary: true, run: (el) => {
      const name = $('#mcp-rename', el).value.trim();
      if (!name) return fail('Gib dem Schlüssel einen Namen', $('#mcp-rename', el));
      k.label = name;
      redraw();
      CC.toast('Name gespeichert');
    } }]);
  }

  function setWrite(k, allow) {
    if (!allow) { k.canWrite = false; redraw(); CC.toast(`${k.label} darf nur noch lesen`); return; }
    redraw();
    if (!mine(k)) { fail(`Schreiben erlauben kann nur ${k.owner}`, $(`[data-key="${k.id}"] .sw`)); return; }
    A.confirmWin({ title: 'Schreiben erlauben', verb: 'Erlauben', glyph: 'IconUnlock',
      text: `<p>${esc(k.label)} darf danach auch Dateien ändern.</p><p>Mit dem Schreiben legt der Assistent Ordner an, kopiert, verschiebt, benennt um, legt in den Papierkorb und holt daraus zurück. Jede Änderung ist eine Aufgabe, die du rückgängig machen kannst, und endgültig gelöscht wird nie etwas. Höchstens ${LIMITS.perCall} Einträge pro Aufruf und ${LIMITS.perHour} pro Stunde.</p>`,
      run: () => { k.canWrite = true; redraw(); CC.toast(`${k.label} darf jetzt schreiben`); } });
  }

  function saveOAuth(enabled, issuer, input) {
    if (enabled && !/^https:\/\/[^/\s?#]+$/.test(issuer)) return fail('Gib bitte eine https-Adresse ohne Pfad ein, etwa https://files.example.com', input);
    DATA.mcp.oauth = { enabled, issuer };
    opening = false;
    redraw();
    CC.toast('Anmeldung über OAuth gespeichert');
  }

  function bind(host) {
    host.addEventListener('change', e => {
      const t = e.target;
      if (t.dataset.mwrite) setWrite(find(t.dataset.mwrite), swVal(t));
      else if (t.dataset.mcp === 'oauth') {
        const o = DATA.mcp.oauth;
        if (!swVal(t)) { opening = false; if (o.enabled) saveOAuth(false, o.issuer); else redraw(); }
        else if (o.issuer) saveOAuth(true, o.issuer);
        else { opening = true; redraw(); }
      }
    });
    host.addEventListener('keydown', e => { if (e.key === 'Enter' && e.target.id === 'mcp-issuer') saveOAuth(true, e.target.value.trim(), e.target); });
    host.addEventListener('click', e => {
      const b = e.target.closest('button');
      if (!b) return;
      if (b.id === 'mcp-choose') return choose();
      if (b.id === 'mcp-issuer-save') return saveOAuth(true, $('#mcp-issuer', host).value.trim(), $('#mcp-issuer', host));
      if ('mrevoked' in b.dataset) { showRevoked = !showRevoked; redraw(); return; }
      const k = b.dataset.k && find(b.dataset.k);
      if (!k) return;
      const what = b.dataset.mact;
      if (what === 'rename') rename(k);
      else if (what === 'log') logWin(k);
      else if (what === 'replace') A.confirmWin({ title: 'Schlüssel ersetzen', verb: 'Ersetzen', glyph: 'IconRetry',
        text: `<p>${esc(k.label)} bekommt einen neuen Schlüssel, der alte gilt sofort nicht mehr. Trag den neuen danach in jedem Client ein, der ihn benutzt.</p>`,
        run: () => {
          const secret = newKey();
          Object.assign(k, { hint: secret.slice(-4), rotated: A.now() });
          setTimeout(() => showOnce(A.stepWin({ title: 'Schlüssel ersetzen' }), k, secret, `Schlüssel ${k.label} ersetzt`), 0);
        } });
      else if (what === 'revoke') A.confirmWin({ title: 'Schlüssel widerrufen', verb: 'Widerrufen', glyph: 'IconKeyRevoke',
        text: `<p>${esc(k.label)} gilt danach nicht mehr. Aufgaben, die er gestartet hat, behalten seinen Namen.</p>`,
        run: () => {
          DATA.mcp.keys = DATA.mcp.keys.filter(x => x !== k);
          DATA.mcp.revoked.unshift({ ...k, revoked: A.now() });
          redraw();
          CC.toast(`Schlüssel ${k.label} widerrufen`);
        } });
      else if (what === 'purge') A.confirmWin({ title: 'Schlüssel entfernen', verb: 'Entfernen', glyph: 'IconTrash',
        text: `<p>${esc(k.label)} verschwindet mit seinem Protokoll aus der Liste.</p>`,
        run: () => { DATA.mcp.revoked = DATA.mcp.revoked.filter(x => x !== k); redraw(); CC.toast(`${k.label} entfernt`); } });
    });
  }

  CC.mcp = { cards, bind };

  CC.css(`
.mcp-mark { display: contents; }
.a-row .tile .mcp-mark svg, .mcp-client .mcp-mark svg { color: var(--carbon-text) !important; }
.a-row:hover .tile .mcp-mark svg { color: var(--accent-contrast) !important; }
.a-row .tile .mcp-muted svg { color: var(--carbon-text-muted) !important; }
.mcp-muted { display: contents; }
.mcp-write { display: inline-flex; align-items: center; gap: .5rem; margin-right: .3rem; font-size: 14px; cursor: pointer; }
.mcp-pick { display: grid; grid-template-columns: repeat(auto-fill, minmax(13rem, 1fr)); gap: .8125rem; }
.mcp-client { justify-content: flex-start; gap: .7rem; height: auto; padding: .7rem .8rem; text-align: left; }
.mcp-client svg { width: 1.5rem; height: 1.5rem; flex: none; }
.mcp-client b { display: block; font-weight: 600; }
.mcp-client small { display: block; font-size: 12px; color: var(--carbon-text-sub); }
.mcp-log { gap: .3rem; font-size: 13px; }
.mcp-log > div { display: flex; flex-wrap: wrap; gap: .2rem .6rem; }
.mcp-log .warn { color: var(--status-warn-text, #ff832b); }
@media (max-width: 760px) { .mcp-pick { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: .5rem; } }
`);
})();
