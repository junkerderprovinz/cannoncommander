<script setup lang="ts">
// The editor window's body: a tab for each file, CodeMirror in the prototype's look with a find and replace card over
// it, for markdown a live preview beside it, and the status bar under it. One view shows the tab in front, and each
// tab keeps its own text, history and selection while it is behind.
import { Button, Checkbox, IconButton, Pill, TextField, Toggle } from '@glimstone/vue';
import { type EditorState, type Extension, Text, type TransactionSpec } from '@codemirror/state';
import { EditorView } from '@codemirror/view';
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from 'vue';
import { ApiError } from '@/api/client';
import { G, glyphComponent } from '@/core/glyphs';
import { inScope, keymap } from '@/core/keymap';
import { isMenuOpen, menu, type MenuEntry } from '@/core/menu';
import { prefs, setPref } from '@/core/prefs';
import { toast } from '@/core/toast';
import { win, type WinEntry } from '@/core/windows';
import { editable, type FileNode, joinPath } from '@/features/viewer/kinds';
import { esc, glyphSvg, md } from '@/features/viewer/markdown';
import { winKeys } from '@/features/viewer/winkeys';
import { byName, plural, t } from '@/i18n';
import { useListing } from '@/stores/listing';
import { COMMANDS, indentOf, languageOf, makeState, makeView, parts, setMatches, vimHooks, vimOf, wrapOf } from './cm';
import { detectIndent } from './commands';
import { type Doc, type Eol, encodingName, type Indent, makeDoc, readFile, writeDoc } from './docs';
import { findAll, type FindOptions, nearest, type Range, replacement } from './find';
import GotoBody from './GotoBody.vue';
import { detect } from './languages';
import { actionId, commandOf, setEditorHost } from './session';
import { opts } from './state';
import StatusBar from './StatusBar.vue';

const props = defineProps<{ docs: Doc[]; close?: () => void; win?: WinEntry }>();

const docs = reactive<Doc[]>([]);
const active = ref(0);
const cur = computed(() => docs.find((d) => d.id === active.value));
const isMd = computed(() => cur.value?.lang === 'markdown');
const states = new Map<number, EditorState>();
const savedTexts = new Map<number, Text>();
const host = ref<HTMLElement | null>(null);
const pos = reactive({ line: 1, col: 1, chosen: 0 });
const findOpen = ref(false);
const q = ref('');
const rep = ref('');
const how = reactive<FindOptions>({ regex: false, matchCase: false, word: false });
const qIn = ref<InstanceType<typeof TextField> | null>(null);
const repIn = ref<InstanceType<typeof TextField> | null>(null);
const count = ref('');
const preview = ref('');
let view: EditorView | null = null;
let vim: Extension = [];
let matches: Range[] = [];
let hit = -1;

const text = () => view?.state.doc.toString() ?? '';
const stateOf = (d: Doc) => (d.id === active.value && view ? view.state : states.get(d.id));

// Go and makefiles need tabs. Languages written with two spaces get two, the rest four, until the text says otherwise.
const TWO = ['yaml', 'json', 'javascript', 'typescript', 'jsx', 'tsx', 'vue', 'html', 'css', 'scss', 'less', 'xml', 'markdown', 'ruby', 'dart'];
function guess(d: Doc) {
  d.lang = detect(d.name, d.saved.slice(0, 300))?.id ?? '';
  const fallback = { tabs: d.lang === 'go' || d.lang === 'makefile', width: TWO.includes(d.lang) ? 2 : 4 };
  d.indent = detectIndent(d.saved, fallback);
}

// A setting of one tab changes its view when it is in front, else the state it keeps for later.
function reconfigure(d: Doc, spec: TransactionSpec) {
  if (d.id === active.value && view) view.dispatch(spec);
  else {
    const s = states.get(d.id);
    if (s) states.set(d.id, s.update(spec).state);
  }
}

