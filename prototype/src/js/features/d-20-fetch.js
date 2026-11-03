// Von Adresse laden: the server fetches a file from a web address into the open folder, as a job with progress.
(() => {
  const { frow, cancel } = CC.ui;
  const { hash } = CC.ck;

  const parse = (text) => { try { const u = new URL(text.trim()); return /^(https?|ftp):$/.test(u.protocol) ? u : null; } catch { return null; } };
  const nameFrom = (u) => { const last = u.pathname.split('/').filter(Boolean).pop(); try { return decodeURIComponent(last || '') || u.hostname; } catch { return last; } };
  // The address decides the made-up size, so the same link always weighs the same.
  const sizeFor = (url, name) => (['iso', 'img'].includes(FS.ext(name)) ? 900 + hash(url) % 5200 : 2 + hash(url) % 400) * MB;

  CC.action('fetchUrl', { label: 'Von Adresse laden', g: 'IconImport', needs: 'writable', run: () => {
    const here = Files.T().path;
    const w = win({
      title: 'Von Adresse laden', cls: 'bk',
      body: `<div class="well bk-form">
          ${frow('Adresse', `<input class="field grow mono" id="fu-url" placeholder="https://example.org/datei.iso" autocomplete="off" spellcheck="false" inputmode="url">`, 'Der Server lädt die Datei selbst herunter. Dein Gerät muss dafür nicht online bleiben.')}
          ${frow('Dateiname', `<input class="field grow" id="fu-name" autocomplete="off" spellcheck="false">`, 'Kommt aus der Adresse, du kannst ihn ändern. Gibt es den Namen im Ordner schon, bekommt die neue Datei eine Nummer dazu.')}
          ${frow('Ziel', CC.bk.pathField('fu-dest', here), 'Der Ordner, der im aktiven Fenster offen ist.')}
        </div>`,
      foot: [{ start: 'Läuft als Aufgabe im Hintergrund' }, cancel, { label: 'Laden', g: 'IconImport', primary: true, run: (s) => {
        const u = parse($('#fu-url', s).value), name = $('#fu-name', s).value.trim(), dest = $('#fu-dest', s).value.trim();
        if (!u) { CC.toast('Das ist keine gültige Adresse. Sie beginnt mit https://, http:// oder ftp://', 'warn'); $('#fu-url', s).focus(); return false; }
        if (!name || /[/\0]/.test(name)) { CC.toast('Der Dateiname fehlt oder enthält einen Schrägstrich', 'warn'); $('#fu-name', s).focus(); return false; }
        const dir = FS.nodeAt(dest);
        if (!dir?.dir || FS.inArchive(dest)) { CC.toast('Diesen Zielordner gibt es nicht', 'warn'); return false; }
        start(u.href, name, dest);
      } }],
    });
    CC.bk.wirePicks(w);
    const url = $('#fu-url', w), nm = $('#fu-name', w);
    let typed = false;
    nm.addEventListener('input', () => { typed = !!nm.value.trim(); });
    url.addEventListener('input', () => { const u = parse(url.value); if (u && !typed) nm.value = nameFrom(u); });
    w.addEventListener('keydown', e => { if (e.key === 'Enter' && e.target.closest('input')) { e.preventDefault(); $('.win-foot .btn.primary', w).click(); } });
  } });

  function start(href, wanted, dest) {
    const size = sizeFor(href, wanted), host = new URL(href).hostname;
    const label = `Laden: ${wanted} von ${host}`;
    Jobs.start({ kind: 'fetch', label, total: size, speed: .45, files: [{ name: wanted, size }], done: () => {
      const dir = FS.nodeAt(dest);
      const name = FS.kids(dir).some(k => k.name === wanted) ? Files.freeName(dir, wanted) : wanted;
      dir.children.push({ name, size, time: Date.now(), owner: 'nobody', group: 'users', mode: '0664' });
      DATA.activity.unshift({ t: Date.now(), who: CC.user.name, what: 'geladen', obj: `${name} von ${host} nach ${dest}` });
      CC.toast(`${name} ist da`, 'ok', { label: 'Hingehen', run: () => { Files.go(dest); CC.bk.reveal(dest, name); } });
      if (CC.view === 'files') CC.render();
    } });
    CC.toast(`${wanted} wird geladen`, 'ok', { label: 'Aufgaben', run: () => CC.run('jobs') });
  }

  const file = MENUS.find(([n]) => n === 'Datei')[1];
  file.splice(file.indexOf('upload') + 1, 0, 'fetchUrl');
})();
