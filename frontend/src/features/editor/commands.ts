// The line and text commands Notepad++ is used for, beyond what CodeMirror's own keymap brings. Each works on the
// lines the selection touches, or on the whole file when nothing is selected.
import { EditorSelection, type EditorState, type StateCommand, type Text } from '@codemirror/state';

// The lines from the first to the last one a selection touches, or every line.
function blockOf(state: EditorState): { from: number; to: number } {
  const sel = state.selection;
  if (sel.ranges.every((r) => r.empty)) return { from: 1, to: state.doc.lines };
  const doc: Text = state.doc;
  const from = doc.lineAt(Math.min(...sel.ranges.map((r) => r.from))).number;
  const last = Math.max(...sel.ranges.map((r) => r.to));
  // A selection that ends at the start of a line leaves that line out, as it looks.
  let to = doc.lineAt(last).number;
  if (to > from && doc.line(to).from === last) to--;
  return { from, to };
}

function rewriteLines(fn: (lines: string[]) => string[]): StateCommand {
  return ({ state, dispatch }) => {
    if (state.readOnly) return false;
    const { from, to } = blockOf(state);
    const start = state.doc.line(from).from, end = state.doc.line(to).to;
    const lines = state.sliceDoc(start, end).split('\n');
    const out = fn(lines).join('\n');
    if (out === state.sliceDoc(start, end)) return true;
    dispatch(state.update({ changes: { from: start, to: end, insert: out }, selection: EditorSelection.range(start, start + out.length), scrollIntoView: true, userEvent: 'input' }));
    return true;
  };
}

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' });
export const sortLines = rewriteLines((l) => [...l].sort(collator.compare));
export const sortLinesDown = rewriteLines((l) => [...l].sort((a, b) => collator.compare(b, a)));
export const trimTrailing = rewriteLines((l) => l.map((s) => s.replace(/[ \t]+$/, '')));

function changeCase(fn: (s: string) => string): StateCommand {
  return ({ state, dispatch }) => {
    if (state.readOnly || state.selection.ranges.every((r) => r.empty)) return false;
    dispatch(state.update(state.changeByRange((r) => ({ range: r, changes: { from: r.from, to: r.to, insert: fn(state.sliceDoc(r.from, r.to)) } })), { userEvent: 'input' }));
    return true;
  };
}
export const upperCase = changeCase((s) => s.toLocaleUpperCase());
export const lowerCase = changeCase((s) => s.toLocaleLowerCase());

// Tabs or spaces, and how many, as most indented lines of the text have it. A text without indents keeps the default.
export function detectIndent(text: string, fallback = { tabs: false, width: 4 }): { tabs: boolean; width: number } {
  let tabs = 0, spaces = 0;
  const steps = new Map<number, number>();
  let prev = 0;
  for (const line of text.slice(0, 200_000).split('\n')) {
    if (!line.trim()) continue;
    const lead = line.match(/^[ \t]*/)![0];
    if (lead.startsWith('\t')) tabs++;
    else if (lead.length) spaces++;
    const n = lead.startsWith('\t') ? 0 : lead.length;
    const step = Math.abs(n - prev);
    if (step > 1 && step <= 8 && !lead.startsWith('\t')) steps.set(step, (steps.get(step) ?? 0) + 1);
    prev = n;
  }
  if (!tabs && !spaces) return fallback;
  if (tabs > spaces) return { tabs: true, width: fallback.width };
  const width = [...steps].sort((a, b) => b[1] - a[1] || a[0] - b[0])[0]?.[0] ?? fallback.width;
  return { tabs: false, width };
}
