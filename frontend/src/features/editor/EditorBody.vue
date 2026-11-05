<script setup lang="ts">
// The editor window's body: CodeMirror in the prototype's look, a find and replace card over it and, for markdown,
// a live preview beside it.
import { EditorView } from '@codemirror/view';
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue';
import { G } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { saveText, type TextFile } from '@/features/viewer/content';
import { type FileNode, joinPath, langOf, viewKind } from '@/features/viewer/kinds';
import { esc, md } from '@/features/viewer/markdown';
import Sw from '@/ui/Sw.vue';
import { footStartOf, notchOf, winKeys } from '@/features/viewer/winkeys';
import { t } from '@/i18n';
import { useListing } from '@/stores/listing';
import { makeEditor, setMatches } from './cm';
import { findAll, nearest, position, type Range } from './find';
import { opts, type EditorState } from './state';

// The text comes read already, so the editor stands ready the moment its window opens.
const props = defineProps<{ node: FileNode; folder: string; file: Pick<TextFile, 'text' | 'encoding' | 'eol' | 'etag' | 'truncated'>; state: EditorState; close?: () => void }>();

const path = joinPath(props.folder, props.node.name);
const isMd = viewKind(props.node) === 'md';
const host = ref<HTMLElement | null>(null);
const findOpen = ref(false);
const q = ref('');
const rep = ref('');
const qIn = ref<HTMLInputElement | null>(null);
const repIn = ref<HTMLInputElement | null>(null);
const count = ref('');
const preview = ref('');
let view: EditorView | null = null;
let setWrap = (_on: boolean) => {};
let saved = props.file.text;
const how: { encoding?: string; eol?: string; etag?: string } = { encoding: props.file.encoding, eol: props.file.eol, etag: props.file.etag };
let matches: Range[] = [];
const cut = !!props.file.truncated;
let hit = -1;

const text = () => view?.state.doc.toString() ?? '';

function setDirty(d: boolean) {
  props.state.dirty = d;
  const notch = notchOf(host.value);
  if (notch) notch.innerHTML = esc(props.node.name) + (d ? `<i class="ce-dot" aria-label="${esc(t('editor.unsaved'))}"></i>` : '');
}

function moved() {
  if (!view) return;
  const s = view.state.selection.main, at = position(text(), s.head);
  const el = footStartOf(host.value);
  if (el) el.textContent = t('editor.pos', { line: at.line, col: at.col }) + (s.to > s.from ? ', ' + t('editor.marked', { n: s.to - s.from }) : '');
}

function search(keep = false) {
  if (!view) return;
  matches = findAll(text(), q.value);
  if (!keep) hit = nearest(matches, view.state.selection.main.from);
  hit = Math.min(hit, matches.length - 1);
  count.value = q.value ? (matches.length ? t('editor.hits', { i: hit + 1, n: matches.length }) : t('editor.noHits')) : '';
  view.dispatch({ effects: setMatches.of({ ranges: matches, hit }) });
  if (hit >= 0) view.dispatch({ effects: EditorView.scrollIntoView(matches[hit]![0], { y: 'center' }) });
}
function step(d: number) {
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
  const el = replace ? repIn.value : qIn.value;
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
  if (hit < 0 || !view) return;
  const [a, b] = matches[hit]!;
  view.dispatch({ changes: { from: a, to: b, insert: rep.value }, selection: { anchor: a + rep.value.length } });
  search();
}
function replaceAll() {
  if (!matches.length || !view) return;
  const n = matches.length;
  view.dispatch({ changes: matches.map(([from, to]) => ({ from, to, insert: rep.value })) });
  search();
  toast(t(n === 1 ? 'editor.replacedOne' : 'editor.replaced', { n }));
}

let mdTimer: ReturnType<typeof setTimeout> | undefined;
function changed() {
  setDirty(text() !== saved);
  if (findOpen.value) search(true);
  if (isMd) {
    clearTimeout(mdTimer);
    mdTimer = setTimeout(() => { preview.value = md(text()); }, 120);
  }
}