function stateFor(d: Doc) {
  savedTexts.set(d.id, Text.of(d.saved.split('\n')));
  const s = makeState(d.saved, d, { changed, moved }, { wrap: !!prefs.editorWrap, vim });
  if (d.lang) void languageOf(d.lang).then((ext) => reconfigure(d, { effects: parts.lang.reconfigure(ext) }), () => {});
  return s;
}

function markTitle() {
  const d = cur.value;
  if (!props.win || !d) return;
  props.win.title = d.name;
  props.win.dot = d.dirty ? t('editor.unsaved') : undefined;
}

function show(id: number) {
  if (!view) return;
  if (id === active.value) return void view.focus();
  if (cur.value) states.set(cur.value.id, view.state);
  active.value = id;
  const d = cur.value!;
  view.setState(states.get(id) ?? stateFor(d));
  states.delete(id);
  view.dispatch({ effects: [parts.wrap.reconfigure(wrapOf(!!prefs.editorWrap)), parts.vim.reconfigure(vim)] });
  preview.value = isMd.value ? md(text()) : '';
  markTitle();
  moved();
  if (findOpen.value) search();
  if (d.cut) toast(t('editor.tooLong'), 'warn');
  setTimeout(() => view?.focus(), 0);
}

function add(list: Doc[]) {
  let last = 0;
  for (const d of list) {
    const there = docs.find((o) => o.path === d.path);
    if (there) {
      last = there.id;
      continue;
    }
    guess(d);
    docs.push(d);
    last = d.id;
  }
  if (last) show(last);
}

// The comparison waits for a pause in the typing, since a large file takes a moment to compare.
let dirtyTimer: ReturnType<typeof setTimeout> | undefined;
function refreshDirty(d: Doc) {
  const s = stateOf(d);
  if (!s) return;
  const sameText = s.doc.eq(savedTexts.get(d.id) ?? Text.of(d.saved.split('\n')));
  d.dirty = !sameText || d.encoding !== d.savedEncoding || d.eol !== d.savedEol;
  if (d.id === active.value) markTitle();
}

let mdTimer: ReturnType<typeof setTimeout> | undefined;
function changed() {
  const d = cur.value;
  if (!d) return;
  clearTimeout(dirtyTimer);
  dirtyTimer = setTimeout(() => refreshDirty(d), 120);
  if (findOpen.value) search(true);
  if (isMd.value) {
    clearTimeout(mdTimer);
    mdTimer = setTimeout(() => { preview.value = md(text()); }, 120);
  }
}

function moved() {
  if (!view) return;
  const s = view.state, head = s.selection.main.head, line = s.doc.lineAt(head);
  Object.assign(pos, { line: line.number, col: head - line.from + 1, chosen: s.selection.ranges.reduce((n, r) => n + r.to - r.from, 0) });
}

function search(keep = false) {
  if (!view) return;
  matches = findAll(text(), q.value, how);
  if (!keep) hit = nearest(matches, view.state.selection.main.from);
  hit = Math.min(hit, matches.length - 1);
  count.value = q.value ? (matches.length ? t('editor.hits', { i: hit + 1, n: matches.length }) : t('editor.noHits')) : '';
  view.dispatch({ effects: setMatches.of({ ranges: matches, hit }) });
  if (hit >= 0) view.dispatch({ effects: EditorView.scrollIntoView(matches[hit]![0], { y: 'center' }) });
}
function step(d: number) {
  if (!findOpen.value || !q.value) {
    void openFind();
    return;
  }
  if (!matches.length || !view) return;
  hit = (hit + d + matches.length) % matches.length;
  view.dispatch({ selection: { anchor: matches[hit]![0], head: matches[hit]![1] } });
  search(true);
}
async function openFind(replace = false) {
  if (!view) return;
  const s = view.state.selection.main, picked = view.state.sliceDoc(s.from, s.to);
  if (picked && !picked.includes('\n')) q.value = picked;
  findOpen.value = true;
  await nextTick();
  const el = replace ? repIn.value?.input : qIn.value?.input;
  el?.focus();
  el?.select();
  search();
}
function closeFind() {
  findOpen.value = false;
  matches = [];
  hit = -1;
  view?.dispatch({ effects: setMatches.of({ ranges: [], hit: -1 }) });
  view?.focus();
}
function replaceOne() {
  if (hit < 0 || !view || view.state.readOnly) return;
  const [a, b] = matches[hit]!, insert = replacement(text(), [a, b], q.value, rep.value, how);
  view.dispatch({ changes: { from: a, to: b, insert }, selection: { anchor: a + insert.length } });
  search();
}
function replaceAll() {
  if (!matches.length || !view || view.state.readOnly) return;
  const all = text(), n = matches.length;
  view.dispatch({ changes: matches.map(([from, to]) => ({ from, to, insert: replacement(all, [from, to], q.value, rep.value, how) })) });
  search();
  toast(plural(n, 'editor.replaced'));
}
function option(key: keyof FindOptions, on: boolean) {
  how[key] = on;
  search();
}

