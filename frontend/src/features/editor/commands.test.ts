import { EditorSelection, EditorState, type StateCommand } from '@codemirror/state';
import { describe, expect, it } from 'vitest';
import { detectIndent, lowerCase, sortLines, sortLinesDown, trimTrailing, upperCase } from './commands';

function apply(cmd: StateCommand, doc: string, sel?: EditorSelection) {
  let state = EditorState.create({ doc, selection: sel, extensions: EditorState.allowMultipleSelections.of(true) });
  cmd({ state, dispatch: (tr) => { state = tr.state; } });
  return state.doc.toString();
}

describe('line commands', () => {
  it('sorts every line without a selection, numbers by their value', () => {
    expect(apply(sortLines, 'b\na10\nA2\nc')).toBe('A2\na10\nb\nc');
    expect(apply(sortLinesDown, 'b\na\nc')).toBe('c\nb\na');
  });

  it('sorts only the lines a selection touches', () => {
    expect(apply(sortLines, 'z\nc\nb\na', EditorSelection.single(2, 5))).toBe('z\nb\nc\na');
    // Ending at the start of the next line leaves that line where it is.
    expect(apply(sortLines, 'c\nb\na', EditorSelection.single(0, 4))).toBe('b\nc\na');
  });

  it('trims the spaces and tabs at the end of lines', () => {
    expect(apply(trimTrailing, 'a  \nb\t\n  c ')).toBe('a\nb\n  c');
  });

  it('changes the case of every selected range', () => {
    const sel = EditorSelection.create([EditorSelection.range(0, 3), EditorSelection.range(9, 12)]);
    expect(apply(upperCase, 'grün ist grün', sel)).toBe('GRÜn ist GRÜn');
    expect(apply(lowerCase, 'ABC', EditorSelection.single(0, 3))).toBe('abc');
    expect(apply(upperCase, 'abc')).toBe('abc');
  });
});

describe('indentation', () => {
  it('reads tabs or spaces and their width from the text', () => {
    expect(detectIndent('a:\n  b:\n    c: 1\n  d: 2')).toEqual({ tabs: false, width: 2 });
    expect(detectIndent('func a() {\n\tif x {\n\t\ty()\n\t}\n}')).toEqual({ tabs: true, width: 4 });
    expect(detectIndent('def a():\n    if x:\n        y()\n')).toEqual({ tabs: false, width: 4 });
    expect(detectIndent('plain\ntext', { tabs: true, width: 8 })).toEqual({ tabs: true, width: 8 });
  });
});
