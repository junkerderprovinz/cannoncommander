// Prototype: made-up jobs in every state, in German and English.
import { reactive } from 'vue';

export type Dialog = 'copy' | 'move' | 'delete';
export type State = 'confirm' | 'running' | 'paused' | 'conflict' | 'done' | 'errors' | 'cancelled';
export type Lang = 'de' | 'en';

export const DIALOGS: Dialog[] = ['copy', 'move', 'delete'];
export const STATES: State[] = ['confirm', 'running', 'paused', 'conflict', 'done', 'errors', 'cancelled'];

const T = {
  de: {
    dialog: { copy: 'Kopieren', move: 'Verschieben', delete: 'Löschen' },
    state: { confirm: 'Bestätigen', running: 'Läuft', paused: 'Angehalten', conflict: 'Konflikt', done: 'Fertig', errors: 'Fertig mit Fehlern', cancelled: 'Abgebrochen' },
    badge: { running: 'läuft', paused: 'angehalten', conflict: 'fragt nach', done: 'fertig', errors: 'mit Fehlern', cancelled: 'abgebrochen' },
    from: 'Von', to: 'Nach', more: 'Weitere Optionen', keepAttrs: 'Rechte und Zeitstempel übernehmen',
    keep: 'Fenster nach Abschluss offen lassen', background: 'Im Hintergrund', cancel: 'Abbrechen', pause: 'Anhalten', resume: 'Weiter', close: 'Schließen',
    retry: 'Fehlgeschlagene wiederholen', goOn: 'Fortsetzen', open: 'Ziel öffnen', toTrash: 'In den Papierkorb', trashToggle: 'In den Papierkorb legen',
    trashHint: 'Bleibt 30 Tage im Papierkorb und lässt sich wiederherstellen.', options: 'Optionen',
    moreDetails: 'Mehr Details', lessDetails: 'Weniger Details',
    replace: 'Ersetzen', skip: 'Überspringen', both: 'Beide behalten', forAll: 'Für alle übernehmen', taken: 'Gibt es im Ziel schon',
    newSide: 'Neu', oldSide: 'Im Ziel', newer: 'neuer', bigger: 'größer', smaller: 'kleiner', older: 'älter',
    changed: 'Geändert', speed: 'Tempo', left: 'Noch', of: 'von', files: 'Dateien', size: 'Größe', progress: 'Fortschritt', now: 'Gerade', after: 'nach',
    trash: 'Papierkorb', items: 'Objekte',
    sentence: { copy: 'kopieren nach', move: 'verschieben nach', delete: 'in den Papierkorb legen' },
    ask: { copy: 'Kopieren?', move: 'Verschieben?', delete: 'In den Papierkorb legen?' },
    doneLine: { copy: '4,2 GB in 38 s kopiert', move: '8,4 GB in 1 min 12 s verschoben', delete: '3 Objekte liegen im Papierkorb' },
    errorsLine: { copy: '10 von 12 Dateien kopiert, 2 fehlgeschlagen', move: '1 Datei nicht verschoben', delete: '2 von 3 Objekten im Papierkorb, 1 fehlgeschlagen' },
    cancelledLine: { copy: 'Nach 1,1 GB abgebrochen. Was schon kopiert ist, bleibt im Ziel.', move: 'Nach 2,3 GB abgebrochen. Die Quelle ist vollständig.', delete: 'Nach 1 von 3 Objekten abgebrochen.' },
    error: 'Kein Platz mehr auf dem Ziel',
    variant: { A: 'Wie jetzt', B: 'Kompakt in einer Spalte', C: 'Zwei Zonen: Weg oben, Fortschritt unten', D: 'Schlicht, Details auf Wunsch', E: 'Ruhig wie ein Mac-Blatt', F: 'Mit Graph wie unter Windows' },
    doneOf: 'abgeschlossen', remaining: 'Verbleibend', stop: 'Stoppen', objects: 'Objekte', into: 'nach',
    all: 'Alle', light: 'Hell', dark: 'Dunkel', oled: 'OLED',
    motion: { off: 'Aus', subtle: 'Ruhig', wild: 'Wild', storm: 'Sturm' }, motionLabel: 'Bewegung',
  },
  en: {
    dialog: { copy: 'Copy', move: 'Move', delete: 'Delete' },
    state: { confirm: 'Confirm', running: 'Running', paused: 'Paused', conflict: 'Conflict', done: 'Done', errors: 'Done with errors', cancelled: 'Cancelled' },
    badge: { running: 'running', paused: 'paused', conflict: 'asking', done: 'done', errors: 'with errors', cancelled: 'cancelled' },
    from: 'From', to: 'To', more: 'More options', keepAttrs: 'Keep owner, rights and times',
    keep: 'Keep this window open when done', background: 'In the background', cancel: 'Cancel', pause: 'Pause', resume: 'Resume', close: 'Close',
    retry: 'Retry the failed ones', goOn: 'Resume', open: 'Open target', toTrash: 'Move to trash', trashToggle: 'Move to the trash',
    trashHint: 'Stays in the trash for 30 days and can be restored.', options: 'Options',
    moreDetails: 'More details', lessDetails: 'Fewer details',
    replace: 'Replace', skip: 'Skip', both: 'Keep both', forAll: 'Use for all', taken: 'Already in the target',
    newSide: 'New', oldSide: 'In the target', newer: 'newer', bigger: 'larger', smaller: 'smaller', older: 'older',
    changed: 'Modified', speed: 'Speed', left: 'Left', of: 'of', files: 'Files', size: 'Size', progress: 'Progress', now: 'Now', after: 'after',
    trash: 'Trash', items: 'items',
    sentence: { copy: 'copy to', move: 'move to', delete: 'move to the trash' },
    ask: { copy: 'Copy?', move: 'Move?', delete: 'Move to the trash?' },
    doneLine: { copy: 'Copied 4.2 GB in 38 s', move: 'Moved 8.4 GB in 1 min 12 s', delete: '3 items are in the trash' },
    errorsLine: { copy: 'Copied 10 of 12 files, 2 failed', move: '1 file not moved', delete: '2 of 3 items in the trash, 1 failed' },
    cancelledLine: { copy: 'Cancelled after 1.1 GB. What is copied stays in the target.', move: 'Cancelled after 2.3 GB. The source is whole.', delete: 'Cancelled after 1 of 3 items.' },
    error: 'No room left on the target',
    variant: { A: 'As it is', B: 'Compact, one column', C: 'Two zones: route on top, progress below', D: 'Plain, details on demand', E: 'Calm like a Mac sheet', F: 'With a graph like on Windows' },
    doneOf: 'complete', remaining: 'Remaining', stop: 'Stop', objects: 'items', into: 'to',
    all: 'All', light: 'Light', dark: 'Dark', oled: 'OLED',
    motion: { off: 'Off', subtle: 'Calm', wild: 'Wild', storm: 'Storm' }, motionLabel: 'Motion',
  },
};
export const texts = (lang: Lang) => T[lang];
export type Texts = ReturnType<typeof texts>;