async function save(d = cur.value): Promise<boolean> {
  const s = d && stateOf(d);
  if (!d || !s) return false;
  // Saving only the part that was read would cut the file short.
  if (d.cut) return toast(t('editor.tooLong'), 'warn'), false;
  if (d.readOnly) return toast(t('editor.readOnlyTip'), 'warn'), false;
  const now = s.doc.toString();
  try {
    await writeDoc(d, now);
    savedTexts.set(d.id, s.doc);
    refreshDirty(d);
    toast(t('editor.saved', { name: d.name }));
    const listing = useListing();
    listing.invalidate(d.folder);
    listing.load(d.folder).catch(() => {});
    return true;
  } catch (e) {
    const code = e instanceof ApiError ? e.problem?.code : undefined;
    toast(code === 'changedWhileOpen' ? t('editor.conflict') : code === 'notEncodable' ? (e as Error).message : t('editor.saveFailed', { name: d.name }), 'fault');
    return false;
  }
}
async function saveAll(list: Doc[]) {
  for (const d of list) if (!(await save(d))) return false;
  return true;
}

function ask(list: Doc[], then: () => void) {
  const what = list.length === 1 ? t('editor.askText', { name: `<b>${esc(list[0]!.name)}</b>` }) : plural(list.length, 'editor.askMany');
  win({
    title: t('editor.askTitle'),
    body: `<div class="glim-subcard"><div class="ce-ask">${glyphSvg('IconWarning')}<span>${what}</span></div></div>`,
    foot: [
      { label: t('editor.keepEditing'), g: 'IconPencil', run: () => { setTimeout(() => view?.focus(), 0); } },
      { label: t('editor.discard'), g: 'IconUndo', run: () => { setTimeout(then, 0); } },
      { label: t(list.length === 1 ? 'editor.save' : 'editor.saveAll'), g: 'IconSave', primary: true, run: () => { void saveAll(list).then((ok) => ok && setTimeout(then, 0)); } },
    ],
  });
}

function drop(d: Doc) {
  const at = docs.indexOf(d);
  states.delete(d.id);
  savedTexts.delete(d.id);
  docs.splice(at, 1);
  if (!docs.length) return props.close?.();
  if (d.id === active.value) {
    active.value = 0;
    show(docs[Math.min(at, docs.length - 1)]!.id);
  }
}
function closeTab(d = cur.value) {
  if (!d) return;
  if (!d.dirty) return drop(d);
  if (d.id !== active.value) show(d.id);
  ask([d], () => drop(d));
}
function leave() {
  const d = cur.value;
  if (d) refreshDirty(d);
  const unsaved = docs.filter((o) => o.dirty);
  if (!unsaved.length) return true;
  ask(unsaved, () => { for (const o of unsaved) o.dirty = false; props.close?.(); });
  return false;
}

