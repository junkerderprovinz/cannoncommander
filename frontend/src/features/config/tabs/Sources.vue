<script setup lang="ts">
import { Button, Card, EmptyState, FieldRow, IconButton, ListRow, Pill } from '@glimstone/vue';
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { admin, type IndexState, type Source } from '@/api/admin';
import { run } from '@/core/actions';
import { fmtSize } from '@/core/format';
import { G, glyphComponent } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { go } from '@/core/views';
import { win } from '@/core/windows';
import { locale } from '@/i18n';
import { ago, attempt, c, confirmWin, fail, soon } from '@/features/config/shared';
import SourceWin from '../wins/SourceWin.vue';

const sources = ref<Source[]>([]);
const states = ref<IndexState[]>([]);
const restartNeeded = ref(false);
let poll: ReturnType<typeof setInterval> | undefined;

const stateOf = (s: Source) => states.value.find((x) => x.source === s.name);
const busy = (st?: IndexState) => !!st && (st.status === 'indexing' || !!st.job);

async function load() {
  const [list, ix] = await Promise.all([admin.sources(), admin.index()]);
  const was = states.value.filter(busy).map((x) => x.source);
  sources.value = list.sources;
  restartNeeded.value = list.restartNeeded;
  states.value = ix;
  for (const st of ix) if (was.includes(st.source) && !busy(st)) toast(c('sources.indexed', { name: st.source }));
  clearInterval(poll);
  // Indexing runs as a job on the server, and the list looks again until no source is busy.
  if (ix.some(busy)) poll = setInterval(() => load().catch(() => {}), 1500);
}
onMounted(() => { load().catch(() => {}); });
onBeforeUnmount(() => clearInterval(poll));

const glyph = (s: Source) => (s.path === '/boot' ? 'IconFlash' : s.path.startsWith('/mnt/remotes') ? 'IconCloud' : 'IconDatabase');
const hasIndex = (s: Source) => s.indexed && !!stateOf(s) && stateOf(s)!.status !== 'off';

function every(seconds: number) {
  const min = Math.round(seconds / 60);
  return min < 60 ? c('sources.everyMinutes', { n: min }) : c('sources.everyHours', { n: Math.round(min / 60) });
}
function rowTip(s: Source) {
  const st = stateOf(s);
  if (!hasIndex(s) || !st?.scanEverySeconds) return null;
  return c('sources.planTip', { every: every(st.scanEverySeconds) });
}

// The scheduler plans per source, so the card names the soonest run and the latest one.
const next = computed(() => Math.min(...states.value.map((x) => x.nextScan ?? Infinity)));
const last = computed(() => Math.max(0, ...states.value.map((x) => x.lastIndexed ?? 0)) || null);

async function reindex(s?: Source) {
  if (s && busy(stateOf(s))) return;
  if (!(await attempt(admin.reindex(s?.name)))) return;
  toast(s ? c('sources.reading', { name: s.name }) : c('sources.readingAll'), 'ok', { label: c('sources.jobs'), run: () => run('jobs') });
  await load();
}

function edit(s: Source | null) {
  win({ title: s ? c('sources.edit') : c('sources.add'), body: SourceWin, props: { source: s, all: sources.value, saved: load } });
}
function remove(s: Source) {
  if (sources.value.length === 1) return fail(c('sources.keepOne'));
  confirmWin({
    title: c('sources.remove'), verb: c('remove'), glyph: 'IconTrash',
    text: [c('sources.removeText1', { name: s.name }), c('sources.removeText2', { path: s.path })],
    run: async () => {
      if (!(await attempt(admin.saveSources(sources.value.filter((x) => x !== s))))) return;
      await load();
      toast(c('removed', { name: s.name }));
    },
  });
}
</script>

<template>
  <Card :title="c('sources.title')">
    <div v-if="sources.length" class="a-list">
      <ListRow v-for="(s, i) in sources" :key="s.path" :name="s.name" :tip="rowTip(s)" :data-open="i" @click="edit(s)">
        <template #tile><G :n="glyph(s)" /></template>
        <template #badges>
          <Pill class="mono">{{ s.path }}</Pill>
          <Pill v-if="busy(stateOf(s))" class="run">{{ c('sources.readingPill') }}</Pill>
          <template v-else-if="hasIndex(s)"><Pill>{{ fmtSize(stateOf(s)!.size) }}</Pill><Pill class="glim-num">{{ c('sources.files', { n: stateOf(s)!.files.toLocaleString(locale()) }) }}</Pill></template>
          <Pill v-else tone="state">{{ c('sources.noIndex') }}</Pill>
          <Pill v-if="s.readOnly" tone="state">{{ c('sources.readOnly') }}</Pill>
        </template>
        <template #actions>
          <IconButton :absent="!hasIndex(s) || busy(stateOf(s))" :glyph="glyphComponent('IconRefresh')" :tip="c('sources.reindex')" @click="reindex(s)" />
          <IconButton :glyph="glyphComponent('IconPencil')" :tip="c('edit')" :data-open="i" @click="edit(s)" />
          <IconButton :glyph="glyphComponent('IconTrash')" :tip="c('remove')" @click="remove(s)" />
        </template>
      </ListRow>
    </div>
    <EmptyState v-else nested :glyph="glyphComponent('IconDatabase')" :title="c('sources.none')" />
    <FieldRow v-if="restartNeeded" :label="c('sources.restart')" :hint="c('sources.restartTip')">
      <Pill tone="warn">{{ c('sources.restartPill') }}</Pill><Button :label="c('sources.toRestart')" label-key="IconRefresh" @click="go('settings', 'app')" />
    </FieldRow>
    <div class="a-foot"><Button data-add :label="c('sources.add')" label-key="IconAdd" @click="edit(null)" /></div>
  </Card>
  <Card :title="c('sources.index')">
    <FieldRow :label="c('sources.every')" :hint="c('sources.everyTip')"><Pill>{{ Number.isFinite(next) ? c('sources.nextRun', { when: soon(next) }) : c('sources.notPlanned') }}</Pill></FieldRow>
    <FieldRow :label="c('sources.last')"><Pill>{{ last ? ago(last) : c('sources.never') }}</Pill><Button data-reindexall :label="c('sources.reindexAll')" label-key="IconRefresh" @click="reindex()" /></FieldRow>
  </Card>
</template>
