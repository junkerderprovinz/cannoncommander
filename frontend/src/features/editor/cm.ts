// CodeMirror dressed as the prototype's editor and taught Notepad++'s habits: line numbers and folding in the margin,
// the caret's line lit, brackets matched, a block selected with Alt and the mouse, more cursors with Ctrl and a
// click, the other places of the selected word marked, and the find marks in the accent colour.
import { copyLineDown, defaultKeymap, deleteLine, history, historyKeymap, indentLess, indentMore, moveLineDown, moveLineUp, toggleComment } from '@codemirror/commands';
import { bracketMatching, foldAll, foldCode, foldGutter, HighlightStyle, indentOnInput, indentUnit, syntaxHighlighting, unfoldAll, unfoldCode } from '@codemirror/language';
import { highlightSelectionMatches } from '@codemirror/search';
import { Compartment, EditorState, type Extension, StateEffect, StateField } from '@codemirror/state';
import {
  type Command, crosshairCursor, Decoration, type DecorationSet, drawSelection, dropCursor, EditorView, highlightActiveLine, highlightActiveLineGutter,
  highlightSpecialChars, keymap, lineNumbers, rectangularSelection,
} from '@codemirror/view';
import { tags } from '@lezer/highlight';
import { lowerCase, sortLines, sortLinesDown, trimTrailing, upperCase } from './commands';
import type { Doc, Indent } from './docs';
import type { Range } from './find';
import { byId } from './languages';

export const setMatches = StateEffect.define<{ ranges: Range[]; hit: number }>();

const matchMark = Decoration.mark({ class: 'ce-mark' });
const hitMark = Decoration.mark({ class: 'ce-mark cur' });
export const matchField = StateField.define<DecorationSet>({
  create: () => Decoration.none,
  update(deco, tr) {
    deco = deco.map(tr.changes);
    for (const e of tr.effects) {
      if (e.is(setMatches)) deco = Decoration.set(e.value.ranges.map(([a, b], k) => (k === e.value.hit ? hitMark : matchMark).range(a, b)));
    }
    return deco;
  },
  provide: (f) => EditorView.decorations.from(f),
});

// The theme's own colours, so the code reads the same in light and dark and in every accent.
const style = HighlightStyle.define([
  { tag: [tags.comment, tags.meta], color: 'var(--carbon-text-muted)' },
  { tag: [tags.string, tags.special(tags.string), tags.regexp, tags.character, tags.url], color: 'var(--status-ok-text)' },
  { tag: [tags.keyword, tags.controlKeyword, tags.operatorKeyword, tags.moduleKeyword, tags.definitionKeyword, tags.modifier, tags.heading, tags.tagName], color: 'var(--accent-ink)' },
  { tag: [tags.number, tags.bool, tags.atom, tags.null, tags.escape, tags.color, tags.unit], color: 'var(--status-warn-text)' },
  { tag: [tags.typeName, tags.className, tags.namespace, tags.attributeName, tags.propertyName, tags.labelName, tags.special(tags.variableName)], color: 'var(--status-info-text)' },
  { tag: [tags.function(tags.variableName), tags.function(tags.propertyName), tags.definition(tags.variableName)], color: 'var(--carbon-text)', fontWeight: '600' },
  { tag: [tags.inserted], color: 'var(--status-ok-text)' },
  { tag: [tags.deleted, tags.invalid], color: 'var(--status-fail-text)' },
  { tag: tags.strong, fontWeight: '600' },
  { tag: tags.emphasis, fontStyle: 'italic' },
  { tag: tags.link, textDecoration: 'underline' },
]);

