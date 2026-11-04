// CodeMirror dressed as the prototype's editor: line numbers in the margin, the caret's line lit, find marks in
// the accent colour and a quiet highlight that matches the viewer's.
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands';
import { HighlightStyle, type LanguageSupport, StreamLanguage, syntaxHighlighting } from '@codemirror/language';
import { Compartment, type Extension, StateEffect, StateField } from '@codemirror/state';
import { Decoration, type DecorationSet, drawSelection, EditorView, highlightActiveLine, highlightActiveLineGutter, keymap, lineNumbers } from '@codemirror/view';
import { tags } from '@lezer/highlight';
import { ext } from '@/features/viewer/kinds';
import type { Range } from './find';

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

const quiet = HighlightStyle.define([
  { tag: [tags.comment, tags.meta], color: 'var(--carbon-text-muted)' },
  { tag: [tags.string, tags.special(tags.string), tags.regexp], color: 'var(--carbon-text-sub)' },
  { tag: [tags.keyword, tags.propertyName, tags.definitionKeyword, tags.modifier, tags.heading, tags.tagName], color: 'var(--accent-ink)' },
  { tag: tags.strong, fontWeight: '600' },
  { tag: tags.emphasis, fontStyle: 'italic' },
]);

const look = EditorView.theme({
  '&': { height: '100%', background: 'transparent', color: 'var(--carbon-text)', fontSize: '13px' },
  '&.cm-focused': { outline: 'none' },
  '.cm-scroller': { fontFamily: 'ui-monospace, "Cascadia Code", Menlo, monospace', lineHeight: '1.6', letterSpacing: '0', overflow: 'auto' },
  '.cm-content': { padding: '.7rem 0 3rem', caretColor: 'var(--accent-ink)', tabSize: '4' },
  '.cm-line': { padding: '0 1rem 0 1.1rem' },
  '.cm-gutters': { background: 'transparent', border: 'none', color: 'var(--carbon-text-muted)', fontVariantNumeric: 'tabular-nums' },
  '.cm-gutter.cm-lineNumbers .cm-gutterElement': { width: '3.1rem', padding: '0', textAlign: 'right' },
  '.cm-activeLine, .cm-activeLineGutter': { background: 'var(--carbon-hover)' },
  '.cm-activeLineGutter': { color: 'var(--carbon-text)' },
  '.cm-cursor': { borderLeftColor: 'var(--accent-ink)', borderLeftWidth: '2px' },
  '&.cm-focused .cm-selectionBackground, .cm-selectionBackground, ::selection': { background: 'color-mix(in srgb, var(--accent) 30%, transparent) !important' },
  '.ce-mark': { background: 'color-mix(in srgb, var(--accent) 35%, transparent)', borderRadius: '3px' },
  '.ce-mark.cur': { background: 'var(--accent)' },
});

async function language(name: string): Promise<LanguageSupport | Extension | null> {
  const e = ext(name);
  if (['js', 'mjs', 'cjs', 'jsx', 'ts', 'tsx', 'vue'].includes(e)) return (await import('@codemirror/lang-javascript')).javascript({ typescript: e.startsWith('ts'), jsx: e.endsWith('x') });
  if (e === 'json') return (await import('@codemirror/lang-json')).json();
  if (e === 'yml' || e === 'yaml') return (await import('@codemirror/lang-yaml')).yaml();
  if (e === 'md' || e === 'markdown') return (await import('@codemirror/lang-markdown')).markdown();
  if (['xml', 'html', 'htm', 'svg', 'plist'].includes(e)) return (await import('@codemirror/lang-xml')).xml();
  if (e === 'go') return (await import('@codemirror/lang-go')).go();
  if (['sh', 'bash', 'zsh'].includes(e) || name === 'go' || name === 'Dockerfile') return StreamLanguage.define((await import('@codemirror/legacy-modes/mode/shell')).shell);
  if (e === 'conf') return StreamLanguage.define((await import('@codemirror/legacy-modes/mode/nginx')).nginx);
  if (['ini', 'cfg', 'toml', 'properties', 'env'].includes(e) || name === 'ident') return StreamLanguage.define((await import('@codemirror/legacy-modes/mode/properties')).properties);
  return null;
}

export interface EditorHooks { changed: () => void; moved: () => void }

export function makeEditor(parent: HTMLElement, doc: string, name: string, hooks: EditorHooks) {
  const wrap = new Compartment(), lang = new Compartment();
  // Go wants tabs, everything else here two spaces.
  const indent = name.endsWith('.go') ? '\t' : '  ';
  const view = new EditorView({
    parent,
    doc,
    extensions: [
      lineNumbers(), highlightActiveLine(), highlightActiveLineGutter(), drawSelection(), history(),
      keymap.of([{ key: 'Tab', run: (v) => { v.dispatch(v.state.replaceSelection(indent)); return true; } }, ...historyKeymap, ...defaultKeymap]),
      syntaxHighlighting(quiet), matchField, look, wrap.of([]), lang.of([]),
      EditorView.contentAttributes.of({ spellcheck: 'false', autocapitalize: 'off', 'aria-label': name }),
      EditorView.updateListener.of((u) => {
        if (u.docChanged) hooks.changed();
        if (u.docChanged || u.selectionSet) hooks.moved();
      }),
    ],
  });
  language(name).then((l) => { if (l) view.dispatch({ effects: lang.reconfigure(l) }); }).catch(() => {});
  return {
    view,
    setWrap: (on: boolean) => view.dispatch({ effects: wrap.reconfigure(on ? EditorView.lineWrapping : []) }),
  };
}
