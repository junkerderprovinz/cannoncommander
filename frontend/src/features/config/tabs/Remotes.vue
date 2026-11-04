<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { admin, type AppSettings, type Remote } from '@/api/admin';
import { G } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { win } from '@/core/windows';
import ARow from '../kit/ARow.vue';
import BareBtn from '../kit/BareBtn.vue';
import Btn from '../kit/Btn.vue';
import Card from '../kit/Card.vue';
import Frow from '../kit/Frow.vue';
import { attempt, c, confirmWin } from '../kit/kit';
import Seg from '../kit/Seg.vue';
import Sw from '../kit/Sw.vue';
import { typeOf } from '../remotes';
import RemoteWin from '../wins/RemoteWin.vue';

type Mounting = AppSettings['remotes'];
const remotes = ref<Remote[]>([]);
const mounting = ref<Mounting | null>(null);
const load = async () => { remotes.value = (await admin.remotes()).remotes; };
onMounted(() => { load().catch(() => {}); admin.app().then((a) => { mounting.value = a.remotes; }, () => {}); });

function saveRc(change: Partial<Mounting>) {
  if (!mounting.value) return;
  mounting.value = { ...mounting.value, ...change };
  admin.saveApp({ remotes: change }).then(() => toast(c('saved')), () => toast(c('notSaved'), 'warn'));
}
const automount = computed({ get: () => mounting.value?.automount ?? true, set: (v: boolean) => saveRc({ automount: v }) });
const cache = computed({ get: () => mounting.value?.cache ?? 'writes', set: (v: Mounting['cache']) => saveRc({ cache: v }) });
const cacheSize = computed({ get: () => mounting.value?.cacheSize ?? 50, set: (v: number) => saveRc({ cacheSize: v }) });

async function mount(r: Remote) {
  if (!(await attempt(admin.mountRemote(r.name, !r.mounted)))) return load();
  await load();
  toast(r.mounted ? c('remotes.unmounted', { name: r.name }) : c('remotes.mounted', { name: r.name }));
}
function edit(r: Remote | null) {
  win({ title: r ? c('remotes.edit') : c('remotes.add'), wide: true, body: RemoteWin, props: { remote: r, all: remotes.value, saved: load } });
}
function remove(r: Remote) {
  confirmWin({
    title: c('remotes.remove'), verb: c('remove'), glyph: 'IconTrash',
    text: [c('remotes.removeText1', { name: r.name }), c('remotes.removeText2', { type: typeOf(r.type).label })],
    run: async () => {
      if (!(await attempt(admin.deleteRemote(r.name)))) return;
      await load();
      toast(c('removed', { name: r.name }));
    },
  });
}
</script>

<template>
  <Card :title="c('remotes.title')">
    <div v-if="remotes.length" class="a-list">
      <ARow v-for="(r, i) in remotes" :key="r.name" :name="r.name" :tip="r.error ? c('remotes.mountFailed', { error: r.error }) : null" :data-open="i" @click="edit(r)">
        <template #tile><G :n="typeOf(r.type).g" /></template>
        <template #badges>
          <span class="pill">{{ typeOf(r.type).label }}</span><span class="pill mono">{{ r.mount }}</span>
          <span v-if="r.error" class="pill warn">{{ c('remotes.failedPill') }}</span>
          <span v-else-if="!r.mounted" class="pill state">{{ c('remotes.unmountedPill') }}</span>
        </template>
        <template #acts>
          <BareBtn :g="r.mounted ? 'IconUnlink' : 'IconLink'" :tip="r.mounted ? c('remotes.unmount') : c('remotes.mount')" @click="mount(r)" />
          <BareBtn g="IconPencil" :tip="c('edit')" :data-open="i" @click="edit(r)" />
          <BareBtn g="IconTrash" :tip="c('remove')" @click="remove(r)" />
        </template>
      </ARow>
    </div>
    <div v-else class="empty"><G n="IconCloud" /><div>{{ c('remotes.none') }}</div></div>
    <div class="a-foot"><Btn data-add :label="c('remotes.add')" g="IconAdd" @click="edit(null)" /></div>
  </Card>
  <Card :title="c('remotes.mounting')">
    <Frow :label="c('remotes.automount')" :tip="c('remotes.automountTip')"><Sw v-model="automount" /></Frow>
    <Frow :label="c('remotes.cache')" :tip="c('remotes.cacheTip')"><Seg v-model="cache" :options="[['off', c('remotes.cacheOff')], ['writes', c('remotes.cacheWrites')], ['full', c('remotes.cacheFull')]]" /></Frow>
    <Frow v-if="cache !== 'off'" :label="c('remotes.atMost')" :tip="c('remotes.atMostTip')"><Seg v-model="cacheSize" :options="[[10, '10 GB'], [50, '50 GB'], [200, '200 GB']]" /></Frow>
  </Card>
</template>
