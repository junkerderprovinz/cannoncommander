<script setup lang="ts">
// Hochladen: a drop zone, the target, and the queue with its progress per file and in total.
import { FieldRow, GlyphTile, IconButton, InfoBubble, Pill, ProgressBar, SubCard } from '@glimstone/vue';
import { computed, ref } from 'vue';
import { fmtSize } from '@/core/format';
import { G, glyphOf, glyphComponent } from '@/core/glyphs';
import { plural, t } from '@/i18n';
import { addFiles, cancelItem, clearDone, pauseItem, pausedAll, toggleAll, Up, type UpItem } from './queue';

const props = defineProps<{ st: { path: string } }>();
const over = ref(false);
const files = ref<HTMLInputElement | null>(null);

const items = computed(() => Up.items.slice().reverse());
const alive = computed(() => items.value.filter((i) => i.state !== 'cancelled'));
const total = computed(() => alive.value.reduce((a, i) => a + i.size, 0));
const done = computed(() => alive.value.reduce((a, i) => a + i.loaded, 0));
const finished = computed(() => alive.value.filter((i) => i.state === 'done').length);
const busy = computed(() => items.value.some((i) => ['waiting', 'running', 'paused'].includes(i.state)));
const ended = computed(() => items.value.some((i) => ['done', 'cancelled', 'failed'].includes(i.state)));
const open = (i: UpItem) => ['waiting', 'running', 'paused'].includes(i.state);
const stateOf = (i: UpItem) => (i.state === 'running' ? `${Math.floor((i.loaded / (i.size || 1)) * 100)} %` : t(`upload.state.${i.state}`));

function picked(e: Event) {
  const input = e.target as HTMLInputElement;
  if (input.files) void addFiles([...input.files], props.st.path);
  input.value = '';
}
function dropped(e: DragEvent) {
  over.value = false;
  if (e.dataTransfer?.files.length) void addFiles([...e.dataTransfer.files], props.st.path);
}
</script>

<template>
  <button id="cu-drop" class="glim-well cu-drop" :class="{ over }" @click="files?.click()" @dragover.prevent.stop="over = true" @dragleave="over = false" @drop.prevent.stop="dropped">
    <G n="IconUpload" /><span class="t">{{ t('upload.dropHere') }}</span><span class="muted">{{ t('upload.orClick') }}</span>
  </button>
  <input id="cu-in" ref="files" type="file" multiple hidden @change="picked"><input id="cu-din" type="file" webkitdirectory hidden @change="picked">
  <SubCard><FieldRow :label="t('upload.target')" :hint="t('upload.targetTip')"><Pill class="mono cu-path">{{ st.path }}</Pill></FieldRow></SubCard>
  <SubCard id="cu-queue">
    <div class="glim-eyebrow">{{ t('upload.queue') }} <InfoBubble :tip="t('upload.queueTip')" />
      <IconButton v-if="busy" id="cu-all" :tip="pausedAll() ? t('upload.resumeAll') : t('upload.pauseAll')" :glyph="glyphComponent(pausedAll() ? 'IconPlay' : 'IconPause')" @click="toggleAll" />
      <IconButton v-if="ended" id="cu-clear" :tip="t('upload.clear')" :glyph="glyphComponent('IconEmpty')" @click="clearDone" />
    </div>
    <template v-if="items.length">
      <div class="cu-total">
        <FieldRow :label="t('upload.progress', { done: finished, all: plural(alive.length, 'upload.files') })"><Pill class="glim-num">{{ t('upload.bytes', { done: fmtSize(done) || '0 B', total: fmtSize(total) || '0 B' }) }}</Pill></FieldRow>
        <ProgressBar class="bar" :value="total ? done / total : 0" :label="t('upload.progress', { done: finished, all: plural(alive.length, 'upload.files') })" />
      </div>
      <div class="cu-list">
        <div v-for="i in items" :key="i.id" class="cu-row" :class="{ done: i.state === 'done' }" :data-id="i.id">
          <GlyphTile :glyph="glyphComponent(glyphOf({ name: i.name }))" /><span class="t">{{ i.name }}</span>
          <span v-if="i.state === 'done'" class="cu-ok"><i></i>{{ t('upload.state.done') }}</span>
          <Pill v-else-if="!open(i)">{{ t(`upload.state.${i.state}`) }}</Pill>
          <template v-else><ProgressBar class="bar" :value="i.loaded / (i.size || 1)" :label="i.name" :paused="i.state === 'paused'" /><Pill class="glim-num">{{ stateOf(i) }}</Pill></template>
          <span class="cu-acts">
            <template v-if="open(i)">
              <IconButton :tip="i.state === 'paused' ? t('upload.resume') : t('upload.pause')" :glyph="glyphComponent(i.state === 'paused' ? 'IconPlay' : 'IconPause')" data-up="pause" @click="pauseItem(i)" />
              <IconButton :tip="t('upload.cancel')" :glyph="glyphComponent('IconStop')" data-up="cancel" @click="cancelItem(i)" />
            </template>
          </span>
        </div>
      </div>
    </template>
    <div v-else class="cu-empty muted">{{ t('upload.nothing') }}</div>
  </SubCard>
</template>
