<script setup lang="ts">
import { segItems } from '@/ui/kit';
import { Button, Card, FieldRow, IconButton, ListRow, Pill, Selector, Toggle } from '@glimstone/vue';
import { computed, onMounted, ref } from 'vue';
import { admin, type AppSettings, type Remote } from '@/api/admin';
import { G, glyphComponent } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { win } from '@/core/windows';
import { attempt, c, confirmWin } from '@/features/config/shared';
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
      <ListRow v-for="(r, i) in remotes" :key="r.name" :name="r.name" :tip="r.error ? c('remotes.mountFailed', { error: r.error }) : null" :data-open="i" @click="edit(r)">
        <template #tile><G :n="typeOf(r.type).g" /></template>
        <template #badges>
          <Pill>{{ typeOf(r.type).label }}</Pill><Pill class="mono">{{ r.mount }}</Pill>
          <Pill v-if="r.error" tone="warn">{{ c('remotes.failedPill') }}</Pill>
          <Pill v-else-if="!r.mounted" tone="state">{{ c('remotes.unmountedPill') }}</Pill>
        </template>
        <template #actions>
          <IconButton :glyph="glyphComponent(r.mounted ? 'IconUnlink' : 'IconLink')" :tip="r.mounted ? c('remotes.unmount') : c('remotes.mount')" @click="mount(r)" />
          <IconButton :glyph="glyphComponent('IconPencil')" :tip="c('edit')" :data-open="i" @click="edit(r)" />
          <IconButton :glyph="glyphComponent('IconTrash')" :tip="c('remove')" @click="remove(r)" />
        </template>
      </ListRow>
    </div>
    <div v-else class="empty"><G n="IconCloud" /><div>{{ c('remotes.none') }}</div></div>
    <div class="a-foot"><Button data-add :label="c('remotes.add')" label-key="IconAdd" @click="edit(null)" /></div>
  </Card>
  <Card :title="c('remotes.mounting')">
    <FieldRow :label="c('remotes.automount')" :hint="c('remotes.automountTip')"><Toggle v-model="automount" /></FieldRow>
    <FieldRow :label="c('remotes.cache')" :hint="c('remotes.cacheTip')"><Selector v-model="cache" :items="segItems([['off', c('remotes.cacheOff')], ['writes', c('remotes.cacheWrites')], ['full', c('remotes.cacheFull')]])" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <FieldRow v-if="cache !== 'off'" :label="c('remotes.atMost')" :hint="c('remotes.atMostTip')"><Selector v-model="cacheSize" :items="segItems([[10, '10 GB'], [50, '50 GB'], [200, '200 GB']])" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
  </Card>
</template>