const look = EditorView.theme({
  '&': { height: '100%', background: 'transparent', color: 'var(--carbon-text)', fontSize: '13px' },
  '&.cm-focused': { outline: 'none' },
  '.cm-scroller': { fontFamily: 'ui-monospace, "Cascadia Code", Menlo, monospace', lineHeight: '1.6', letterSpacing: '0', overflow: 'auto' },
  '.cm-content': { padding: '.7rem 0 3rem', caretColor: 'var(--accent-ink)' },
  '.cm-line': { padding: '0 1rem 0 1.1rem' },
  '.cm-gutters': { background: 'transparent', border: 'none', color: 'var(--carbon-text-muted)', fontVariantNumeric: 'tabular-nums' },
  '.cm-gutter.cm-lineNumbers .cm-gutterElement': { width: '3.1rem', padding: '0', textAlign: 'right' },
  '.cm-foldGutter .cm-gutterElement': { width: '1.1rem', textAlign: 'center', cursor: 'pointer', opacity: '.7' },
  '.cm-activeLine, .cm-activeLineGutter': { background: 'var(--carbon-hover)' },
  '.cm-activeLineGutter': { color: 'var(--carbon-text)' },
  '.cm-cursor, .cm-dropCursor': { borderLeftColor: 'var(--accent-ink)', borderLeftWidth: '2px' },
  '&.cm-focused .cm-selectionBackground, .cm-selectionBackground, ::selection': { background: 'color-mix(in srgb, var(--accent) 30%, transparent) !important' },
  '.cm-selectionMatch': { background: 'color-mix(in srgb, var(--accent) 16%, transparent)', borderRadius: '2px' },
  '&.cm-focused .cm-matchingBracket': { background: 'color-mix(in srgb, var(--accent) 28%, transparent)', outline: '1px solid color-mix(in srgb, var(--accent-ink) 55%, transparent)', borderRadius: '2px' },
  '&.cm-focused .cm-nonmatchingBracket': { background: 'color-mix(in srgb, var(--status-fail-text) 25%, transparent)' },
  '.cm-foldPlaceholder': { background: 'var(--carbon-surface3)', border: 'none', color: 'var(--carbon-text-sub)', borderRadius: 'var(--radius-pill)', padding: '0 .45rem', margin: '0 .2rem' },
  '.ce-mark': { background: 'color-mix(in srgb, var(--accent) 35%, transparent)', borderRadius: '3px' },
  '.ce-mark.cur': { background: 'var(--accent)' },
  // Vim's block caret and its command line.
  '.cm-fat-cursor': { background: 'color-mix(in srgb, var(--accent-ink) 70%, transparent) !important', color: 'var(--accent-contrast) !important' },
  '&:not(.cm-focused) .cm-fat-cursor': { background: 'none !important', outline: '1px solid var(--accent-ink)' },
  '.cm-vim-panel': { padding: '.25rem 1.1rem', fontFamily: 'ui-monospace, "Cascadia Code", Menlo, monospace', background: 'var(--carbon-surface3)' },
  '.cm-vim-panel input': { background: 'transparent', color: 'var(--carbon-text)', border: 'none', outline: 'none', fontFamily: 'inherit' },
  '.cm-panels': { background: 'transparent', color: 'var(--carbon-text)' },
});

// The commands that have a named action of their own: CodeMirror's keys for them go, so the key the person gives
// such an action is the only one it has.
const NAMED = new Set<unknown>([moveLineUp, moveLineDown, copyLineDown, toggleComment, deleteLine, indentLess, indentMore]);
const ownKeys = defaultKeymap.filter((b) => !NAMED.has(b.run));

// Tab indents the lines of a selection that spans more than one, and else types the indent, as Notepad++ does.
const tab: Command = (v) => {
  const { state } = v;
  if (state.readOnly) return false;
  if (state.selection.ranges.some((r) => state.doc.lineAt(r.from).number !== state.doc.lineAt(r.to).number)) return indentMore(v);
  v.dispatch(state.replaceSelection(state.facet(indentUnit)), { scrollIntoView: true, userEvent: 'input' });
  return true;
};