export interface Version { size: string; bytes: number; date: string; when: number }
export interface Clash { name: string; kind: 'image' | 'video' | 'folder'; fresh: Version; there: Version }

// What each dialog works on, the same in every variant. totalMB drives the made-up throughput.
export const JOBS = {
  copy: { name: 'Urlaub 2026', count: '12', from: '/mnt/user/fotos', to: '/mnt/user/backups/fotos', total: '4,2 GB', totalMB: 4300, glyph: 'IconCopy', speed: 112,
    run: { pct: 43, current: 'IMG_20260812_1430.jpg' },
    files: ['IMG_20260812_1430.jpg', 'IMG_20260812_1431.jpg', 'Strand bei Nacht.mp4', 'IMG_20260813_0902.heic', 'Reiseroute.pdf', 'IMG_20260813_1117.jpg', 'Tagebuch.txt', 'Rohdaten.zip'],
    failed: ['IMG_20260814_0912.jpg', 'Urlaub 2026.mp4'],
    clash: { name: 'IMG_20260812_1430.jpg', kind: 'image', fresh: { size: '2,1 MB', bytes: 2.1, date: '12.08.2026, 14:32', when: 2 }, there: { size: '2,0 MB', bytes: 2.0, date: '12.08.2026, 09:15', when: 1 } } as Clash },
  move: { name: 'Citizen Vigilante (2026).mkv', count: '1', from: '/mnt/user/downloads', to: '/mnt/user/medien/Filme', total: '8,4 GB', totalMB: 8600, glyph: 'IconMove', speed: 96,
    run: { pct: 27, current: 'Citizen Vigilante (2026).mkv' },
    files: ['Citizen Vigilante (2026).mkv'],
    failed: ['Citizen Vigilante (2026).mkv'],
    clash: { name: 'Citizen Vigilante (2026).mkv', kind: 'video', fresh: { size: '8,4 GB', bytes: 8.4, date: '03.11.2026, 21:40', when: 2 }, there: { size: '8,1 GB', bytes: 8.1, date: '28.10.2026, 18:05', when: 1 } } as Clash },
  delete: { name: '3 Objekte', count: '3', from: '/mnt/user/dokumente', to: '', total: '38 MB', totalMB: 38, glyph: 'IconTrash', speed: 0,
    run: { pct: 33, current: 'Steuer 2024' },
    files: ['Steuer 2024/', 'Notizen.txt', 'Belege 2025.ods'],
    failed: ['Belege 2025.ods'], items: ['Steuer 2024', 'Notizen.txt', 'Belege 2025.ods'],
    clash: { name: 'Steuer 2024', kind: 'folder', fresh: { size: '24 MB', bytes: 24, date: '20.11.2026, 10:12', when: 2 }, there: { size: '24 MB', bytes: 24, date: '02.04.2025, 16:48', when: 1 } } as Clash },
} as const;