async function save() {
  // Saving only the part that was read would cut the file short.
  if (cut) return toast(t('editor.tooLong'), 'warn'), false;
  const now = text();
  try {
    const got = await saveText(path, now, how);
    how.etag = got.etag;
    saved = now;
    setDirty(text() !== saved);
    toast(t('editor.saved', { name: props.node.name }));
    const listing = useListing();
    listing.invalidate(props.folder);
    listing.load(props.folder).catch(() => {});
    return true;
  } catch (e) {
    toast((e as { status?: number }).status === 412 ? t('editor.conflict') : t('editor.saveFailed', { name: props.node.name }), 'fault');
    return false;
  }
}
props.state.save = save;

function setWrapOpt(on: boolean) {
  opts.wrap = on;
  setWrap(on);
}

let unbind = () => {};
onMounted(() => {
  if (cut) toast(t('editor.tooLong'), 'warn');
  const made = makeEditor(host.value!, saved, props.node.name, { changed, moved });
  view = made.view;
  setWrap = made.setWrap;
  setWrap(opts.wrap);
  preview.value = isMd ? md(saved) : '';
  setDirty(false);
  moved();
  setTimeout(() => { view?.focus(); view?.dispatch({ selection: { anchor: 0 } }); }, 30);
  unbind = winKeys(() => host.value?.closest('.glim-modal-backdrop') as HTMLElement | null, (e, combo) => {
    if (combo === 'Ctrl+s') return save(), true;
    if (combo === 'Ctrl+f') return openFind(), true;
    if (combo === 'Ctrl+h') return openFind(true), true;
    if (combo === 'F3' || combo === 'Shift+F3') return step(e.shiftKey ? -1 : 1), true;
    if (combo === 'Escape' && findOpen.value) return closeFind(), true;
    if (combo === 'Escape' && props.state.dirty) return props.state.ask(), true;
    return undefined;
  });
});
onBeforeUnmount(() => {
  unbind();
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
    <span class="pill">{{ langOf(node) }}</span><span class="grow"></span>
    <span v-if="isMd" class="cv-swl">{{ t('editor.preview') }} <Sw id="ce-split" v-model="opts.split" /></span>
    <span class="cv-swl">{{ t('editor.wrap') }} <Sw id="ce-wrap" :model-value="opts.wrap" @update:model-value="setWrapOpt" /></span>
    <button id="ce-findbtn" class="btn" data-tip="Strg+F" @click="openFind()"><G n="IconSearch" />{{ t('editor.find') }}</button>
  </div>
  <div class="ce-main" :class="{ split: isMd && opts.split }">
    <div class="ce-pane">
      <div ref="host" class="ce-scroll" :class="{ wrap: opts.wrap }"></div>
      <div class="ce-find glim-card" :hidden="!findOpen">
        <div class="ce-frow">
          <input id="ce-q" ref="qIn" v-model="q" class="glim-input" :placeholder="t('editor.findField')" :aria-label="t('editor.findField')" @input="search()" @keydown="qKey">
          <span id="ce-n" class="pill glim-num">{{ count }}</span>
          <button class="btn icon" data-hit="-1" :data-tip="t('editor.prevHitTip')" :aria-label="t('editor.prevHit')" @click="step(-1)"><G n="IconMoveUp" /></button>
          <button class="btn icon" data-hit="1" :data-tip="t('editor.nextHitTip')" :aria-label="t('editor.nextHit')" @click="step(1)"><G n="IconMoveDown" /></button>
          <button id="ce-fclose" class="btn icon" :data-tip="t('editor.closeFindTip')" :aria-label="t('editor.closeFind')" @click="closeFind"><G n="IconClose" /></button>
        </div>
        <div class="ce-frow">
          <input id="ce-r" ref="repIn" v-model="rep" class="glim-input" :placeholder="t('editor.replaceField')" :aria-label="t('editor.replaceField')" @keydown="repKey">
          <button id="ce-one" class="btn" @click="replaceOne">{{ t('editor.replaceOne') }}</button>
          <button id="ce-all" class="btn" @click="replaceAll">{{ t('editor.replaceAll') }}</button>
        </div>
      </div>
    </div>
    <div v-if="isMd" class="ce-preview"><article class="cv-md" v-html="preview"></article></div>
  </div>
</template>
