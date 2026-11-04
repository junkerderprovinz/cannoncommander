<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { admin, type AppSettings, type IndexState, type Source } from '@/api/admin';
import { run } from '@/core/actions';
import { fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { win } from '@/core/windows';
import { locale } from '@/i18n';
import ARow from '../kit/ARow.vue';
import BareBtn from '../kit/BareBtn.vue';
import Btn from '../kit/Btn.vue';
import Card from '../kit/Card.vue';
import Frow from '../kit/Frow.vue';
import { ago, c, confirmWin } from '../kit/kit';
import Seg from '../kit/Seg.vue';
import SourceWin from '../wins/SourceWin.vue';

const sources = ref<Source[]>([]);
const ix = ref<IndexState>({ every: 'daily', last: null, running: [] });
let app: AppSettings | null = null;
let poll: ReturnType<typeof setInterval> | undefined;

async function load() {
  const [s, i] = await Promise.all([admin.sources(), admin.index()]);
  const was = sources.value.filter((x) => x.indexing).map((x) => x.path);
  sources.value = s;
  ix.value = i;
  for (const x of s) if (was.includes(x.path) && !x.indexing) toast(c('sources.indexed', { name: x.name }));
  clearInterval(poll);
  // Indexing runs as a job on the server, and the list looks again until no source is busy.
  if (s.some((x) => x.indexing)) poll = setInterval(() => load().catch(() => {}), 1500);
}
onMounted(() => { load().catch(() => {}); admin.app().then((a) => { app = a; }, () => {}); });
onBeforeUnmount(() => clearInterval(poll));

const glyph = (s: Source) => (s.path === '/boot' ? 'IconFlash' : s.path.startsWith('/mnt/remotes') ? 'IconCloud' : 'IconDatabase');

async function reindex(s: Source) {
  if (s.indexing) return;
  await admin.reindex(s.path);
  toast(c('sources.reading', { name: s.name }), 'ok', { label: c('sources.jobs'), run: () => run('jobs') });
  await load();
}
async function reindexAll() {
  for (const s of sources.value.filter((x) => x.indexed)) await reindex(s);
}
const every = computed({
  get: () => ix.value.every,
  set: (v: IndexState['every']) => {
    ix.value.every = v;
    if (app) admin.saveApp({ ...app, indexEvery: v }).then(() => toast(c('saved')), () => toast(c('notSaved'), 'warn'));
  },
});

function edit(s: Source | null) {
  win({ title: s ? c('sources.edit') : c('sources.add'), body: SourceWin, props: { source: s, all: sources.value, saved: async (path: string | null) => { await load(); const n = sources.value.find((x) => x.path === path); if (n) await reindex(n); } } });
}
function remove(s: Source) {
  confirmWin({
    title: c('sources.remove'), verb: c('remove'), glyph: 'IconTrash',
    text: [c('sources.removeText1', { name: s.name }), c('sources.removeText2', { path: s.path })],
    run: async () => { await admin.saveSources(sources.value.filter((x) => x !== s)); await load(); toast(c('removed', { name: s.name })); },
  });
}
</script>

<template>
  <Card :title="c('sources.title')">
    <div v-if="sources.length" class="a-list">
      <ARow v-for="s in sources" :key="s.path" :name="s.name" data-open @click="edit(s)">
        <template #tile><G :n="glyph(s)" /></template>
        <template #badges>
          <span class="pill mono">{{ s.path }}</span>
          <span v-if="s.indexing" class="pill run">{{ c('sources.readingPill') }}</span>
          <template v-else-if="s.indexed"><span class="pill">{{ fmtSize(s.size) }}</span><span class="pill glim-num">{{ c('sources.files', { n: s.files.toLocaleString(locale()) }) }}</span></template>
          <span v-else class="pill state">{{ c('sources.noIndex') }}</span>
          <span v-if="s.readOnly" class="pill state">{{ c('sources.readOnly') }}</span>
        </template>
        <template #acts>
          <BareBtn :g="s.indexed && !s.indexing ? 'IconRefresh' : undefined" :tip="c('sources.reindex')" @click="reindex(s)" />
          <BareBtn g="IconPencil" :tip="c('edit')" @click="edit(s)" />
          <BareBtn g="IconTrash" :tip="c('remove')" @click="remove(s)" />
        </template>
      </ARow>
    </div>
    <div v-else class="empty"><G n="IconDatabase" /><div>{{ c('sources.none') }}</div></div>
    <div class="a-foot"><Btn :label="c('sources.add')" g="IconAdd" @click="edit(null)" /></div>
  </Card>
  <Card :title="c('sources.index')">
    <Frow :label="c('sources.every')" :tip="c('sources.everyTip')"><Seg v-model="every" :options="[['hourly', c('sources.hourly')], ['daily', c('sources.daily')], ['manual', c('sources.manual')]]" /></Frow>
    <Frow :label="c('sources.last')"><span class="a-ctl"><span class="pill">{{ ix.last ? ago(ix.last) : c('sources.never') }}</span><Btn :label="c('sources.reindexAll')" g="IconRefresh" @click="reindexAll" /></span></Frow>
  </Card>
</template>
