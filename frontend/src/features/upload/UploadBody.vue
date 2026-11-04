<script setup lang="ts">
// Hochladen: a drop zone, the target, and the queue with its progress per file and in total.
import { InfoBubble } from '@glimstone/vue';
import { computed, ref } from 'vue';
import { fmtSize } from '@/core/format';
import { G, glyphOf } from '@/core/glyphs';
import { t } from '@/i18n';
import Frow from '@/ui/Frow.vue';
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
  <button id="cu-drop" class="well glim-well cu-drop" :class="{ over }" @click="files?.click()" @dragover.prevent.stop="over = true" @dragleave="over = false" @drop.prevent.stop="dropped">
    <G n="IconUpload" /><span class="t">{{ t('upload.dropHere') }}</span><span class="muted">{{ t('upload.orClick') }}</span>
  </button>
  <input id="cu-in" ref="files" type="file" multiple hidden @change="picked"><input id="cu-din" type="file" webkitdirectory hidden @change="picked">
  <div class="well glim-well"><Frow :label="t('upload.target')" :tip="t('upload.targetTip')"><span class="pill mono cu-path">{{ st.path }}</span></Frow></div>
  <div id="cu-queue" class="well glim-well">
    <div class="eyebrow">{{ t('upload.queue') }} <InfoBubble :tip="t('upload.queueTip')" />
      <button v-if="busy" id="cu-all" class="btn icon" :data-tip="pausedAll() ? t('upload.resumeAll') : t('upload.pauseAll')" :aria-label="pausedAll() ? t('upload.resumeAll') : t('upload.pauseAll')" @click="toggleAll"><G :n="pausedAll() ? 'IconPlay' : 'IconPause'" /></button>
      <button v-if="ended" id="cu-clear" class="btn icon" :data-tip="t('upload.clear')" :aria-label="t('upload.clear')" @click="clearDone"><G n="IconEmpty" /></button>
    </div>
    <template v-if="items.length">
      <div class="cu-total">
        <Frow :label="t('upload.progress', { done: finished, all: alive.length === 1 ? t('upload.one') : t('upload.many', { n: alive.length }) })"><span class="pill glim-num">{{ t('upload.bytes', { done: fmtSize(done) || '0 B', total: fmtSize(total) || '0 B' }) }}</span></Frow>
        <div class="bar"><i :style="{ width: (total ? (done / total) * 100 : 0) + '%' }"></i></div>
      </div>
      <div class="cu-list">
        <div v-for="i in items" :key="i.id" class="cu-row" :class="{ done: i.state === 'done' }" :data-id="i.id">
          <span class="tile"><G :n="glyphOf({ name: i.name })" /></span><span class="t">{{ i.name }}</span>
          <span v-if="i.state === 'done'" class="cu-ok"><i></i>{{ t('upload.state.done') }}</span>
          <span v-else-if="!open(i)" class="pill">{{ t(`upload.state.${i.state}`) }}</span>
          <template v-else><span class="bar"><i :style="{ width: (i.loaded / (i.size || 1)) * 100 + '%' }"></i></span><span class="pill glim-num">{{ stateOf(i) }}</span></template>
          <span class="cu-acts">
            <template v-if="open(i)">
              <button class="btn icon" data-up="pause" :data-tip="i.state === 'paused' ? t('upload.resume') : t('upload.pause')" :aria-label="i.state === 'paused' ? t('upload.resume') : t('upload.pause')" @click="pauseItem(i)"><G :n="i.state === 'paused' ? 'IconPlay' : 'IconPause'" /></button>
              <button class="btn icon" data-up="cancel" :data-tip="t('upload.cancel')" :aria-label="t('upload.cancel')" @click="cancelItem(i)"><G n="IconStop" /></button>
            </template>
          </span>
        </div>
      </div>
    </template>
    <div v-else class="cu-empty muted">{{ t('upload.nothing') }}</div>
  </div>
</template>
