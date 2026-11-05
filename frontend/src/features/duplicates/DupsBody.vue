<script setup lang="ts">
// Doppelte Dateien: the server's duplicates job groups true copies, and the copies picked here go to the trash.
import { pathField } from '@/ui/picker';
import { Button, EmptyState, FieldRow, FieldRowGroup, GlyphTile, PathField, Pill, ProgressBar, Selector, SubCard, Toggle } from '@glimstone/vue';
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { type Job, ops } from '@/api/ops';
import { type DupCopy, type Duplicates, tools } from '@/api/tools';
import { fmtSize, fmtTime } from '@/core/format';
import { glyphComponent, glyphOf } from '@/core/glyphs';
import { prefs } from '@/core/prefs';
import { toast } from '@/core/toast';
import type { WinEntry } from '@/core/windows';
import { t } from '@/i18n';
import { pctOf } from '@/features/jobs/label';
import { undoAct } from '@/features/undo/act';
import { useJobs } from '@/stores/jobs';
import { useListing } from '@/stores/listing';
import { num, plural, setFoot, segItems } from '@/ui/kit';

const props = defineProps<{ root: string; win: WinEntry; close: () => void }>();
const jobs = useJobs();
const where = ref(props.root);
const root = ref(props.root);
const by = ref<'name' | 'size'>('name');
const res = ref<Duplicates | null>(null);
const missing = ref(false);
const job = ref<Job | null>(null);
const drop = ref(new Set<string>());

const BY = computed(() => [['name', t('duplicates.byName')], ['size', t('duplicates.bySize')]] as const);
const live = computed(() => (job.value ? jobs.find(job.value.id) ?? job.value : null));
const pct = computed(() => (live.value ? pctOf(live.value) : 0));
const seen = computed(() => live.value?.doneFiles ?? 0);

const all = computed(() => res.value?.groups.flatMap((g) => g.copies.map((c) => ({ c, size: g.size }))) ?? []);
const picked = computed(() => all.value.filter((x) => drop.value.has(x.c.path)));
const freed = computed(() => picked.value.reduce((a, x) => a + x.size, 0));
const sum = computed(() => (res.value?.groups.length
  ? t('duplicates.sum', { groups: plural(res.value.groups.length, 'duplicates.groupCount'), size: fmtSize(res.value.wasted) })
  : t('duplicates.groups')));
const copies = (n: number) => plural(n, 'duplicates.copies');

const stopJob = () => {
  if (job.value && live.value && ['queued', 'running', 'paused'].includes(live.value.state)) ops.jobStep(job.value.id, 'cancel').catch(() => {});
  job.value = null;
};

async function start() {
  stopJob();
  res.value = null;
  missing.value = false;
  drop.value = new Set();
  const target = where.value.trim() || '/';
  if (!(await useListing().load(target).catch(() => null))) { missing.value = true; return; }
  root.value = target;
  try {
    const started = await tools.duplicates(target, by.value, !!prefs.showHidden);
    job.value = started;
    jobs.whenDone(started, async (j) => {
      if (job.value?.id !== j.id) return;
      if (j.state !== 'done') {
        if (j.state === 'failed') toast(t('duplicates.failed', { error: j.error || j.state }), 'fault');
        return;
      }
      const result = (j.result ?? (await tools.job(j.id).catch(() => j)).result) as Duplicates | undefined;
      if (job.value?.id !== j.id) return;
      res.value = result ?? { groups: [], wasted: 0, files: 0 };
    });
  } catch (e) {
    toast((e as Error).message, 'warn');
  }
}

let typing: ReturnType<typeof setTimeout> | undefined;
watch(where, () => { clearTimeout(typing); typing = setTimeout(start, 400); });
watch(by, start);

function keep(mode: 'new' | 'old' | 'none') {
  const next = new Set<string>();
  if (mode !== 'none') {
    for (const g of res.value?.groups ?? []) g.copies.forEach((c, i) => { if (mode === 'new' ? i > 0 : i < g.copies.length - 1) next.add(c.path); });
  }
  drop.value = next;
}
function toggle(gi: number, c: DupCopy, on: boolean) {
  const g = res.value!.groups[gi]!, next = new Set(drop.value);
  if (on) {
    if (g.copies.every((x) => x === c || next.has(x.path))) { toast(t('duplicates.oneStays'), 'warn'); return; }
    next.add(c.path);
  } else next.delete(c.path);
  drop.value = next;
}