// The commands behind the editor's named actions, by the name the window runs them by.
export const COMMANDS: Record<string, Command> = {
  duplicateLine: copyLineDown, deleteLine, moveLineUp, moveLineDown, toggleComment,
  sortLines, sortLinesDown, trimTrailing, upperCase, lowerCase,
  fold: foldCode, unfold: unfoldCode, foldAll, unfoldAll,
};

export const parts = { lang: new Compartment(), wrap: new Compartment(), indent: new Compartment(), readOnly: new Compartment(), vim: new Compartment() };

export const indentOf = (i: Indent): Extension => [indentUnit.of(i.tabs ? '\t' : ' '.repeat(i.width)), EditorState.tabSize.of(i.width)];
export const wrapOf = (on: boolean): Extension => (on ? EditorView.lineWrapping : []);
export const readOnlyOf = (on: boolean): Extension => [EditorState.readOnly.of(on)];

export interface EditorHooks { changed: () => void; moved: () => void }

export function makeState(text: string, doc: Doc, hooks: EditorHooks, now: { wrap: boolean; vim: Extension }) {
  return EditorState.create({
    doc: text,
    extensions: [
      parts.vim.of(now.vim),
      lineNumbers(), foldGutter({ markerDOM: foldMarker }), highlightActiveLine(), highlightActiveLineGutter(), highlightSpecialChars(),
      drawSelection(), dropCursor(), EditorState.allowMultipleSelections.of(true), rectangularSelection(), crosshairCursor(),
      history(), bracketMatching(), indentOnInput(), highlightSelectionMatches({ minSelectionLength: 2 }),
      keymap.of([{ key: 'Tab', run: tab, shift: indentLess }, ...historyKeymap, ...ownKeys]),
      syntaxHighlighting(style), matchField, look,
      parts.wrap.of(wrapOf(now.wrap)), parts.lang.of([]), parts.indent.of(indentOf(doc.indent)), parts.readOnly.of(readOnlyOf(doc.readOnly)),
      EditorView.contentAttributes.of({ spellcheck: 'false', autocapitalize: 'off', 'aria-label': doc.name }),
      EditorView.updateListener.of((u) => {
        if (u.docChanged) hooks.changed();
        if (u.docChanged || u.selectionSet) hooks.moved();
      }),
    ],
  });
}

// A chevron that points down over a fold that is open and along the line over one that is closed.
function foldMarker(open: boolean) {
  const el = document.createElement('span');
  el.className = open ? 'ce-fold open' : 'ce-fold';
  el.innerHTML = '<svg viewBox="0 0 14 14" aria-hidden="true"><path d="M5 3l4 4-4 4" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>';
  return el;
}

// Each language comes once, then from here for every tab that needs it.
const loaded = new Map<string, Promise<Extension>>();
export function languageOf(id: string): Promise<Extension> {
  const lang = byId(id);
  if (!lang) return Promise.resolve([]);
  if (!loaded.has(id)) {
    const got = lang.load();
    got.catch(() => loaded.delete(id));
    loaded.set(id, got);
  }
  return loaded.get(id)!;
}

// What :w and :q do, set by the window that is open.
export const vimHooks = { save: async () => false, close: () => {} };
const saveAndClose = async () => { if (await vimHooks.save()) vimHooks.close(); };

let vimLoaded: Promise<Extension> | null = null;
// Vim comes in a chunk of its own, the first time someone switches it on. :w saves the tab and :q closes it.
export function vimOf(): Promise<Extension> {
  vimLoaded ??= import('@replit/codemirror-vim').then(({ vim, Vim }) => {
    Vim.defineEx('write', 'w', () => { void vimHooks.save(); });
    Vim.defineEx('quit', 'q', () => vimHooks.close());
    Vim.defineEx('wq', 'wq', () => { void saveAndClose(); });
    Vim.defineEx('xit', 'x', () => { void saveAndClose(); });
    return vim({ status: true });
  });
  return vimLoaded;
}

export const makeView = (parent: HTMLElement, state: EditorState) => new EditorView({ parent, state });
