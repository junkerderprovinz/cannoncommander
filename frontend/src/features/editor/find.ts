// Find and replace as Notepad++ does it: plain text or a regular expression, in any case or the exact one, as part
// of a word or as a whole word, one match after the other.
export type Range = [from: number, to: number];
export interface FindOptions { regex?: boolean; matchCase?: boolean; word?: boolean }

// Past this many marks a search only slows the typing down, and nobody steps through them one by one.
export const MAX_MATCHES = 20000;

const escape = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

// The pattern behind a search, or null while the field is empty or holds an expression that is still being typed.
export function pattern(needle: string, o: FindOptions = {}): RegExp | null {
  if (!needle) return null;
  let src = o.regex ? needle : escape(needle);
  if (o.word) src = `\\b(?:${src})\\b`;
  try {
    return new RegExp(src, 'gmu' + (o.matchCase ? '' : 'i'));
  } catch {
    return null;
  }
}

export function findAll(text: string, needle: string, o: FindOptions = {}): Range[] {
  const re = pattern(needle, o), out: Range[] = [];
  if (!re) return out;
  for (let m = re.exec(text); m && out.length < MAX_MATCHES; m = re.exec(text)) {
    // An expression that matches nothing, such as ^, would stand still.
    if (!m[0].length) {
      re.lastIndex++;
      continue;
    }
    out.push([m.index, m.index + m[0].length]);
  }
  return out;
}

// What a match turns into. With an expression, $1 or \1 bring back a group, $& the whole match, and \n and \t a line
// break and a tab, as in Notepad++.
export function replacement(text: string, at: Range, needle: string, rep: string, o: FindOptions = {}): string {
  if (!o.regex) return rep;
  const re = pattern(needle, o);
  if (!re) return rep;
  const sticky = new RegExp(re.source, re.flags.replace('g', '') + 'y');
  sticky.lastIndex = at[0];
  const m = sticky.exec(text);
  if (!m) return rep;
  return rep.replace(/\$(\$|&|\d{1,2})|\\(\d|n|t|\\)/g, (all, dollar: string | undefined, slash: string | undefined) => {
    const k = dollar ?? slash!;
    if (k === '$') return '$';
    if (k === '&') return m[0];
    if (k === 'n') return '\n';
    if (k === 't') return '\t';
    if (k === '\\') return '\\';
    return m[Number(k)] ?? all;
  });
}

// The first match at or after the caret, so a search starts where you are.
export const nearest = (matches: Range[], pos: number) => (matches.length ? Math.max(0, matches.findIndex(([a]) => a >= pos)) : -1);

export const position = (text: string, pos: number) => {
  const before = text.slice(0, pos).split('\n');
  return { line: before.length, col: before.at(-1)!.length + 1 };
};