async function remove() {
  const victims = picked.value;
  if (!victims.length) return;
  const size = freed.value, paths = victims.map((x) => x.c.path);
  try {
    const started = await jobs.start({ kind: 'trash', params: { paths } });
    jobs.whenDone(started, (j) => {
      if (j.state !== 'done') return;
      const gone = new Set(paths);
      if (res.value) {
        const groups = res.value.groups.map((g) => ({ ...g, copies: g.copies.filter((c) => !gone.has(c.path)) })).filter((g) => g.copies.length > 1);
        res.value = { ...res.value, groups, wasted: groups.reduce((a, g) => a + g.wasted, 0) };
      }
      drop.value = new Set();
      toast(t('duplicates.removed', { copies: copies(victims.length), size: fmtSize(size) }), 'ok', j.undo ? undoAct(j.undo) : undefined);
    });
  } catch (e) {
    toast((e as Error).message, 'warn');
  }
}

watch([picked, freed], () => {
  const n = picked.value.length;
  setFoot(props.win, t('duplicates.start'), [{
    label: n ? t('duplicates.removeSome', { copies: copies(n), size: fmtSize(freed.value) }) : t('duplicates.remove'),
    g: 'IconTrash', primary: true, disabled: !n, run: () => { void remove(); return false; },
  }]);
}, { immediate: true });

void start();
onBeforeUnmount(() => { clearTimeout(typing); stopJob(); });
</script>

<template>
  <SubCard class="bk-form">
    <FieldRowGroup class="bk-fields2">
      <FieldRow :label="t('duplicates.in')"><PathField v-model="where" v-bind="pathField('u-in')" /></FieldRow>
      <FieldRow :label="t('duplicates.by')" :hint="t('duplicates.byTip')"><Selector id="u-by" v-model="by" :items="segItems(BY)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    </FieldRowGroup>
  </SubCard>
  <SubCard class="bk-fill bk-dupwell">
    <div class="bk-tools"><span id="u-sum" class="glim-eyebrow">{{ sum }}</span>
      <Button :label="t('duplicates.keepNew')" label-key="IconLatest" data-bk-keep="new" :disabled="!res?.groups.length" @click="keep('new')" />
      <Button :label="t('duplicates.keepOld')" label-key="IconFirst" data-bk-keep="old" :disabled="!res?.groups.length" @click="keep('old')" />
      <Button :label="t('duplicates.keepAll')" label-key="IconClearSelection" data-bk-keep="none" :disabled="!res?.groups.length" @click="keep('none')" />
    </div>
    <div id="u-list" class="bk-list">
      <EmptyState v-if="missing" nested :glyph="glyphComponent('IconFolder')" :title="t('duplicates.noFolder')" />
      <div v-else-if="!res" class="bk-scan"><div>{{ t('duplicates.scanning', { path: root }) }}</div><ProgressBar id="u-bar" class="bar" :value="pct / 100" :label="t('duplicates.scanning', { path: root })" /><div id="u-n" class="glim-num">{{ t('duplicates.files', { n: num(seen) }) }}</div></div>
      <template v-else-if="res.groups.length">
        <section v-for="(g, gi) in res.groups" :key="g.sha256 + gi" class="bk-group">
          <div class="bk-ghead"><GlyphTile :glyph="glyphComponent(glyphOf({ name: g.name }))" ground quiet /><span class="bk-name">{{ by === 'name' ? g.name : t('duplicates.sameSize', { n: g.copies.length }) }}</span><Pill class="glim-num">{{ t('duplicates.each', { copies: copies(g.copies.length), size: fmtSize(g.size) }) }}</Pill></div>
          <div v-for="(c, ci) in g.copies" :key="c.path" class="bk-copy" :class="{ gone: drop.has(c.path) }">
            <span class="bk-path mono" :title="c.path"><bdi>{{ by === 'name' ? c.at : c.path }}</bdi></span>
            <span v-if="g.copies[0]!.mtime === g.copies.at(-1)!.mtime"></span><Pill v-else-if="ci === 0">{{ t('duplicates.newest') }}</Pill><Pill v-else-if="ci === g.copies.length - 1">{{ t('duplicates.oldest') }}</Pill><span v-else></span>
            <Pill class="glim-num">{{ fmtTime(Date.parse(c.mtime)) }}</Pill>
            <span class="bk-swl"><Toggle :model-value="drop.has(c.path)" :data-bk-dup="`${gi}:${ci}`" :label="t('duplicates.dropLabel', { path: c.path })" :data-tip="t('duplicates.toTrash')" hide-label @update:model-value="(v: boolean) => toggle(gi, c, v)" /></span>
          </div>
        </section>
      </template>
      <EmptyState v-else nested :glyph="glyphComponent('IconCheck')" :title="t('duplicates.none')" :hint="t('duplicates.scanned', { n: num(res.files) })" />
    </div>
  </SubCard>
</template>
