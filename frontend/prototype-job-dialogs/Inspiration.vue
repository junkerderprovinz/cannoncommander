<script setup lang="ts">
// Prototype: how other file managers show a running copy, drawn by hand for comparison on this page only.
import { G } from '@/core/glyphs';
</script>

<template>
  <section class="insp" aria-labelledby="insp-title">
    <h2 id="insp-title">Zum Vergleich: Kopierfenster anderer Dateimanager</h2>
    <p class="insp-lead">Von Hand nachgebaut, mit denselben Zahlen wie oben. E und F nehmen das Beste davon in unseren Stil.</p>
    <div class="insp-grid">
      <figure class="insp-item">
        <div class="insp-stage insp-stage--light">
          <div class="mx">
            <div class="mx-bar"><i></i><i></i><i></i><span>Kopieren</span></div>
            <div class="mx-body">
              <div class="mx-icon"><G n="IconFolder" /></div>
              <div class="mx-main">
                <p>„Urlaub 2026“ wird nach „fotos“ kopiert</p>
                <div class="mx-row"><span class="mx-track"><span style="width: 43%"></span></span><button aria-label="Stoppen"><G n="IconClose" /></button></div>
                <p class="mx-sub">1,8 GB von 4,2 GB · Noch etwa 22 Sekunden</p>
              </div>
            </div>
          </div>
        </div>
        <figcaption><b>macOS Finder</b>Ein schlichtes Blatt: ein Satz, ein dünner Balken und daneben nur ein kleiner Stopp-Knopf. Keine Fußzeile, nichts lenkt ab.</figcaption>
      </figure>

      <figure class="insp-item">
        <div class="insp-stage insp-stage--light">
          <div class="wx">
            <div class="wx-title">43 % abgeschlossen</div>
            <div class="wx-body">
              <p>Kopiere 12 Elemente von <a>fotos</a> nach <a>fotos</a></p>
              <div class="wx-head"><span>43 % abgeschlossen</span><span class="wx-btns"><button aria-label="Anhalten"><G n="IconPause" /></button><button aria-label="Abbrechen"><G n="IconClose" /></button></span></div>
              <div class="wx-graph">
                <svg viewBox="0 0 100 40" preserveAspectRatio="none" aria-hidden="true">
                  <path d="M0,40 L0,14 L4,12 L8,13 L12,11 L16,12 L20,10 L24,13 L28,15 L32,14 L36,12 L40,11 L43,12 L43,40 Z" />
                </svg>
                <span class="wx-speed" style="top: 30%">Geschwindigkeit: 112 MB/s</span>
              </div>
              <dl class="wx-facts">
                <div><dt>Name:</dt><dd>IMG_20260812_1430.jpg</dd></div>
                <div><dt>Verbleibende Zeit:</dt><dd>Etwa 22 Sekunden</dd></div>
                <div><dt>Verbleibende Elemente:</dt><dd>7 (2,4 GB)</dd></div>
              </dl>
              <div class="wx-less"><G n="IconExpand" />Weniger Details</div>
            </div>
          </div>
        </div>
        <figcaption><b>Windows 11 Explorer</b>Aufgeklappt zeigt ein Graph das Tempo über den ganzen Verlauf, mit einer Linie für das aktuelle Tempo. Zugeklappt bleibt nur der Balken.</figcaption>
      </figure>

      <figure class="insp-item">
        <div class="insp-stage insp-stage--light">
          <div class="gx">
            <div class="gx-header"><span class="gx-pie" style="--p: 43%"></span></div>
            <div class="gx-pop">
              <div class="gx-row">
                <div class="gx-main">
                  <p>Kopiere „Urlaub 2026“ nach „fotos“</p>
                  <p class="gx-sub">1,8 von 4,2 GB · noch 22 Sekunden</p>
                  <span class="gx-track"><span style="width: 43%"></span></span>
                </div>
                <button aria-label="Abbrechen"><G n="IconClose" /></button>
              </div>
            </div>
          </div>
        </div>
        <figcaption><b>GNOME Dateien</b>Kein eigenes Fenster: Die Vorgänge stecken in einem kleinen Kreis oben in der Leiste, ein Klick zeigt sie. So stört Kopieren nie.</figcaption>
      </figure>

      <figure class="insp-item">
        <div class="insp-stage insp-stage--dark">
          <div class="kx">
            <div class="kx-top"><span class="kx-app"><G n="IconFolder" /></span><span>Dateiverwaltung</span><span class="kx-time">Gerade eben</span></div>
            <p class="kx-title">Kopieren · 43 %</p>
            <p class="kx-what">Urlaub 2026 <span>nach fotos</span></p>
            <div class="kx-row"><span class="kx-track"><span style="width: 43%"></span></span><span class="kx-pct">43 %</span></div>
            <p class="kx-sub">1,8 GB von 4,2 GB (112 MB/s)</p>
            <div class="kx-actions"><button><G n="IconExpand" />Details</button><span></span><button aria-label="Anhalten"><G n="IconPause" /></button><button aria-label="Abbrechen"><G n="IconStop" /></button></div>
          </div>
        </div>
        <figcaption><b>KDE Dolphin</b>Läuft als Meldung im System weiter, mit Anhalten und Abbrechen direkt daneben. Unter Details stehen Tempo und Restgröße.</figcaption>
      </figure>
    </div>
  </section>
</template>
