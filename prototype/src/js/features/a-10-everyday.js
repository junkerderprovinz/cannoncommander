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

  // The accent presets are the rainbow's eight colours, so the accent and the palette offer the same swatches.
  const ACCENT = '#FCC419';
  const PRESETS = ['#FCC419', '#1D99F3', '#6FDC8C', '#FF8389', '#BE95FF', '#FF832B', '#3DDBD9', '#FF7EB6'];
  // Sunflower sits third so one row of a rainbow list always matches the default accent.
  const RAINBOW = ['#FF8389', '#FF832B', '#FCC419', '#6FDC8C', '#3DDBD9', '#1D99F3', '#BE95FF', '#FF7EB6'];
  const same = (a, b) => a.length === b.length && a.every((x, i) => x.toLowerCase() === b[i].toLowerCase());
  const valid = (list) => Array.isArray(list) && list.length === 8 && list.every(h => /^#[0-9a-f]{6}$/i.test(h));
  if (!valid(CC.prefs.accentPresets)) CC.prefs.accentPresets = PRESETS.slice();
  if (!valid(CC.prefs.palette)) CC.prefs.palette = RAINBOW.slice();

  // Three hidden modes, each opened by repeating one gesture five times. Whether one was found lives here and
  // not in the prefs, so it is offered only while it is chosen or until the page is reloaded.
  const found = { leaf: false, storm: false, disco: false };
  const taps = { leaf: 0, storm: 0, disco: 0, discoLast: 0 };
  // Five more clicks on the level already chosen; any other click starts over.
  const tap = (key, clicked, current, top) => {
    if (clicked !== top || current !== top) { taps[key] = 0; return false; }
    if (++taps[key] < 5) return false;
    taps[key] = 0;
    return true;
  };
  // Five turn-ons of the rainbow, each within three seconds of the last, so comparing it on and off now and then
  // never opens disco by accident. Only turn-ons count, so the gesture ends with the rainbow on.
  const discoTap = (on) => {
    if (!on) return false;
    const now = Date.now(), gap = now - taps.discoLast;
    taps.discoLast = now;
    taps.disco = taps.disco > 0 && gap <= 3000 ? taps.disco + 1 : 1;
    if (taps.disco < 5) return false;
    taps.disco = 0;
    return true;
  };

  // The palette lands on the root as --rb-0 to --rb-7; rotation only moves which colour counts as the first.
  const rgb = (hex) => [1, 3, 5].map(i => parseInt(hex.slice(i, i + 2), 16));
  const mix = (a, b, t) => '#' + rgb(a).map((c, i) => Math.round(c + (rgb(b)[i] - c) * t).toString(16).padStart(2, '0')).join('');
  const paintPalette = (colourAt) => {
    const r = document.documentElement;
    for (let i = 0; i < 8; i++) {
      const c = colourAt(i);
      r.style.setProperty(`--rb-${i}`, c);
      r.style.setProperty(`--rb-ink-${i}`, A.ink(c));
    }
  };
  const seed = () => CC.prefs.rotate ? CC.prefs.rotateSeed || 0 : 0;
  const restPalette = () => paintPalette(i => CC.prefs.palette[(i + seed()) % 8]);

  // Disco walks the palette: one colour's worth every 2.4 seconds, gliding, or stepping when nothing may move.
  let frame = null, last, travelled = 0;
  const walk = (now) => {
    travelled = (travelled + (now - (last ?? now)) / (2400 * 8)) % 1;
    last = now;
    const steps = CC.prefs.motion === 'off' || matchMedia('(prefers-reduced-motion: reduce)').matches;
    const p = CC.prefs.palette, at = travelled * 8;
    paintPalette(i => {
      const k = i + seed() + at, lo = Math.floor(k);
      return steps ? p[lo % 8] : mix(p[lo % 8], p[(lo + 1) % 8], k - lo);
    });
    frame = requestAnimationFrame(walk);
  };
  const disco = () => {
    const on = CC.prefs.disco && CC.prefs.rainbow;
    document.documentElement.toggleAttribute('data-disco', !!CC.prefs.disco);
    if (on && frame === null) { travelled = 0; last = undefined; frame = requestAnimationFrame(walk); }
    if (!on && frame !== null) { cancelAnimationFrame(frame); frame = null; }
    if (!on) restPalette();
  };
  const baseApply = CC.applyPrefs;
  CC.applyPrefs = () => {
    baseApply();
    const r = document.documentElement;
    // Reactive is a way of showing the rainbow, so it has no effect without it, and rotation is an offset, not a spin.
    r.toggleAttribute('data-reactive', CC.prefs.rainbow && CC.prefs.reactive);
    r.removeAttribute('data-rotate');
    disco();
  };
  CC.applyPrefs();
  CC.css(`${[0, 1, 2, 3, 4, 5, 6, 7].map(i => `:root[data-rainbow] :is(#shortcuts, #tree, .tiles, .list) > :nth-child(8n+${i + 1}) { --accent-glyph: var(--rb-${i}); }`).join('\n')}`);

  const disc = (hex, attr, label) => `<button class="a-swatch" style="--c:${hex}" ${attr} data-tip="${esc(label)}" aria-label="${esc(label)}"></button>`;
  const resetBadge = (id, tip, off) => `<button class="a-reset" id="${id}" data-tip="${esc(tip)}" aria-label="${esc(tip)}" ${off ? 'disabled' : ''}>${g('IconResetArrow')}</button>`;

  function coloursCard() {
    const p = CC.prefs;
    const accentDefault = p.accent.toLowerCase() === ACCENT.toLowerCase() && same(p.accentPresets, PRESETS);
    // While the rainbow owns the colours the accent row stays visible but dimmed, and its bubble says how to get it back.
    const accentRow = `<div class="frow a-accent ${p.rainbow ? 'dim' : ''}"><span class="lab"><span class="a-dimmable">Akzentfarbe</span>${p.rainbow ? ' ' + info('Der Regenbogen-Modus ist an. Schalte ihn aus, um wieder eine Akzentfarbe zu wählen.') : ''}</span>
      <div class="a-swatches a-dimmable" data-k-focus="accent">${p.accentPresets.map((hex, i) => disc(hex, `data-preset="${i}" aria-pressed="${hex.toLowerCase() === p.accent.toLowerCase()}" ${p.rainbow ? 'disabled' : ''}`, `Voreinstellung ${i + 1}`)).join('')}
        ${resetBadge('a-accent-reset', 'Akzentfarbe und Voreinstellungen zurücksetzen', accentDefault || p.rainbow)}</div></div>`;
    const rainbow = frow('Regenbogen-Modus', sw(p.rainbow, 'id="a-rainbow"'), 'Jede Zeile in einer Liste bekommt eine eigene Farbe aus einer festen Auswahl von acht, statt dass alles dieselbe Akzentfarbe hat. Das macht lange Listen auf einen Blick leichter unterscheidbar.');
    // Everything under the rainbow only shows while it is on: a palette editor under a rainbow that is not running offers edits with no effect.
    const under = !p.rainbow ? '' : (found.disco || p.disco ? frow('Disco-Modus', sw(p.disco, 'data-pref="disco"'), 'Jede farbige Zeile und jedes farbige Element gleitet langsam zur nächsten Farbe der Palette, solange der Regenbogen-Modus aktiv ist.') : '')
      + frow('Reaktiver Modus', sw(p.reactive, 'data-pref="reactive"'), 'Wenn aktiv, bleiben farbige Zeilen und Elemente neutral, bis du sie mit der Maus berührst oder sie gerade laufen oder ausgewählt sind. Die Farbe erscheint also nur bei Bedarf. Wenn deaktiviert, zeigen alle farbigen Zeilen und Elemente ihre Farbe durchgehend.')
      + frow('Farbenrotation', sw(p.rotate, 'id="a-rotate"'), 'Verschiebt, welche Farbe der Palette als Position 0 gilt, damit dieselbe Liste nicht bei jedem Aktivieren des Regenbogen-Modus oder jedem Neuladen der Seite mit genau derselben Farbe beginnt.')
      + `<div class="frow"><span class="lab">Farbpalette</span><div class="a-swatches">${p.palette.map((hex, i) => disc(hex, `data-palette="${i}"`, `Palettenfarbe ${i + 1}`)).join('')}
        ${resetBadge('a-palette-reset', 'Farbpalette zurücksetzen', same(p.palette, RAINBOW))}</div></div>`;
    return CC.card('Farben', accentRow + rainbow + under);
  }

  CC.settingsTab({ id: 'look', label: 'Aussehen', g: 'IconTabLook', order: 20, render(host) {
    const p = CC.prefs;
    const shapes = [['round', 'Rund', 'IconCornerRound'], ['soft', 'Abgerundet', 'IconCornerSoft'], ['square', 'Eckig', 'IconCornerSquare']];
    if (p.shape === 'leaf' || found.leaf) shapes.push(['leaf', 'Blatt', 'IconCornerLeaf']);
    const motions = [['off', 'Aus', 'IconMotionOff'], ['subtle', 'Dezent', 'IconMotionSubtle'], ['wild', 'Wild', 'IconMotionWild']];
    if (p.motion === 'storm' || found.storm) motions.push(['storm', 'Sturm', 'IconMotionStorm']);
    const labelModes = [['text', 'Text', 'IconLabelText'], ['textGlyph', 'Text und Symbol', 'IconLabelTextGlyph'], ['glyph', 'Symbol', 'IconLabelGlyph'], ['reactive', 'Reaktiv', 'IconLabelReactive']];
    const ordered = (CC.prefs.tabOrder || []).length > 0;
    host.innerHTML = CC.card('Darstellung',
      frow('Sprache', A.dropdown([['de', 'Deutsch', null, 'de'], ['en', 'English', null, 'gb']], p.language, 'data-pref="language"'))
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
      + coloursCard();
    A.bindPrefs(host);
    $('#a-order-reset', host)?.addEventListener('click', () => { A.setPref('tabOrder', []); CC.toast('Reihenfolge zurückgesetzt'); });
    host.addEventListener('change', e => {
      if (e.target.closest('[data-pref="language"]') && CC.prefs.language === 'en') CC.toast('Gespeichert. Die englische Oberfläche kommt in einer späteren Runde');
      // The gesture is read before the choice is saved, because it counts clicks on the level already chosen.
      if (e.target.id === 'a-shape') {
        let v = segVal(e.target);
        if (tap('leaf', v, CC.prefs.shape, 'square')) { v = 'leaf'; found.leaf = true; }
        A.setPref('shape', v);
      }
      if (e.target.id === 'a-motion') {
        let v = segVal(e.target);
        if (tap('storm', v, CC.prefs.motion, 'wild')) { v = 'storm'; found.storm = true; }
        A.setPref('motion', v);
      }
      if (e.target.id === 'a-rainbow') {
        const on = swVal(e.target);
        if (discoTap(on)) found.disco = true;
        A.setPref('rainbow', on);
      }
      // Turning rotation on draws a fresh offset at once, so the switch does something visible.
      if (e.target.id === 'a-rotate') {
        const on = swVal(e.target);
        CC.prefs.rotateSeed = on ? 1 + Math.floor(Math.random() * 7) : 0;
        A.setPref('rotate', on);
      }
    });
    host.addEventListener('click', e => {
      const preset = e.target.closest('[data-preset]'), slot = e.target.closest('[data-palette]');
      // A preset is chosen by its click and edited in the picker that opens with it, so a custom colour is an edited preset.
      if (preset) {
        const i = +preset.dataset.preset;
        A.setPref('accent', CC.prefs.accentPresets[i]);
        const el = $(`[data-preset="${i}"]`);
        A.colorPicker(el, CC.prefs.accentPresets[i], (hex) => {
          el.style.setProperty('--c', hex);
          document.documentElement.style.setProperty('--accent', hex);
          document.documentElement.style.setProperty('--accent-contrast', A.ink(hex));
        }, (hex) => { CC.prefs.accentPresets[i] = hex; A.setPref('accent', hex); });
      }
      if (slot) {
        const i = +slot.dataset.palette;
        A.colorPicker(slot, CC.prefs.palette[i], (hex) => {
          slot.style.setProperty('--c', hex);
          document.documentElement.style.setProperty(`--rb-${(i - seed() + 8) % 8}`, hex);
        }, (hex) => { const next = CC.prefs.palette.slice(); next[i] = hex; A.setPref('palette', next); });
      }
      if (e.target.closest('#a-accent-reset')) { CC.prefs.accentPresets = PRESETS.slice(); A.setPref('accent', ACCENT); }
      if (e.target.closest('#a-palette-reset')) A.setPref('palette', RAINBOW.slice());
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
.a-swatches { display: flex; align-items: center; gap: .5rem; flex-wrap: wrap; justify-content: flex-end; }
/* A 28px disc in a 2px ring is 32px, the size of the reset badge beside it. */
.a-swatch { width: 2rem; height: 2rem; border-radius: var(--radius-pill); flex: none; background: var(--c); border: 2px solid var(--carbon-border, color-mix(in srgb, var(--carbon-text) 16%, transparent)); background-clip: padding-box; transition: transform .12s; }
.a-swatch:hover:not(:disabled) { transform: scale(1.1); }
.a-swatch[aria-pressed="true"] { border-color: var(--carbon-text); }
.a-swatch:disabled { cursor: default; }
.a-accent.dim .a-dimmable { opacity: .45; }
/* Neutral, so a reset beside the swatches never reads as one more colour to pick. */
.a-reset { width: 2rem; height: 2rem; border-radius: var(--radius-control); display: grid; place-items: center; flex: none; background: var(--carbon-surface2); border: 2px solid var(--carbon-border, color-mix(in srgb, var(--carbon-text) 16%, transparent)); color: var(--carbon-text); }
.a-reset svg { width: 1rem; height: 1rem; }
.a-reset:hover:not(:disabled) { background: var(--carbon-surface3); }
.a-reset:disabled { opacity: .45; cursor: default; }
`);
})();
