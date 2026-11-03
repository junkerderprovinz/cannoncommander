// Anmelden: name and password, a second step for the TOTP code, OIDC through Authentik and the reset through the container log.
(() => {
  const { secret } = CC.ck;
  const S = { step: 'pw', user: null, err: '' };
  const field = (id, attrs = '', cls = '') => `<input class="field grow ${cls}" id="${id}" ${attrs}>`;
  const pwField = (id, ph, auto) => secret(id, ph).replace('class="field w"', 'class="field grow"').replace('autocomplete="new-password"', `autocomplete="${auto}"`);

  const STEPS = {
    pw: () => ({
      title: 'Anmelden',
      body: `<div class="cl-fields">
          ${frow('Name', field('cl-user', `autocomplete="username" autocapitalize="off" spellcheck="false" value="${esc(S.user?.name || CC.user.name)}"`))}
          ${frow('Passwort', pwField('cl-pass', '', 'current-password'))}
          ${frow('Angemeldet bleiben', sw(true, 'id="cl-keep" type="button"'), 'Du bleibst 30 Tage angemeldet. Ohne den Schalter endet die Sitzung, wenn du den Browser schließt.')}
        </div>
        <div class="cl-btns">
          <button class="btn key primary" type="submit">${g('IconSignIn')}Anmelden</button>
          <button class="btn key" type="button" data-cl="oidc">${g('IconKey')}Mit Authentik anmelden</button>
          <div class="cl-row"><button class="btn cl-quiet" type="button" data-cl="forgot">${g('IconForgotPassword')}Passwort vergessen</button>${info('CannonCommander schreibt dann einen Code in sein Container-Log. Mit dem Code setzt du hier ein neues Passwort. Auf Unraid findest du das Log im Docker-Tab, Klick auf das CannonCommander-Symbol, Logs.')}</div>
        </div>`,
      focus: S.user ? '#cl-pass' : '#cl-user',
    }),
    totp: () => ({
      title: 'Bestätigen',
      body: `<div class="cl-totp"><div class="cl-lab">Code aus deiner Authenticator-App ${info('Die App zeigt alle 30 Sekunden einen neuen sechsstelligen Code. Hast du das Handy nicht dabei, geht auch einer deiner Wiederherstellungscodes.')}</div>
          <div class="cl-digits">${Array.from({ length: 6 }, (_, i) => `<input class="field glim-num" inputmode="numeric" maxlength="1" autocomplete="${i ? 'off' : 'one-time-code'}" aria-label="Ziffer ${i + 1}" data-d="${i}">`).join('')}</div></div>
        <div class="cl-btns"><button class="btn key primary" type="submit">${g('IconConfirm')}Bestätigen</button><button class="btn cl-quiet" type="button" data-cl="back">${g('IconBack')}Zurück</button></div>`,
      focus: '[data-d="0"]',
    }),
    reset: () => ({
      title: 'Passwort zurücksetzen',
      body: `<div class="cl-fields">
          ${frow('Code aus dem Container-Log', field('cl-code', 'placeholder="XXXX-XXXX-XXXX" spellcheck="false" autocomplete="one-time-code" autocapitalize="characters"', 'mono cl-code'), 'Der Code steht im Log des CannonCommander-Containers: auf Unraid im Docker-Tab, Klick auf das Symbol, Logs. Er gilt 10 Minuten. Das Zurücksetzen beendet alle anderen Sitzungen, schaltet die Zwei-Faktor-Anmeldung aus und widerruft alle API-Tokens.')}
          ${frow('Neues Passwort', pwField('cl-new', 'mindestens 12 Zeichen', 'new-password'))}
          ${frow('Wiederholen', pwField('cl-new2', '', 'new-password'))}
        </div>
        <div class="cl-btns"><button class="btn key primary" type="submit">${g('IconKey')}Passwort setzen</button><button class="btn cl-quiet" type="button" data-cl="back">${g('IconBack')}Zurück</button></div>`,
      focus: '#cl-code',
    }),
    done: () => ({
      title: 'Neues Passwort gesetzt',
      body: `<div class="well cl-gone"><div class="eyebrow">Damit sind auch weg</div>
          ${['Alle anderen Sitzungen', 'Die Zwei-Faktor-Anmeldung', 'Alle API-Tokens'].map(t => `<div class="cl-li">${g('IconCheck')}<span>${t}</span></div>`).join('')}</div>
        <div class="cl-btns"><button class="btn key primary" type="submit">${g('IconForward')}Weiter zu den Dateien</button></div>`,
    }),
    oidc: () => ({
      title: 'Anmelden',
      body: `<div class="cl-wait"><span class="cl-spin"></span><div>Weiter zu Authentik</div><div class="muted mono">auth.example.org</div></div>`,
    }),
  };

  function render(root) {
    const st = STEPS[S.step]();
    root.innerHTML = `<div class="cl">
      <form class="cl-card glim-card" id="cl-form" novalidate><span class="notch">${esc(st.title)}</span>
        <div class="cl-brand"><span class="brand-mark">${g('IconFolderOpen')}</span><span><span class="cl-name">CannonCommander</span><span class="muted">auf Bottich</span></span></div>
        ${S.err ? `<div class="cl-err" role="alert">${g('IconWarning')}<span>${esc(S.err)}</span></div>` : ''}
        ${st.body}
      </form>
      <div class="cl-ver muted glim-num">Version ${esc(DATA.version.app)}</div></div>`;
    wireControls(root);
    const form = $('#cl-form', root);
    form.addEventListener('submit', (e) => { e.preventDefault(); submit(form); });
    form.addEventListener('click', (e) => {
      const b = e.target.closest('[data-cl]');
      if (!b) return;
      const a = b.dataset.cl;
      S.err = '';
      if (a === 'back') S.step = 'pw';
      if (a === 'forgot') { S.step = 'reset'; CC.toast('Der Code steht jetzt im Container-Log'); }
      if (a === 'oidc') { S.step = 'oidc'; setTimeout(() => finish(DATA.users[0], 'über Authentik'), 1400); }
      CC.render();
    });
    const digits = $$('[data-d]', root);
    digits.forEach((d, i) => {
      d.addEventListener('input', () => {
        d.value = d.value.replace(/\D/g, '').slice(-1);
        if (d.value && i < 5) digits[i + 1].focus();
        if (digits.every(x => x.value)) submit(form);
      });
      d.addEventListener('keydown', (e) => { if (e.key === 'Backspace' && !d.value && i) { digits[i - 1].focus(); digits[i - 1].value = ''; } });
      d.addEventListener('paste', (e) => {
        const t = (e.clipboardData.getData('text') || '').replace(/\D/g, '').slice(0, 6);
        if (!t) return;
        e.preventDefault();
        digits.forEach((x, k) => { x.value = t[k] || ''; });
        if (t.length === 6) submit(form); else digits[t.length].focus();
      });
    });
    if (st.focus) setTimeout(() => $(st.focus, root)?.focus(), 0);
  }

  function submit(form) {
    const v = (id) => $('#' + id, form)?.value.trim() || '';
    const fail = (msg, focus) => { S.err = msg; CC.render(); setTimeout(() => $(focus)?.focus(), 0); };
    if (S.step === 'pw') {
      const name = v('cl-user'), pass = $('#cl-pass', form).value;
      if (!name || !pass) return fail('Gib Name und Passwort ein.', name ? '#cl-pass' : '#cl-user');
      const u = DATA.users.find(x => x.name.toLowerCase() === name.toLowerCase());
      if (!u) return fail('Name oder Passwort stimmt nicht.', '#cl-pass');
      S.user = u;
      S.err = '';
      if (u.twofa) { S.step = 'totp'; return CC.render(); }
      return finish(u);
    }
    if (S.step === 'totp') {
      const code = $$('[data-d]', form).map(d => d.value).join('');
      if (code.length < 6) return fail('Der Code hat sechs Ziffern.', '[data-d="0"]');
      return finish(S.user);
    }
    if (S.step === 'reset') {
      const code = v('cl-code').replace(/[\s-]/g, '');
      if (!/^[a-z0-9]{12}$/i.test(code)) return fail('Der Code hat zwölf Zeichen, so wie er im Log steht.', '#cl-code');
      if ($('#cl-new', form).value.length < 12) return fail('Das neue Passwort braucht mindestens 12 Zeichen.', '#cl-new');
      if ($('#cl-new', form).value !== $('#cl-new2', form).value) return fail('Die beiden Passwörter sind nicht gleich.', '#cl-new2');
      S.user = S.user || DATA.users[0];
      S.user.twofa = false;
      S.err = '';
      S.step = 'done';
      return CC.render();
    }
    if (S.step === 'done') finish(S.user);
  }

  function finish(u, how = '') {
    if (S.step === 'oidc' && CC.signedIn) return;
    CC.user = { name: u.name, full: u.full, role: u.role };
    u.last = Date.now();
    Object.assign(S, { step: 'pw', err: '' });
    CC.signedIn = true;
    CC.render();
    CC.toast(`Hallo ${u.full}, du bist angemeldet${how ? ' ' + how : ''}`);
  }

  CC.registerView('login', { render });
  // While the sign-in card is up, its keys stay with the form and never reach the file manager's shortcuts.
  $('#root').addEventListener('keydown', (e) => { if (!CC.signedIn) e.stopPropagation(); });

  CC.action('logout', { label: 'Abmelden', g: 'IconSignOut', run: () => {
    while (winStack.length) CC.closeWin();
    Object.assign(S, { step: 'pw', err: '', user: null });
    CC.signedIn = false;
    CC.render();
    CC.toast('Du bist abgemeldet');
  } });

  CC.css(`
.cl { min-height: 100vh; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 1.2rem; padding: 2rem 1rem 6rem; overflow: auto; }
.cl-card { position: relative; width: min(28rem, 100%); padding: 2rem 1.4rem 1.4rem; display: flex; flex-direction: column; gap: 1.2rem; }
.cl-brand { display: flex; align-items: center; gap: .8rem; padding: .2rem .1rem .3rem; }
.cl-brand .brand-mark { width: 2.75rem; height: 2.75rem; }
.cl-brand .brand-mark svg { width: 1.45rem; height: 1.45rem; }
.cl-brand > span:last-child { display: flex; flex-direction: column; }
.cl-name { font-size: 20px; font-weight: 700; letter-spacing: -.01em; line-height: 1.2; }
.cl-brand .muted { font-size: 13px; }
.cl-fields { display: flex; flex-direction: column; gap: .55rem; }
.cl .frow { flex-wrap: wrap; row-gap: .35rem; }
.cl .frow > .lab { flex: 1 0 7rem; }
.cl .frow > .field, .cl .frow > .cs-secret { flex: 1 1 13rem; }
.cl .cs-secret .field { width: 100%; }
.cl .cl-code { text-transform: uppercase; letter-spacing: .12em; }
.cl .cl-code::placeholder { letter-spacing: .12em; }
.cl-btns { display: flex; flex-direction: column; gap: .5rem; padding-top: .2rem; }
.cl-btns .btn { width: 100%; }
.cl-row { display: flex; align-items: center; gap: .5rem; }
.cl-row .btn { flex: 1; }
.cl-quiet { background: var(--carbon-surface2); color: var(--carbon-text-sub); }
.cl-quiet:hover { color: var(--carbon-text); }
.cl-err { display: flex; align-items: center; gap: .6rem; padding: .6rem .8rem; border-radius: var(--radius-control); background: var(--status-fail-bg); color: var(--carbon-text); font-size: 13px; }
.cl-err svg { width: 1rem; height: 1rem; color: var(--status-fail-text); flex: none; }
.cl-totp { display: flex; flex-direction: column; gap: .8rem; }
.cl-lab { display: flex; align-items: center; gap: .35rem; }
.cl-digits { display: grid; grid-template-columns: repeat(6, 1fr); gap: .45rem; }
.cl-digits .field { height: 3.1rem; text-align: center; font-size: 22px; font-weight: 600; padding: 0; width: 100%; }
.cl-digits .field:nth-child(3) { margin-right: .5rem; }
.cl-gone .cl-li { display: flex; align-items: center; gap: .6rem; min-height: 1.9rem; }
.cl-gone .cl-li svg { width: .9rem; height: .9rem; color: var(--status-ok-text); }
.cl-wait { display: flex; flex-direction: column; align-items: center; gap: .5rem; padding: 1.6rem 0 1.2rem; }
.cl-wait .muted { font-size: 12.5px; }
.cl-spin { width: 2rem; height: 2rem; border-radius: 50%; border: 3px solid var(--carbon-surface3); border-top-color: var(--accent); animation: cl-spin .8s linear infinite; margin-bottom: .4rem; }
@keyframes cl-spin { to { transform: rotate(360deg); } }
@media (prefers-reduced-motion: reduce) { .cl-spin { animation-duration: 2.4s; } }
.cl-ver { font-size: 12px; }
`);
})();
