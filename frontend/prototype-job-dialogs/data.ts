// Prototype: made-up jobs in every state, in German and English.
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
    from: 'Von', to: 'Nach', way: 'Art', clash: 'Wenn es die Datei schon gibt', more: 'Weitere Optionen', keepAttrs: 'Rechte und Zeitstempel übernehmen',
    rules: ['Fragen', 'Überspringen', 'Ersetzen', 'Wenn neuer', 'Beide behalten'], ways: { copy: 'Kopieren', move: 'Verschieben', link: 'Hart verlinken' },
    keep: 'Fenster nach Abschluss offen lassen', background: 'Im Hintergrund', cancel: 'Abbrechen', pause: 'Anhalten', resume: 'Weiter', close: 'Schließen',
    retry: 'Fehlgeschlagene wiederholen', goOn: 'Fortsetzen', open: 'Ziel öffnen', toTrash: 'In den Papierkorb', trashToggle: 'In den Papierkorb legen',
    trashHint: 'Bleibt 30 Tage im Papierkorb und lässt sich wiederherstellen.', details: 'Details', options: 'Optionen', optionsLine: 'Bei gleichem Namen fragen · Rechte übernehmen',
    replace: 'Ersetzen', skip: 'Überspringen', both: 'Beide behalten', forAll: 'Für alle übernehmen', taken: 'Gibt es im Ziel schon', newSide: 'Neu', oldSide: 'Vorhanden',
    speed: 'Tempo', left: 'Noch', files: 'Dateien', size: 'Größe', progress: 'Fortschritt', now: 'Gerade',
    sentence: { copy: 'kopieren nach', move: 'verschieben nach', delete: 'in den Papierkorb legen' },
    ask: { copy: 'Kopieren?', move: 'Verschieben?', delete: 'In den Papierkorb legen?' },
    doneLine: { copy: '4,2 GB in 38 s kopiert', move: '8,4 GB in 1 min 12 s verschoben', delete: '3 Objekte liegen im Papierkorb' },
    errorsLine: { copy: '10 von 12 Dateien kopiert, 2 fehlgeschlagen', move: '1 Datei nicht verschoben', delete: '2 von 3 Objekten im Papierkorb, 1 fehlgeschlagen' },
    cancelledLine: { copy: 'Nach 1,1 GB abgebrochen. Was schon kopiert ist, bleibt im Ziel.', move: 'Nach 2,3 GB abgebrochen. Die Quelle ist vollständig.', delete: 'Nach 1 von 3 Objekten abgebrochen.' },
    error: 'Kein Platz mehr auf dem Ziel',
    variant: { A: 'Wie jetzt', B: 'Kompakt in einer Spalte', C: 'Zwei Zonen: Weg oben, Fortschritt unten', D: 'Schlicht, Details auf Wunsch' },
    all: 'Alle', light: 'Hell', dark: 'Dunkel', oled: 'OLED',
  },
  en: {
    dialog: { copy: 'Copy', move: 'Move', delete: 'Delete' },
    state: { confirm: 'Confirm', running: 'Running', paused: 'Paused', conflict: 'Conflict', done: 'Done', errors: 'Done with errors', cancelled: 'Cancelled' },
    badge: { running: 'running', paused: 'paused', conflict: 'asking', done: 'done', errors: 'with errors', cancelled: 'cancelled' },
    from: 'From', to: 'To', way: 'Way', clash: 'When the file exists', more: 'More options', keepAttrs: 'Keep owner, rights and times',
    rules: ['Ask', 'Skip', 'Replace', 'If newer', 'Keep both'], ways: { copy: 'Copy', move: 'Move', link: 'Hard link' },
    keep: 'Keep this window open when done', background: 'In the background', cancel: 'Cancel', pause: 'Pause', resume: 'Resume', close: 'Close',
    retry: 'Retry the failed ones', goOn: 'Resume', open: 'Open target', toTrash: 'Move to trash', trashToggle: 'Move to the trash',
    trashHint: 'Stays in the trash for 30 days and can be restored.', details: 'Details', options: 'Options', optionsLine: 'Ask on the same name · Keep rights',
    replace: 'Replace', skip: 'Skip', both: 'Keep both', forAll: 'Use for all', taken: 'Already in the target', newSide: 'New', oldSide: 'There',
    speed: 'Speed', left: 'Left', files: 'Files', size: 'Size', progress: 'Progress', now: 'Now',
    sentence: { copy: 'copy to', move: 'move to', delete: 'move to the trash' },
    ask: { copy: 'Copy?', move: 'Move?', delete: 'Move to the trash?' },
    doneLine: { copy: 'Copied 4.2 GB in 38 s', move: 'Moved 8.4 GB in 1 min 12 s', delete: '3 items are in the trash' },
    errorsLine: { copy: 'Copied 10 of 12 files, 2 failed', move: '1 file not moved', delete: '2 of 3 items in the trash, 1 failed' },
    cancelledLine: { copy: 'Cancelled after 1.1 GB. What is copied stays in the target.', move: 'Cancelled after 2.3 GB. The source is whole.', delete: 'Cancelled after 1 of 3 items.' },
    error: 'No room left on the target',
    variant: { A: 'As it is', B: 'Compact, one column', C: 'Two zones: route on top, progress below', D: 'Plain, details on demand' },
    all: 'All', light: 'Light', dark: 'Dark', oled: 'OLED',
  },
};
export const texts = (lang: Lang) => T[lang];

// What each dialog works on, the same in every variant.
export const JOBS = {
  copy: { name: 'Urlaub 2026', count: '12', from: '/mnt/user/fotos', to: '/mnt/user/backups/fotos', total: '4,2 GB', glyph: 'IconCopy',
    run: { pct: 43, done: '1,8 GB', speed: '112 MB/s', left: '22 s', files: '5 / 12', current: 'IMG_20260812_1430.jpg' },
    failed: ['IMG_20260814_0912.jpg', 'Urlaub 2026.mp4'] },
  move: { name: 'Citizen Vigilante (2026).mkv', count: '1', from: '/mnt/user/downloads', to: '/mnt/user/medien/Filme', total: '8,4 GB', glyph: 'IconMove',
    run: { pct: 27, done: '2,3 GB', speed: '96 MB/s', left: '1 min 04 s', files: '0 / 1', current: 'Citizen Vigilante (2026).mkv' },
    failed: ['Citizen Vigilante (2026).mkv'] },
  delete: { name: '3 Objekte', count: '3', from: '/mnt/user/dokumente', to: '', total: '38 MB', glyph: 'IconTrash',
    run: { pct: 33, done: '1 / 3', speed: '', left: '', files: '1 / 3', current: 'Steuer 2024' },
    failed: ['Belege 2025.ods'], items: ['Steuer 2024', 'Notizen.txt', 'Belege 2025.ods'] },
} as const;
