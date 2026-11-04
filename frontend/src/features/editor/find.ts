// Find and replace as the prototype does it: plain text, any case, one match after the other.
export type Range = [from: number, to: number];

export function findAll(text: string, needle: string): Range[] {
  const out: Range[] = [];
  if (!needle) return out;
  const hay = text.toLowerCase(), n = needle.toLowerCase();
  for (let i = hay.indexOf(n); i >= 0; i = hay.indexOf(n, i + n.length)) out.push([i, i + n.length]);
  return out;
}

// The first match at or after the caret, so a search starts where you are.
export const nearest = (matches: Range[], pos: number) => (matches.length ? Math.max(0, matches.findIndex(([a]) => a >= pos)) : -1);

export const position = (text: string, pos: number) => {
  const before = text.slice(0, pos).split('\n');
  return { line: before.length, col: before.at(-1)!.length + 1 };
};
