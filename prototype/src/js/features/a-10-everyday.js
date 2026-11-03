// The everyday settings tabs: Allgemein, Aussehen and Dateiliste.
(() => {
  const A = CC.admin;

  CC.settingsTab({ id: 'general', label: 'Allgemein', g: 'IconTabGeneral', order: 10, render(host) {
    const p = CC.prefs;
    const days = p.trashDays || 30;
    host.innerHTML = CC.card('Bedienung',
      frow('Startordner', A.pathField(p.startFolder || '/mnt/user/downloads', 'a-start'), 'Dieser Ordner öffnet sich nach dem Anmelden im linken Fenster.')
      + frow('Doppelklick auf eine Datei', seg([['open', 'Öffnen'], ['preview', 'Vorschau'], ['download', 'Herunterladen']], p.dblclick, 'data-pref="dblclick"'), 'Ein Doppelklick auf einen Ordner öffnet ihn immer.'))
      + CC.card('Löschen',
        frow('Vor dem Löschen fragen', sw(p.confirmDelete, 'data-pref="confirmDelete"'), 'Zeigt vor dem Löschen, was alles weggeht.')
        + frow('Papierkorb verwenden', sw(p.trash, 'data-pref="trash"'), 'Gelöschtes landet erst im Papierkorb und lässt sich von dort zurückholen. Ohne Papierkorb ist es sofort weg.')
        + (p.trash ? frow('Aufbewahren', seg([[7, '7 Tage'], [14, '14 Tage'], [30, '30 Tage'], [90, '90 Tage']], days, 'data-pref="trashDays" data-num="1"'), 'Danach leert sich der Papierkorb von selbst.') : ''))
      + CC.card('Aufgaben',
        frow('Mehrere Aufgaben', seg([['serial', 'Nacheinander'], ['parallel', 'Gleichzeitig']], p.jobMode || 'serial', 'data-pref="jobMode"'), 'Nacheinander schont die Platten: eine Aufgabe läuft, die anderen warten in der Reihenfolge der Liste. Gleichzeitig teilen sich alle die Bandbreite.'));
    A.bindPrefs(host);
    $('#a-start', host).addEventListener('change', e => {
      const v = e.target.value.trim(), n = FS.nodeAt(v);
      if (!n || !n.dir) return A.fail('Diesen Ordner gibt es nicht', e.target);
      A.setPref('startFolder', v);
      CC.toast('Startordner gespeichert');
    });
  } });

  const ACCENTS = [['Sunflower', '#FCC419'], ['Blue', '#1D99F3'], ['Green', '#6FDC8C'], ['Red', '#FF8389'], ['Purple', '#BE95FF'], ['Orange', '#FF832B'], ['Teal', '#3DDBD9'], ['Magenta', '#FF7EB6']];
  const dist = (a, b) => [16, 8, 0].reduce((s, sh) => s + (((parseInt(a.slice(1), 16) >> sh) & 255) - ((parseInt(b.slice(1), 16) >> sh) & 255)) ** 2, 0);
  let leafFound = false, stormFound = false;

  function swatches() {
    const cur = CC.prefs.accent.toUpperCase();
    const exact = ACCENTS.findIndex(a => a[1] === cur);
    const slot = exact >= 0 ? exact : ACCENTS.reduce((best, a, i) => dist(a[1], cur) < dist(ACCENTS[best][1], cur) ? i : best, 0);
    return `<div class="a-swatches" data-k-focus="accent">${ACCENTS.map(([name, hex], i) => {
      const on = i === slot, col = on ? cur : hex, label = on && exact < 0 ? cur : name;
      return `<button class="a-swatch" style="background:${col}" aria-pressed="${on}" data-hex="${hex}" data-tip="${esc(on ? label + ', noch einmal klicken zum Anpassen' : label)}" aria-label="${esc(label)}"></button>`;
    }).join('')}</div>`;
  }

  CC.settingsTab({ id: 'look', label: 'Aussehen', g: 'IconTabLook', order: 20, render(host) {
    const p = CC.prefs;
    const shapes = [['round', 'Rund', 'IconCornerRound'], ['soft', 'Abgerundet', 'IconCornerSoft'], ['square', 'Eckig', 'IconCornerSquare']];
    if (p.shape === 'leaf' || leafFound) shapes.push(['leaf', 'Blatt', 'IconCornerLeaf']);
    const motions = [['off', 'Aus', 'IconMotionOff'], ['subtle', 'Dezent', 'IconMotionSubtle'], ['wild', 'Wild', 'IconMotionWild']];
    if (p.motion === 'storm' || stormFound) motions.push(['storm', 'Sturm', 'IconMotionStorm']);
    const labelModes = [['text', 'Text', 'IconLabelText'], ['textGlyph', 'Text und Symbol', 'IconLabelTextGlyph'], ['glyph', 'Symbol', 'IconLabelGlyph'], ['reactive', 'Reaktiv', 'IconLabelReactive']];
    const ordered = (CC.prefs.tabOrder || []).length > 0;
    host.innerHTML = CC.card('Darstellung',
      frow('Sprache', A.dropdown([['de', '🇩🇪 Deutsch'], ['en', '🇬🇧 English']], p.language, 'data-pref="language"'))
      + frow('Design', seg([['dark', 'Dunkel', 'IconMoon'], ['light', 'Hell', 'IconSun'], ['system', 'System', 'IconThemeSystem']], p.theme, 'data-pref="theme"'), 'System folgt der Einstellung deines Geräts.')
      + frow('Ecken', seg(shapes, p.shape, 'data-k-focus="shape" id="a-shape"'), 'Wie rund Karten, Knöpfe und Fenster sind.')
      + frow('Animation', seg(motions, p.motion, 'data-k-focus="motion" id="a-motion"'), 'Wie lebhaft Fenster und Seiten hereinkommen. Wenn dein System weniger Bewegung wünscht, gilt das immer.')
      + frow('Leise Benachrichtigungen', sw(p.quietToasts, 'data-pref="quietToasts"'), 'Zeigt nur noch Fehler und Hinweise, bei denen du etwas tun kannst. Routinemeldungen wie „Kopiert“ bleiben weg.')
      + frow('Reihenfolge der Einstellungsseiten', `<button class="btn" id="a-order-reset" ${ordered ? '' : 'disabled'}>${g('IconRefresh')}Zurücksetzen</button>`, 'Die Kacheln links lassen sich mit der Maus an eine andere Stelle ziehen.'))
      + CC.card('Beschriftungen',
        frow('Buttons', seg(labelModes, p.labelsButtons, 'data-pref="labelsButtons"'), 'Wie viel von einem Bedienelement gezeigt wird: seine Wörter, sein Symbol oder beides. Reaktiv zeigt nur das Symbol und blendet die Wörter ein, sobald du darauf zeigst.')
        + frow('Seitenleiste', seg(labelModes, p.labelsSidebar, 'data-pref="labelsSidebar"'))
        + frow('Tabs', seg(labelModes, p.labelsTabs, 'data-pref="labelsTabs"'), 'Die Kacheln der Einstellungen und alle Wähler.')
        + frow('Untere Leiste', seg(labelModes, p.labelsBottombar, 'data-pref="labelsBottombar"'), 'Nur auf dem Handy.'))
      + CC.card('Farben',
        frow('Akzentfarbe', swatches(), 'Ein Klick wählt eine Farbe. Ein zweiter Klick auf die gewählte öffnet den Farbwähler für einen eigenen Ton.')
        + frow('Regenbogen-Modus', sw(p.rainbow, 'data-pref="rainbow"'), 'Jede Stelle in einer Liste bekommt ihre eigene Farbe aus der Palette.')
        + frow('Reaktiver Modus', sw(p.reactive, 'data-pref="reactive"'), 'Symbole bleiben grau und färben sich erst, wenn du darauf zeigst.')
        + frow('Farbenrotation', sw(p.rotate, `data-pref="rotate" ${p.rainbow ? '' : 'disabled'}`), 'Die Farben des Regenbogens wandern langsam weiter. Nur mit Regenbogen-Modus.'));
    A.bindPrefs(host);
    $('#a-order-reset', host)?.addEventListener('click', () => { A.setPref('tabOrder', []); CC.toast('Reihenfolge zurückgesetzt'); });
    host.addEventListener('change', e => {
      if (e.target.closest('[data-pref="language"]') && CC.prefs.language === 'en') CC.toast('Gespeichert. Die englische Oberfläche kommt in einer späteren Runde');
      if (e.target.id === 'a-shape') {
        let v = segVal(e.target);
        if (A.tapCount('shape', v, 'square') >= 5) { v = 'leaf'; leafFound = true; CC.toast('Gefunden: Blatt'); }
        A.setPref('shape', v);
      }
      if (e.target.id === 'a-motion') {
        let v = segVal(e.target);
        if (A.tapCount('motion', v, 'wild') >= 5) { v = 'storm'; stormFound = true; CC.toast('Gefunden: Sturm'); }
        A.setPref('motion', v);
      }
    });
    host.addEventListener('click', e => {
      const s = e.target.closest('.a-swatch');
      if (!s) return;
      if (s.getAttribute('aria-pressed') === 'true') {
        const live = (hex) => {
          document.documentElement.style.setProperty('--accent', hex);
          document.documentElement.style.setProperty('--accent-contrast', A.ink(hex));
          const on = $('.a-swatch[aria-pressed="true"]');
          if (on) on.style.background = hex;
        };
        A.colorPicker(s, CC.prefs.accent, live, (hex) => A.setPref('accent', hex));
      } else A.setPref('accent', s.dataset.hex);
    });
  } });

  CC.settingsTab({ id: 'files', label: 'Dateiliste', g: 'IconFiles', order: 30, render(host) {
    const p = CC.prefs;
    const sample = A.now() - 2 * DAY;
    host.innerHTML = CC.card('Liste',
      frow('Versteckte Dateien zeigen', sw(p.showHidden, 'data-pref="showHidden"'), 'Dateien, deren Name mit einem Punkt beginnt, etwa .DS_Store. Strg+H schaltet sie auch in der Liste um.')
      + frow('Ordnergrößen berechnen', sw(p.folderSizes, 'data-pref="folderSizes"'), 'Zählt alles in einem Ordner zusammen. Ohne den Schalter steht dort nur, wie viele Objekte er enthält.')
      + frow('Ordner zuerst', sw(p.foldersFirst !== false, 'data-pref="foldersFirst"'), 'Ordner stehen über den Dateien, egal wonach sortiert wird.')
      + frow('Datum', seg([['short', fmtDate(sample)], ['long', fmtTime(sample)], ['relative', A.ago(sample)]], p.dateFormat, 'data-pref="dateFormat"'))
      + frow('Zeilenhöhe', seg([['compact', 'Kompakt', 'IconRowCompact'], ['medium', 'Mittel', 'IconRowMedium'], ['comfortable', 'Bequem', 'IconRowComfortable']], p.density, 'data-pref="density"'))
      + frow('Spalten', `<button class="btn" data-run="columns">${g('IconSliders')}Spalten wählen</button>`, 'Spalten lassen sich auch direkt im Spaltenkopf verschieben, und ein Rechtsklick auf den Kopf blendet sie ein und aus.'))
      + CC.card('Leisten',
        frow('Seitenleiste', sw(p.side, 'data-pref="side"'), 'Verknüpfungen und Ordnerbaum links neben den Fenstern.')
        + frow('Funktionstastenleiste', sw(p.fkeys, 'data-pref="fkeys"'), 'Die Knöpfe für F3 bis F8 am unteren Rand, wie im Total Commander.')
        + frow('Vorschauleiste', sw(p.quickview, 'id="a-qv" data-k-focus="qv"'), 'Zeigt im zweiten Fenster eine Vorschau der Datei, auf der der Cursor steht. Schaltet dafür die zwei Fenster ein.'));
    A.bindPrefs(host);
    $('#a-qv', host).addEventListener('change', e => {
      const on = swVal(e.target);
      if (on) CC.prefs.dual = true;
      A.setPref('quickview', on);
    });
  } });

  CC.css(`
.a-swatches { display: flex; gap: .45rem; flex-wrap: wrap; justify-content: flex-end; }
.a-swatch { width: 1.6rem; height: 1.6rem; border-radius: var(--radius-pill); flex: none; box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--carbon-text) 12%, transparent); }
.a-swatch:hover { transform: scale(1.08); }
.a-swatch[aria-pressed="true"] { box-shadow: 0 0 0 2px var(--carbon-surface), 0 0 0 4px var(--carbon-text); }
`);
})();
