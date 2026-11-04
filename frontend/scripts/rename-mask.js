// The tokens of the rename window as backend/internal/tools/rename.go reads them, for the fixture's fs/rename/batch.
// [N] name, [E] ending, [D] date, [P] folder, [C] counter with [C3] for three digits, and from the metadata [A] taken,
// [K] camera, [I] artist, [T] title, [R] resolution, [S2] season and [F2] episode.
const TOKEN = /\[([NEDPAKIRT])\]|\[([CSF])(\d?)\]/g;

const isoDate = (t) => new Date(t).toISOString().slice(0, 10);

export function splitName(name, dir) {
  const dot = dir ? -1 : name.lastIndexOf('.');
  return dot > 0 ? [name.slice(0, dot), name.slice(dot + 1)] : [name, ''];
}

const CASES = {
  keep: (s) => s,
  lower: (s) => s.toLowerCase(),
  upper: (s) => s.toUpperCase(),
  title: (s) => s.toLowerCase().replace(/(^|[\s_\-.(])(\p{L})/gu, (m, a, b) => a + b.toUpperCase()),
};

// Each entry is { name, dir, time, folder, meta(id) }; meta answers a token letter with the file's value or ''.
// Throws a SyntaxError for a find pattern that is no valid expression.
export function applyMask(mask, entries) {
  const start = mask.start ?? 0, step = mask.step || 1;
  const re = !mask.find ? null : mask.regex ? new RegExp(mask.find, 'g') : new RegExp(mask.find.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'gi');
  return entries.map((e, i) => {
    const [base, ext] = splitName(e.name, e.dir);
    const missing = new Set();
    const fill = (text) => text.replace(TOKEN, (m, t, counted, pad) => {
      if (t === 'N') return base;
      if (t === 'E') return ext;
      if (t === 'D') return isoDate(e.time);
      if (t === 'P') return e.folder;
      if (counted === 'C') return String(start + i * step).padStart(+pad || 1, '0');
      const id = t || counted, v = e.meta(id) || '';
      if (!v) missing.add(id);
      return counted ? v.padStart(+pad || 1, '0') : v;
    });
    let out = fill(mask.name).trim();
    const end = e.dir ? '' : fill(mask.ext);
    if (end) out += '.' + end;
    if (re) out = out.replace(re, mask.repl || '');
    return { name: (CASES[mask.case] || CASES.keep)(out).trim(), missing: [...missing] };
  });
}

// The error of every row: empty, invalid, a name an entry that stays already has, or one given twice in a folder.
// Names are compared without case, as SMB clients on Windows and macOS see them.
export function rowErrors(rows, takenIn) {
  const seen = new Map();
  const key = (r) => r.folder + '\0' + r.new.toLowerCase();
  rows.forEach((r) => seen.set(key(r), (seen.get(key(r)) || 0) + 1));
  return rows.map((r) => (!r.new ? 'empty'
    : /[/\0]/.test(r.new) || r.new === '.' || r.new === '..' ? 'invalid'
      : takenIn(r.folder).has(r.new.toLowerCase()) ? 'exists'
        : seen.get(key(r)) > 1 ? 'duplicate' : ''));
}