const safe = <T>(fn: () => T, or: T): T => { try { return fn(); } catch { return or; } };

// The person's choice of the larger window, kept in this browser the way the app keeps it per user.
export const ui = reactive({
  details: safe(() => localStorage.getItem('cc-proto-job-details') === '1', false),
  tick: 0,
});
// The file the job is at, from a made-up stream that moves on every two seconds.
export const currentFile = (files: readonly string[]) => files[Math.floor(ui.tick / 2) % files.length]!;
export function setDetails(on: boolean) {
  ui.details = on;
  safe(() => localStorage.setItem('cc-proto-job-details', on ? '1' : '0'), undefined);
}
setInterval(() => { ui.tick++; }, 1000);

// A made-up but steady throughput: a base rate with slow swells and a little grain, the same on every load.
export function speedAt(base: number, x: number) {
  const grain = Math.sin(x * 12.9898) * 43758.5453;
  return base * (1 + 0.09 * Math.sin(x / 4.2) + 0.035 * Math.sin(x / 1.6 + 1) + 0.015 * (grain - Math.floor(grain) - 0.5));
}

export interface Sample { x: number; mbs: number; s: number }
// One sample per half percent up to pct, each with the time it took to get there.
export function samplesTo(base: number, totalMB: number, pct: number): Sample[] {
  const out: Sample[] = [];
  let s = 0;
  for (let x = 0; x <= pct; x += 0.5) {
    const mbs = speedAt(base, x);
    if (x > 0) s += (totalMB * 0.005) / mbs;
    out.push({ x, mbs, s });
  }
  return out;
}

export const fmtSeconds = (s: number, lang: Lang) => {
  const r = Math.max(0, Math.round(s));
  if (r < 60) return `${r} s`;
  return `${Math.floor(r / 60)} min ${String(r % 60).padStart(2, '0')} s`;
};
export const fmtNumber = (n: number, digits: number, lang: Lang) => n.toLocaleString(lang === 'de' ? 'de-DE' : 'en-GB', { minimumFractionDigits: digits, maximumFractionDigits: digits });
