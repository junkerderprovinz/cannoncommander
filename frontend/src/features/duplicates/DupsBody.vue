<script setup lang="ts">
// Doppelte Dateien: the server's duplicates job groups true copies, and the copies picked here go to the trash.
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { type Job, ops } from '@/api/ops';
import { type DupCopy, type Duplicates, tools } from '@/api/tools';
import { fmtSize, fmtTime } from '@/core/format';
import { G } from '@/core/glyphs';
import { prefs } from '@/core/prefs';
import { toast } from '@/core/toast';
import type { WinEntry } from '@/core/windows';
import { num, plural, setFoot } from '@/bk/kit';
import PathPick from '@/bk/PathPick.vue';
import Tile from '@/bk/Tile.vue';
import { t } from '@/i18n';
import { pctOf } from '@/features/jobs/label';
import { undoAct } from '@/features/undo/act';
import { useJobs } from '@/stores/jobs';
import { useListing } from '@/stores/listing';
import Frow from '@/ui/Frow.vue';
import Seg from '@/ui/Seg.vue';
import Sw from '@/ui/Sw.vue';

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
  ? t('duplicates.sum', { groups: plural(res.value.groups.length, t('duplicates.group'), t('duplicates.groupsWord')), size: fmtSize(res.value.wasted) })
  : t('duplicates.groups')));
const copies = (n: number) => plural(n, t('duplicates.copy'), t('duplicates.copies'));

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
  <div class="well bk-form"><div class="bk-grid2">
    <Frow :label="t('duplicates.in')"><PathPick id="u-in" v-model="where" /></Frow>
    <Frow :label="t('duplicates.by')" :tip="t('duplicates.byTip')"><Seg id="u-by" v-model="by" :options="BY" /></Frow>
  </div></div>
  <div class="well bk-fill bk-dupwell">
    <div class="bk-tools"><span id="u-sum" class="eyebrow">{{ sum }}</span>
      <button class="btn" data-bk-keep="new" :disabled="!res?.groups.length" @click="keep('new')"><G n="IconLatest" />{{ t('duplicates.keepNew') }}</button>
      <button class="btn" data-bk-keep="old" :disabled="!res?.groups.length" @click="keep('old')"><G n="IconFirst" />{{ t('duplicates.keepOld') }}</button>
      <button class="btn" data-bk-keep="none" :disabled="!res?.groups.length" @click="keep('none')"><G n="IconClearSelection" />{{ t('duplicates.keepAll') }}</button>
    </div>
    <div id="u-list" class="bk-list">
      <div v-if="missing" class="empty"><G n="IconFolder" /><div>{{ t('duplicates.noFolder') }}</div></div>
      <div v-else-if="!res" class="bk-scan"><div>{{ t('duplicates.scanning', { path: root }) }}</div><div class="bar"><i id="u-bar" :style="{ width: pct + '%' }"></i></div><div id="u-n" class="glim-num">{{ t('duplicates.files', { n: num(seen) }) }}</div></div>
      <template v-else-if="res.groups.length">
        <section v-for="(g, gi) in res.groups" :key="g.sha256 + gi" class="bk-group">
          <div class="bk-ghead"><Tile :n="{ name: g.name }" /><span class="bk-name">{{ by === 'name' ? g.name : t('duplicates.sameSize', { n: g.copies.length }) }}</span><span class="pill glim-num">{{ t('duplicates.each', { copies: copies(g.copies.length), size: fmtSize(g.size) }) }}</span></div>
          <div v-for="(c, ci) in g.copies" :key="c.path" class="bk-copy" :class="{ gone: drop.has(c.path) }">
            <span class="bk-path mono" :title="c.path"><bdi>{{ by === 'name' ? c.at : c.path }}</bdi></span>
            <span v-if="g.copies[0]!.mtime === g.copies.at(-1)!.mtime"></span><span v-else-if="ci === 0" class="pill">{{ t('duplicates.newest') }}</span><span v-else-if="ci === g.copies.length - 1" class="pill">{{ t('duplicates.oldest') }}</span><span v-else></span>
            <span class="pill glim-num">{{ fmtTime(Date.parse(c.mtime)) }}</span>
            <span class="bk-swl"><Sw :model-value="drop.has(c.path)" :data-bk-dup="`${gi}:${ci}`" :aria-label="t('duplicates.dropLabel', { path: c.path })" :data-tip="t('duplicates.toTrash')" @update:model-value="(v: boolean) => toggle(gi, c, v)" /></span>
          </div>
        </section>
      </template>
      <div v-else class="empty"><G n="IconCheck" /><div>{{ t('duplicates.none') }}</div><div class="muted" style="font-size:12px">{{ t('duplicates.scanned', { n: num(res.files) }) }}</div></div>
    </div>
  </div>
</template>