function cycle(by: number) {
  if (docs.length < 2) return;
  const at = docs.findIndex((d) => d.id === active.value);
  show(docs[(at + by + docs.length) % docs.length]!.id);
}

function gotoLine() {
  if (!view) return;
  const lines = view.state.doc.lines;
  win({
    title: t('editor.cmd.gotoLine'), body: GotoBody, props: { value: String(pos.line), lines },
    foot: [{ label: t('editor.go'), g: 'IconForward', primary: true, run: (el) => {
      const m = (el.querySelector<HTMLInputElement>('#ce-goto')?.value || '').trim().match(/^(\d+)(?:[:,.\s]+(\d+))?$/);
      if (!m || !view) return false;
      const line = view.state.doc.line(Math.min(Math.max(1, Number(m[1])), lines));
      const at = line.from + Math.min(Math.max(0, Number(m[2] || 1) - 1), line.length);
      view.dispatch({ selection: { anchor: at }, effects: EditorView.scrollIntoView(at, { y: 'center' }) });
      setTimeout(() => view?.focus(), 0);
      return undefined;
    } }],
  });
}

function setWrap(on: boolean) {
  setPref('editorWrap', on);
  view?.dispatch({ effects: parts.wrap.reconfigure(wrapOf(on)) });
}

function command(name: string) {
  if (!view) return;
  switch (name) {
    case 'save': void save(); return;
    case 'find': void openFind(); return;
    case 'replace': void openFind(true); return;
    case 'findNext': step(1); return;
    case 'findPrev': step(-1); return;
    case 'gotoLine': gotoLine(); return;
    case 'wrap': setWrap(!prefs.editorWrap); return;
    case 'nextTab': cycle(1); return;
    case 'prevTab': cycle(-1); return;
    case 'closeTab': closeTab(); return;
  }
  const cmd = COMMANDS[name];
  if (!cmd) return;
  if (view.state.readOnly && !/fold/i.test(name)) return toast(t('editor.readOnlyTip'), 'warn');
  cmd(view);
  view.focus();
}

// The status bar's pickers.
function setLang(id: string) {
  const d = cur.value!;
  d.lang = id;
  void (id ? languageOf(id) : Promise.resolve([])).then((ext) => reconfigure(d, { effects: parts.lang.reconfigure(ext) }));
  if (isMd.value) preview.value = md(text());
}
function setIndent(i: Indent) {
  const d = cur.value!;
  d.indent = i;
  reconfigure(d, { effects: parts.indent.reconfigure(indentOf(i)) });
}
function convert(encoding: string) {
  const d = cur.value!;
  d.encoding = encoding;
  refreshDirty(d);
}
function setEol(e: Eol) {
  const d = cur.value!;
  d.eol = e;
  refreshDirty(d);
}
// Reading again in another encoding throws away what was typed, so it waits until that is saved or undone.
async function reopen(encoding: string) {
  const d = cur.value!;
  if (d.dirty) return toast(t('editor.reopenUnsaved'), 'warn');
  const r = await readFile(d.path, encoding).catch(() => null);
  if (!r || !view) return toast(t('editor.readFailed', { name: d.name }), 'fault');
  Object.assign(d, { saved: r.text, encoding: r.encoding, savedEncoding: r.encoding, eol: r.eol, savedEol: r.eol, etag: r.etag, readOnly: r.readOnly, cut: r.cut, dirty: false });
  view.setState(stateFor(d));
  moved();
  if (isMd.value) preview.value = md(text());
  toast(t('editor.reopened', { name: d.name, encoding: encodingName(r.encoding) }));
}

async function openMore(nodes: FileNode[], folder: string) {
  const read = await Promise.all(nodes.map(async (n) => {
    const r = await readFile(joinPath(folder, n.name)).catch(() => null);
    if (!r) toast(t('editor.readFailed', { name: n.name }), 'fault');
    return r && makeDoc(n.name, folder, r);
  }));
  add(read.filter((d): d is Doc => !!d));
}

