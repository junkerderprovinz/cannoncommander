<script setup lang="ts">
// The search window: every criterion Krusader's search has, with hits that follow each keystroke. Names and fields
// come from GET /search at once; a text inside files is a contentSearch job whose hits arrive one batch at a time.
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import { ApiError } from '@/api/client';
import { type Job, ops } from '@/api/ops';
import { on } from '@/api/sse';
import { type ContentHits, type Hit, type SearchQuery, toHit, tools } from '@/api/tools';
import { fmtDate, fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import { prefs } from '@/core/prefs';
import type { WinEntry } from '@/core/windows';
import { num, plural, setFoot } from '@/bk/kit';
import PathPick from '@/bk/PathPick.vue';
import Tile from '@/bk/Tile.vue';
import { t } from '@/i18n';
import { useJobs } from '@/stores/jobs';
import { useSearch } from '@/stores/search';
import Dropdown from '@/ui/Dropdown.vue';
import Frow from '@/ui/Frow.vue';
import Seg from '@/ui/Seg.vue';
import Sw from '@/ui/Sw.vue';
import { openHit, saveSearch } from './saved';

const LIMIT = 400;
const props = defineProps<{ q: SearchQuery; win: WinEntry; close: () => void }>();
const q = reactive(props.q);

const TYPES = computed(() => (['all', 'dir', 'img', 'video', 'doc', 'arch'] as const).map((v) => [v, t(`search.types.${v}`)] as const));
const WHEN = computed(() => (['any', '1', '7', '30', '365', 'old'] as const).map((v) => [v, t(`search.whens.${v}`)] as const));

interface Result { hits: Hit[]; total: number; scanned: number; found: boolean; unknown: string[]; reading: boolean }
const res = ref<Result>({ hits: [], total: 0, scanned: 0, found: true, unknown: [], reading: false });
const cur = ref(0);
const list = ref<HTMLElement | null>(null);

const empty = computed(() => !q.name.trim() && !q.text.trim() && !q.min && !q.max && q.when === 'any' && q.type === 'all');
const shown = computed(() => res.value.hits.slice(0, LIMIT));
const count = computed(() => {
  const r = res.value;
  if (!r.found || r.unknown.length) return t('search.hits');
  return r.total > LIMIT ? t('search.first', { n: LIMIT, total: num(r.total) }) : plural(r.total, t('search.hitsLabel'), t('search.hitsLabel'));
});

// Folders first, then names in the order people count, as the server sorts its hits.
const order = (a: Hit, b: Hit) => Number(b.dir) - Number(a.dir) || a.name.localeCompare(b.name, 'de', { numeric: true });

let stop: AbortController | null = null;
let textJob: Job | null = null;
const dropTextJob = () => {
  if (textJob) ops.jobStep(textJob.id, 'cancel').catch(() => {});
  textJob = null;
};
const offHits = on('search.hits', (e) => {
  const got = e.data as ContentHits;
  if (got.job !== textJob?.id) return;
  const r = res.value;
  res.value = { ...r, hits: [...r.hits, ...got.items.map(toHit)].sort(order), total: got.total, scanned: got.scanned, reading: !got.done };
});

async function run() {
  useSearch().last = { ...q };
  stop?.abort();
  dropTextJob();
  cur.value = 0;
  if (empty.value) { res.value = { hits: [], total: 0, scanned: 0, found: true, unknown: [], reading: false }; return; }
  const mine = (stop = new AbortController());
  try {
    if (q.text.trim()) {
      // The criteria that need no file content go first, so a mistake in them shows before any file is read.
      const check = await tools.find({ ...q, text: '' }, 1, mine.signal);
      if (mine.signal.aborted) return;
      if (check.unknown.length) { res.value = { hits: [], total: 0, scanned: 0, found: true, unknown: check.unknown, reading: false }; return; }
      res.value = { hits: [], total: 0, scanned: 0, found: true, unknown: [], reading: true };
      const job = await tools.findText(q);
      if (mine.signal.aborted) { ops.jobStep(job.id, 'cancel').catch(() => {}); return; }
      textJob = job;
      useJobs().whenDone(job, async (j) => {
        if (textJob?.id !== j.id) return;
        const result = (j.result ?? (await tools.job(j.id).catch(() => j)).result) as { items?: Parameters<typeof toHit>[0][]; total?: number; scanned?: number } | undefined;
        if (textJob?.id !== j.id) return;
        const r = res.value;
        res.value = result?.items ? { ...r, hits: result.items.map(toHit), total: result.total ?? result.items.length, scanned: result.scanned ?? r.scanned, reading: false } : { ...r, reading: false };
        textJob = null;
      });
      return;
    }
    const got = await tools.find(q, LIMIT, mine.signal);
    if (mine.signal.aborted) return;
    res.value = { hits: got.unknown.length ? [] : got.items, total: got.unknown.length ? 0 : got.total, scanned: got.scanned, found: true, unknown: got.unknown, reading: false };
  } catch (e) {
    if (mine.signal.aborted) return;
    res.value = { hits: [], total: 0, scanned: 0, found: !(e instanceof ApiError && e.status === 404), unknown: [], reading: false };
  }
}

let timer: ReturnType<typeof setTimeout> | undefined;
watch(q, () => { clearTimeout(timer); timer = setTimeout(run, 120); });

// The window stays as short as the form until there is something to list, then grows down to its full height.
let drawn = false;
watch(empty, async (none) => {
  const box = props.win.el?.querySelector<HTMLElement>('.win');
  const from = box?.offsetHeight ?? 0;
  props.win.cls = none ? 'bk bk-search' : 'bk bk-search bk-open';
  await nextTick();
  if (!box || !drawn || prefs.motion === 'off' || matchMedia('(prefers-reduced-motion: reduce)').matches) return;
  box.animate([{ height: from + 'px' }, { height: box.offsetHeight + 'px' }], { duration: 240, easing: 'cubic-bezier(.2, .8, .2, 1)' });
}, { immediate: true });

const go = (asTab: boolean) => {
  const hit = shown.value[cur.value];
  if (!hit) return false;
  void openHit(hit, asTab);
  return true;
};
watch([() => res.value.hits.length, empty, () => res.value.scanned, () => res.value.reading], () => {
  const none = !res.value.hits.length, r = res.value;
  const start = empty.value ? '' : r.reading ? t('search.reading', { n: num(r.scanned) }) : r.found && !r.unknown.length ? t('search.scanned', { n: num(r.scanned) }) : '';
  setFoot(props.win, start, [
    { label: t('search.asTab'), g: 'IconAdd', disabled: none, run: () => go(true) },
    { label: t('search.openFolder'), g: 'IconFolderOpen', primary: true, disabled: none, run: () => go(false) },
  ]);
}, { immediate: true });

function move(d: number) {
  if (!res.value.hits.length) return;
  cur.value = Math.max(0, Math.min(shown.value.length - 1, cur.value + d));
  nextTick(() => list.value?.querySelector(`.bk-hit[data-bk-hit="${cur.value}"]`)?.scrollIntoView({ block: 'nearest' }));
}

// A single click on a hit picks it; a second one on the same hit goes there.
let lastClick = { i: -1, at: 0 };
function clickHit(e: MouseEvent, i: number) {
  if (lastClick.i === i && e.timeStamp - lastClick.at < 450) {
    void openHit(shown.value[i]!, false);
    props.close();
    return;
  }
  lastClick = { i, at: e.timeStamp };
  cur.value = i;
  move(0);
}

function key(e: KeyboardEvent) {
  if (e.key === 'ArrowDown') { e.preventDefault(); move(1); }
  if (e.key === 'ArrowUp') { e.preventDefault(); move(-1); }
  if (e.key === 'Enter' && (e.target as Element).closest('input')) {
    e.preventDefault();
    props.win.el?.querySelector<HTMLElement>('.win-foot .btn.primary')?.click();
  }
}

const nameField = ref<HTMLInputElement | null>(null);
onMounted(() => {
  void run().then(() => { drawn = true; });
  setTimeout(() => { nameField.value?.focus(); nameField.value?.select(); }, 0);
});
onBeforeUnmount(() => { stop?.abort(); dropTextJob(); offHits(); clearTimeout(timer); });
</script>

<template>
  <div class="well bk-form" @keydown="key">
    <div class="bk-grid2">
      <Frow :label="t('search.name')" :tip="t('search.nameTip')"><input id="s-name" ref="nameField" v-model="q.name" class="field grow" :placeholder="t('search.namePlaceholder')" autocomplete="off" spellcheck="false"></Frow>
      <Frow :label="t('search.text')" :tip="t('search.textTip')"><input id="s-text" v-model="q.text" class="field grow" :placeholder="t('search.textPlaceholder')" autocomplete="off"></Frow>
      <Frow :label="t('search.in')"><PathPick id="s-in" v-model="q.in" /></Frow>
      <Frow :label="t('search.hidden')" :tip="t('search.hiddenTip')"><Sw id="s-hidden" v-model="q.hidden" /></Frow>
      <Frow :label="t('search.size')" :tip="t('search.sizeTip')"><span class="bk-range"><input id="s-min" v-model="q.min" class="field bk-num glim-num" :placeholder="t('search.from')" autocomplete="off"><span class="muted">{{ t('search.to') }}</span><input id="s-max" v-model="q.max" class="field bk-num glim-num" :placeholder="t('search.any')" autocomplete="off"></span></Frow>
      <Frow :label="t('search.when')"><Dropdown id="s-when" v-model="q.when" :options="WHEN" size="grow" /></Frow>
    </div>
    <Frow :label="t('search.type')"><Seg id="s-type" v-model="q.type" :options="TYPES" /></Frow>
  </div>
  <div class="well bk-fill bk-hits" :hidden="empty" @keydown="key">
    <div class="bk-tools"><span id="s-count" class="eyebrow">{{ count }}</span><button id="s-save" class="btn" :data-tip="t('search.saveTip')" @click="saveSearch({ ...q })"><G n="IconSave" />{{ t('search.save') }}</button></div>
    <div class="bk-hhead"><span></span><span>{{ t('search.colName') }}</span><span>{{ t('search.colFolder') }}</span><span class="r">{{ t('search.colSize') }}</span><span class="r">{{ t('search.colChanged') }}</span></div>
    <div id="s-list" ref="list" class="bk-list" role="listbox" :aria-label="t('search.hitsLabel')">
      <template v-if="!empty">
        <div v-if="!res.found" class="empty"><G n="IconFolder" /><div>{{ t('search.noFolder') }}</div></div>
        <div v-else-if="res.unknown.length" class="empty"><G n="IconInfo" /><div>{{ t('search.unknown', { fields: res.unknown.map((u) => `„${u}:“`).join(', ') }) }}</div><div class="muted" style="font-size:12px">{{ t('search.unknownHint') }}</div></div>
        <template v-else-if="shown.length">
          <div v-for="(h, i) in shown" :key="h.at + '/' + h.name" class="bk-row bk-hit" :class="{ cur: i === cur }" :data-bk-hit="i" role="option" :aria-selected="i === cur" @click="clickHit($event, i)">
            <Tile :n="h" /><span class="bk-name">{{ h.name }}</span><span class="bk-path muted mono" :title="h.at || '/'"><bdi>{{ h.at || '/' }}</bdi></span>
            <span class="pill glim-num">{{ fmtSize(h.size) || '0 B' }}</span><span class="pill glim-num">{{ fmtDate(h.time) }}</span>
          </div>
        </template>
        <div v-else-if="!res.reading" class="empty"><G n="IconFilter" /><div>{{ t('search.nothing') }}</div><div class="muted" style="font-size:12px">{{ t('search.nothingHint') }}</div></div>
      </template>
    </div>
  </div>
</template>
