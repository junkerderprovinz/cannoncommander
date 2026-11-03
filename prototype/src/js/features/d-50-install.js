// Als App installieren: the page is a single file, so its manifest and icon are built here as data URLs.
(() => {
  // The folder glyph in dark ink on the accent colour, drawn on a full square so a round mask can crop it.
  const iconSvg = (bg) => {
    const d = GLYPHS.IconFolderOpen;
    return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512"><rect width="512" height="512" rx="112" fill="${bg}"/><svg x="116" y="116" width="280" height="280" viewBox="${d.viewBox}" fill="#161616" color="#161616">${d.svg}</svg></svg>`;
  };
  const dataUrl = (type, text) => `data:${type},${encodeURIComponent(text)}`;
  const here = new URL('.', location.href).href;
  const icon = dataUrl('image/svg+xml', iconSvg(CC.prefs.accent));

  const manifest = {
    name: 'CastleCommander',
    short_name: 'Castle',
    description: 'Dateimanager für deinen Unraid-Server, mit zwei Fenstern wie Krusader',
    lang: 'de',
    start_url: location.href.split('#')[0],
    scope: here,
    display: 'standalone',
    background_color: '#161616',
    theme_color: CC.prefs.accent,
    icons: [
      { src: icon, sizes: 'any', type: 'image/svg+xml', purpose: 'any' },
      { src: icon, sizes: 'any', type: 'image/svg+xml', purpose: 'maskable' },
    ],
    // Files shared from another app on the phone land in the upload; the prototype has no server to take them.
    share_target: {
      action: new URL('teilen', here).href,
      method: 'POST',
      enctype: 'multipart/form-data',
      params: { title: 'titel', text: 'text', url: 'adresse', files: [{ name: 'dateien', accept: ['*/*'] }] },
    },
  };
  document.head.insertAdjacentHTML('beforeend', `<link rel="manifest" href="${dataUrl('application/manifest+json', JSON.stringify(manifest))}">
    <link rel="icon" href="${icon}"><link rel="apple-touch-icon" href="${icon}"><meta name="theme-color" content="${CC.prefs.accent}">`);

  let offer = null;
  addEventListener('beforeinstallprompt', e => { e.preventDefault(); offer = e; });
  addEventListener('appinstalled', () => { offer = null; CC.toast('CastleCommander ist jetzt als App installiert'); });
  const standalone = () => matchMedia('(display-mode: standalone)').matches || navigator.standalone === true;

  const WAYS = [
    ['chrome', 'IconBrowser', 'Chrome und Edge am Computer', 'In der Adressleiste rechts auf das Installieren-Symbol klicken, oder im Browsermenü App installieren wählen.'],
    ['android', 'IconPhone', 'Android', 'Im Browsermenü App installieren oder Zum Startbildschirm hinzufügen antippen.'],
    ['ios', 'IconPhone', 'iPhone und iPad', 'In Safari auf Teilen tippen, dann Zum Home-Bildschirm.'],
    ['firefox', 'IconBrowser', 'Firefox', 'Auf Android im Menü Installieren antippen. Unter Windows bietet Firefox dafür ein Symbol in der Adressleiste an, sonst tut es ein Lesezeichen.'],
  ];
  const mine = () => {
    const ua = navigator.userAgent;
    return /iPhone|iPad|iPod/.test(ua) ? 'ios' : /Firefox/.test(ua) ? 'firefox' : /Android/.test(ua) ? 'android' : 'chrome';
  };

  CC.action('installApp', { label: 'Als App installieren', g: 'IconInstall', run: () => {
    if (standalone()) return CC.toast('CastleCommander läuft schon als App');
    if (offer) {
      offer.prompt();
      offer.userChoice.then(c => { if (c.outcome !== 'accepted') CC.toast('Nicht installiert. Über Hilfe geht es jederzeit wieder.'); });
      offer = null;
      return;
    }
    const at = mine();
    win({
      title: 'Als App installieren',
      body: `<div class="well"><div class="in-head"><span class="in-icon"><img src="${icon}" alt=""></span><div><b>CastleCommander als App</b><div class="muted">Eigenes Fenster ohne Adressleiste, ein Symbol auf dem Startbildschirm oder in der Taskleiste. Auf dem Handy kannst du Dateien aus anderen Apps direkt hierher teilen.</div></div></div></div>
        <div class="well"><div class="eyebrow">So geht es in deinem Browser</div>
          ${WAYS.map(([id, gl, head, how]) => `<div class="in-way ${id === at ? 'cur' : ''}">${g(gl)}<div><b>${head}</b>${id === at ? '<span class="pill">dein Browser</span>' : ''}<div class="muted">${how}</div></div></div>`).join('')}</div>`,
      foot: [{ start: 'Dein Browser bietet die Installation gerade nicht selbst an' }, closeBtn],
    });
  } });
  const help = MENUS.find(([m]) => m === 'Hilfe')[1];
  help.splice(help.indexOf('docs') + 1, 0, 'installApp');

  CC.css(`
.in-head { display: flex; gap: 1rem; align-items: center; }
.in-head .muted { font-size: 13px; margin-top: .2rem; }
.in-icon { width: 3.5rem; height: 3.5rem; flex: none; }
.in-icon img { width: 100%; height: 100%; display: block; }
.in-way { display: flex; gap: .8rem; align-items: flex-start; padding: .55rem .6rem; border-radius: var(--radius-control); }
.in-way > svg { width: 1.2rem; height: 1.2rem; flex: none; margin-top: .1rem; color: var(--accent-glyph); }
.in-way .muted { font-size: 13px; margin-top: .15rem; }
.in-way .pill { margin-left: .5rem; }
.in-way.cur { background: var(--carbon-surface3); }
`);
})();