// The plus beside the tabs offers the other texts of the folder in front, and a new file there.
async function addMenu(e: MouseEvent) {
  const btn = e.currentTarget as HTMLElement, folder = cur.value?.folder ?? '/', listing = useListing();
  if (!listing.lists.get(folder)) await listing.load(folder).catch(() => null);
  const open = new Set(docs.map((d) => d.path));
  const files = (listing.lists.get(folder)?.items ?? [])
    .filter((k) => !k.dir && editable(k) && !open.has(joinPath(folder, k.name)))
    .sort((a, b) => byName(a.name, b.name)).slice(0, 40);
  const r = btn.getBoundingClientRect();
  const items: MenuEntry[] = [
    ...(files.length ? [{ head: t('editor.openHere') }, ...files.map((k) => ({ label: k.name, g: 'IconFileText', run: () => { void openMore([k], folder); } }))] : []),
    '-', 'newFile',
  ];
  menu(r.left, r.bottom + 4, items, { anchor: btn });
}

const MORE = ['gotoLine', '-', 'duplicateLine', 'deleteLine', 'moveLineUp', 'moveLineDown', 'toggleComment', '-', 'sortLines', 'sortLinesDown', 'trimTrailing', 'upperCase', 'lowerCase', '-', 'fold', 'unfold', 'foldAll', 'unfoldAll'];
function moreMenu(e: MouseEvent) {
  const btn = e.currentTarget as HTMLElement, r = btn.getBoundingClientRect();
  menu(r.right, r.bottom + 4, MORE.map((n) => (n === '-' ? n : actionId(n))), { anchor: btn });
}

async function useVim() {
  if (!prefs.editorVim) return;
  if (props.win) props.win.locked = true;
  vim = await vimOf();
  view?.dispatch({ effects: parts.vim.reconfigure(vim) });
}

let unbind = () => {};
onMounted(() => {
  const first = props.docs[0]!;
  for (const d of props.docs) guess(d);
  docs.push(...props.docs);
  active.value = first.id;
  view = makeView(host.value!, stateFor(first));
  markTitle();
  moved();
  preview.value = isMd.value ? md(first.saved) : '';
  if (first.cut) toast(t('editor.tooLong'), 'warn');
  setTimeout(() => { view?.focus(); view?.dispatch({ selection: { anchor: 0 } }); }, 30);
  setEditorHost({ run: command, add, folder: () => cur.value?.folder ?? '/', leave });
  Object.assign(vimHooks, { save: () => save(), close: () => closeTab() });
  void useVim();
  unbind = winKeys(() => host.value?.closest('.glim-modal-backdrop') as HTMLElement | null, (e, combo) => {
    if (isMenuOpen()) return undefined;
    const target = e.target as Element | null;
    const inText = !!target?.closest?.('.cm-editor'), inField = !!target?.closest?.('input, textarea, select');
    const c = commandOf(keymap[inScope('editor', combo)] ?? '');
    if (c) {
      // Vim has the keys of the text, apart from saving and the tabs, and a field keeps those of its own text.
      if ((prefs.editorVim && inText && !c.vim) || (c.text && inField)) return undefined;
      command(c.name);
      return true;
    }
    if (combo !== 'Escape' || (prefs.editorVim && inText)) return undefined;
    if (findOpen.value) return closeFind(), true;
    return leave() ? undefined : true;
  });
});
onBeforeUnmount(() => {
  setEditorHost(null);
  unbind();
  clearTimeout(dirtyTimer);
  clearTimeout(mdTimer);
  view?.destroy();
});

function qKey(e: KeyboardEvent) {
  if (e.key !== 'Enter') return;
  e.preventDefault();
  step(e.shiftKey ? -1 : 1);
}
function repKey(e: KeyboardEvent) {
  if (e.key !== 'Enter') return;
  e.preventDefault();
  replaceOne();
}
</script>

