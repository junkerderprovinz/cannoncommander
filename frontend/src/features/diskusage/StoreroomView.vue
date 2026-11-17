<script setup lang="ts">
// Abstellkammer: what the server no longer uses, by kind, and what already waits in the storeroom. Nothing happens
// before the server's plan was shown and agreed to, and every find is checked again when it is cleared away.
import { Button, Checkbox, EmptyState, InfoBubble, Pill, SubCard } from '@glimstone/vue';
import { computed, onMounted, reactive, ref, watch } from 'vue';
import { type StoreroomOp, type StoreroomRef, type StoreroomScan, storeroom } from '@/api/storeroom';
import { fmtDate, fmtSize } from '@/core/format';
import { G, glyphComponent } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { plural, t } from '@/i18n';
import { askSteps } from './askSteps';
import { keyOf, nameOf, SECTIONS, sections, stepLine } from './storeroom';

const emit = defineEmits<{ summary: [text: string] }>();
const scan = ref<StoreroomScan | null>(null);
const failed = ref('');
const picked = reactive(new Set<string>());
const busy = ref(false);

async function load() {
  try {
    scan.value = await storeroom.scan();
    failed.value = '';
  } catch (e) {
    failed.value = (e as Error).message;
  }
  const keys = new Set([...(scan.value?.finds ?? []), ...(scan.value?.room ?? []).map((e) => ({ kind: 'room', id: e.id }))].map(keyOf));
  for (const k of [...picked]) if (!keys.has(k)) picked.delete(k);
}
onMounted(load);

const groups = computed(() => sections(scan.value?.finds ?? []));
const room = computed(() => scan.value?.room ?? []);
const glyphOf = (kind: string) => SECTIONS.find((s) => s.id === kind)?.g ?? 'IconCube';
const flip = (k: string) => (picked.has(k) ? picked.delete(k) : picked.add(k));
const pickedFinds = computed(() => (scan.value?.finds ?? []).filter((f) => picked.has(keyOf(f))).map((f): StoreroomRef => ({ kind: f.kind, id: f.id })));
const pickedRoom = computed(() => room.value.filter((e) => picked.has(keyOf({ kind: 'room', id: e.id }))).map((e): StoreroomRef => ({ id: e.id })));
const canWrite = computed(() => !!scan.value?.write && !busy.value);

watch(scan, (s) => {
  if (s) emit('summary', t('diskusage.storeroom.foot', { finds: plural(s.finds.length, 'diskusage.storeroom.finds'), held: plural(s.room.length, 'diskusage.storeroom.held') }));
}, { immediate: true });

const VERBS: Record<StoreroomOp, string> = { store: 'IconArchive', restore: 'IconRestore', discard: 'IconTrash' };

async function act(op: StoreroomOp, refs: StoreroomRef[]) {
  if (!refs.length) return toast(t('diskusage.pickFirst'), 'warn');
  busy.value = true;
  try {
    const plan = await storeroom.plan(op, refs);
    if (!(await askSteps(t(`diskusage.storeroom.ask.${op}`), plan.map((s) => stepLine(op, s)), t(`diskusage.storeroom.${op}`), VERBS[op]))) return;
    const done = await storeroom.run(op, refs);
    const ok = done.filter((s) => s.done).length, left = done.filter((s) => !s.done);
    if (ok) toast(plural(ok, 'diskusage.storeroom.done'));
    for (const s of left) toast(stepLine(op, s), 'warn');
    picked.clear();
  } catch (e) {
    toast((e as Error).message, 'warn');
  } finally {
    busy.value = false;
    await load();
  }
}
</script>

<template>
  <div class="du-room">
    <EmptyState v-if="failed" nested :glyph="glyphComponent('IconWarning')" :title="failed" />
    <template v-else-if="scan">
      <p v-if="!scan.docker" class="du-room-note">{{ t('diskusage.storeroom.noDocker') }}</p>
      <p v-else-if="!scan.write" class="du-room-note">{{ t('diskusage.storeroom.readOnly') }}</p>
      <SubCard v-for="gr in groups" :key="gr.id" :title="t(`diskusage.storeroom.kinds.${gr.id}`)" :data-section="gr.id">
        <div v-for="f in gr.finds" :key="keyOf(f)" class="du-room-row" :class="{ on: picked.has(keyOf(f)) }" :data-find="keyOf(f)">
          <Checkbox slim class="du-room-check" :model-value="picked.has(keyOf(f))" :label="nameOf(f)" @update:model-value="flip(keyOf(f))" />
          <span class="du-dname"><G :n="gr.g" /><span class="du-dtext"><span class="n">{{ nameOf(f) }}</span><span v-if="f.path" class="sub mono"><bdi>{{ f.path }}</bdi></span></span></span>
          <span class="glim-num r">{{ f.size ? fmtSize(f.size) : '' }}</span>
        </div>
        <div class="du-room-foot">
          <InfoBubble v-if="gr.id === 'image'" :tip="t('diskusage.storeroom.forGood')" />
          <Button :label="gr.id === 'image' ? t('diskusage.storeroom.discard') : t('diskusage.storeroom.store')" :label-key="gr.id === 'image' ? 'IconTrash' : 'IconArchive'" :disabled="!canWrite" :data-store="gr.id" @click="act('store', pickedFinds.filter((r) => (r.kind === 'cache' ? 'image' : r.kind) === gr.id))" />
        </div>
      </SubCard>
      <EmptyState v-if="scan.docker && !groups.length" nested :glyph="glyphComponent('IconCheck')" :title="t('diskusage.storeroom.nothing')" />
      <SubCard :title="t('diskusage.storeroom.room')" data-section="room">
        <template v-if="room.length">
          <div v-for="e in room" :key="e.id" class="du-room-row" :class="{ on: picked.has(keyOf({ kind: 'room', id: e.id })) }" :data-entry="e.id">
            <Checkbox slim class="du-room-check" :model-value="picked.has(keyOf({ kind: 'room', id: e.id }))" :label="e.name" @update:model-value="flip(keyOf({ kind: 'room', id: e.id }))" />
            <span class="du-dname"><G :n="glyphOf(e.kind)" /><span class="du-dtext"><span class="n">{{ e.name }}</span><span class="sub mono"><bdi>{{ e.from }}</bdi></span></span></span>
            <Pill>{{ t('diskusage.storeroom.since', { date: fmtDate(Date.parse(e.at)) }) }}</Pill>
            <span class="glim-num r">{{ e.size ? fmtSize(e.size) : '' }}</span>
          </div>
          <div class="du-room-foot">
            <Button :label="t('diskusage.storeroom.restore')" label-key="IconRestore" :disabled="!canWrite" data-restore @click="act('restore', pickedRoom)" />
            <Button :label="t('diskusage.storeroom.discard')" label-key="IconTrash" :disabled="!canWrite" data-discard @click="act('discard', pickedRoom)" />
          </div>
        </template>
        <p v-else class="du-room-note">{{ t('diskusage.storeroom.roomEmpty') }}</p>
      </SubCard>
    </template>
  </div>
</template>
