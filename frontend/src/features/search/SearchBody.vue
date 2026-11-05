<script setup lang="ts">
// The search window: every criterion Krusader's search has, with hits that follow each keystroke. Names and fields
// come from GET /search at once; a text inside files is a contentSearch job whose hits arrive one batch at a time.
import { pathField } from '@/ui/picker';
import { Select } from '@glimstone/vue/Select';
import { Button, EmptyState, FieldRow, FieldRowGroup, GlyphTile, PathField, Pill, Selector, SubCard, TextField, Toggle } from '@glimstone/vue';
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import { ApiError } from '@/api/client';
import { type Job, ops } from '@/api/ops';
import { on } from '@/api/sse';
import { type ContentHits, type Hit, type SearchQuery, toHit, tools } from '@/api/tools';
import { fmtDate, fmtSize } from '@/core/format';
import { glyphComponent, glyphOf } from '@/core/glyphs';
import { prefs } from '@/core/prefs';
import type { WinEntry } from '@/core/windows';
import { t } from '@/i18n';
import { useJobs } from '@/stores/jobs';
import { useSearch } from '@/stores/search';
import { num, plural, setFoot, segItems, selectOptions } from '@/ui/kit';
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
  return r.total > LIMIT ? t('search.first', { n: LIMIT, total: num(r.total) }) : plural(r.total, 'search.hitCount');
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
    props.win.el?.querySelector<HTMLElement>('.glim-modal-foot .glim-btn--accent')?.click();
  }
}

const nameField = ref<InstanceType<typeof TextField> | null>(null);
onMounted(() => {
  void run().then(() => { drawn = true; });
  setTimeout(() => { nameField.value?.input?.focus(); nameField.value?.input?.select(); }, 0);
});
onBeforeUnmount(() => { stop?.abort(); dropTextJob(); offHits(); clearTimeout(timer); });
</script>

<template>
  <SubCard class="bk-form" @keydown="key">
    <FieldRowGroup :columns="2">
      <FieldRow :label="t('search.name')" :hint="t('search.nameTip')"><TextField id="s-name" ref="nameField" v-model="q.name" :placeholder="t('search.namePlaceholder')" autocomplete="off" spellcheck="false" width="grow" /></FieldRow>
      <FieldRow :label="t('search.text')" :hint="t('search.textTip')"><TextField id="s-text" v-model="q.text" :placeholder="t('search.textPlaceholder')" autocomplete="off" width="grow" /></FieldRow>
      <FieldRow :label="t('search.in')"><PathField v-model="q.in" v-bind="pathField('s-in')" /></FieldRow>
      <FieldRow :label="t('search.hidden')" :hint="t('search.hiddenTip')"><Toggle id="s-hidden" v-model="q.hidden" /></FieldRow>
      <FieldRow :label="t('search.size')" :hint="t('search.sizeTip')"><span class="bk-range"><TextField id="s-min" v-model="q.min" class="bk-num glim-num" :placeholder="t('search.from')" autocomplete="off" /><span class="muted">{{ t('search.to') }}</span><TextField id="s-max" v-model="q.max" class="bk-num glim-num" :placeholder="t('search.any')" autocomplete="off" /></span></FieldRow>
      <FieldRow :label="t('search.when')"><Select id="s-when" v-model="q.when" :trigger-attrs="{ 'data-val': q.when }" :options="selectOptions(WHEN)" rows="menu" /></FieldRow>
      <FieldRow wide :label="t('search.type')"><Selector id="s-type" v-model="q.type" :items="segItems(TYPES)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    </FieldRowGroup>
  </SubCard>
  <SubCard class="bk-fill bk-hits" :hidden="empty" @keydown="key">
    <div class="bk-tools"><span id="s-count" class="glim-eyebrow">{{ count }}</span><Button id="s-save" :label="t('search.save')" label-key="IconSave" :title="t('search.saveTip')" @click="saveSearch({ ...q })" /></div>
    <div class="bk-hhead"><span></span><span>{{ t('search.colName') }}</span><span>{{ t('search.colFolder') }}</span><span class="r">{{ t('search.colSize') }}</span><span class="r">{{ t('search.colChanged') }}</span></div>
    <div id="s-list" ref="list" class="bk-list" role="listbox" :aria-label="t('search.hitsLabel')">
      <template v-if="!empty">
        <EmptyState v-if="!res.found" nested :glyph="glyphComponent('IconFolder')" :title="t('search.noFolder')" />
        <EmptyState v-else-if="res.unknown.length" nested :glyph="glyphComponent('IconInfo')" :title="t('search.unknown', { fields: res.unknown.map((u) => `„${u}:“`).join(', ') })" :hint="t('search.unknownHint')" />
        <template v-else-if="shown.length">
          <div v-for="(h, i) in shown" :key="h.at + '/' + h.name" class="bk-row bk-hit" :class="{ cur: i === cur }" :data-bk-hit="i" role="option" :aria-selected="i === cur" @click="clickHit($event, i)">
            <GlyphTile :glyph="glyphComponent(glyphOf(h))" ground :quiet="!h.dir" /><span class="bk-name">{{ h.name }}</span><span class="bk-path muted mono" :title="h.at || '/'"><bdi>{{ h.at || '/' }}</bdi></span>
            <Pill class="glim-num">{{ fmtSize(h.size) || '0 B' }}</Pill><Pill class="glim-num">{{ fmtDate(h.time) }}</Pill>
          </div>
        </template>
        <EmptyState v-else-if="!res.reading" nested :glyph="glyphComponent('IconFilter')" :title="t('search.nothing')" :hint="t('search.nothingHint')" />
      </template>
    </div>
  </SubCard>
</template>