<template>
  <div class="ce-bar">
    <div class="ce-tabs" role="tablist" :aria-label="t('editor.tabList')">
      <span v-for="d in docs" :key="d.id" class="ce-tab" :class="{ cur: d.id === active }" role="tab" :aria-selected="d.id === active" :data-tip="d.path">
        <button type="button" class="ce-tabname" @click="show(d.id)" @auxclick.prevent="(e) => e.button === 1 && closeTab(d)">{{ d.name }}<i v-if="d.dirty" class="ce-tabdot" :aria-label="t('editor.unsaved')"></i></button>
        <button type="button" class="x" :aria-label="t('editor.closeTab', { name: d.name })" @click="closeTab(d)"><G n="IconClose" /></button>
      </span>
      <IconButton id="ce-add" class="ce-addtab" :tip="t('editor.addTab')" :glyph="glyphComponent('IconAdd')" @click="addMenu" />
    </div>
    <span class="grow"></span>
    <span v-if="isMd" class="cv-swl">{{ t('editor.preview') }} <Toggle id="ce-split" v-model="opts.split" /></span>
    <Button id="ce-findbtn" :compact-below="760" :label="t('editor.find')" label-key="IconSearch" :title="t('editor.findKey')" @click="openFind()" />
    <IconButton id="ce-more" :tip="t('editor.more')" :glyph="glyphComponent('IconMore')" @click="moreMenu" />
  </div>
  <div class="ce-main" :class="{ split: isMd && opts.split }">
    <div class="ce-pane">
      <div ref="host" class="ce-scroll"></div>
      <div class="ce-find glim-card" :hidden="!findOpen">
        <div class="ce-frow">
          <TextField id="ce-q" ref="qIn" v-model="q" :placeholder="t('editor.findField')" :aria-label="t('editor.findField')" @input="search()" @keydown="qKey" />
          <Pill id="ce-n" class="glim-num">{{ count }}</Pill>
          <IconButton :tip="t('editor.prevHitTip')" :glyph="glyphComponent('IconMoveUp')" data-hit="-1" @click="step(-1)" />
          <IconButton :tip="t('editor.nextHitTip')" :glyph="glyphComponent('IconMoveDown')" data-hit="1" @click="step(1)" />
          <IconButton id="ce-fclose" :tip="t('editor.closeFindTip')" :glyph="glyphComponent('IconClose')" @click="closeFind" />
        </div>
        <div class="ce-frow">
          <TextField id="ce-r" ref="repIn" v-model="rep" :placeholder="t('editor.replaceField')" :aria-label="t('editor.replaceField')" @keydown="repKey" />
          <Button id="ce-one" :label="t('editor.replaceOne')" :label-key="null" @click="replaceOne" />
          <Button id="ce-all" :label="t('editor.replaceAll')" :label-key="null" @click="replaceAll" />
        </div>
        <div class="ce-frow ce-fopts">
          <Checkbox id="ce-case" :model-value="!!how.matchCase" :label="t('editor.matchCase')" @update:model-value="option('matchCase', $event)" />
          <Checkbox id="ce-word" :model-value="!!how.word" :label="t('editor.wholeWord')" @update:model-value="option('word', $event)" />
          <Checkbox id="ce-regex" :model-value="!!how.regex" :label="t('editor.regex')" @update:model-value="option('regex', $event)" />
        </div>
      </div>
    </div>
    <div v-if="isMd" class="ce-preview"><article class="cv-md" v-html="preview"></article></div>
  </div>
  <StatusBar
    v-if="cur" :doc="cur" :line="pos.line" :col="pos.col" :chosen="pos.chosen" :wrap="!!prefs.editorWrap"
    @lang="setLang" @convert="convert" @reopen="reopen" @eol="setEol" @indent="setIndent" @wrap="setWrap" @goto="gotoLine"
  />
</template>
