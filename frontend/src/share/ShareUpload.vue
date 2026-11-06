<script setup lang="ts">
// Dropping files into a link that collects them. They go one after the other into the folder on screen, and a name
// that is taken gets a number, since a link never replaces a file.
import { Button, Card, ProgressBar } from '@glimstone/vue';
import { computed, reactive, ref } from 'vue';
import { ApiError } from '@/api/client';
import type { ShareApi } from '@/api/share';
import { fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';

const props = defineProps<{ api: ShareApi; dir: string; room?: number }>();
const emit = defineEmits<{ done: [] }>();
const s = (key: string, params?: Record<string, string | number>) => t('visit.' + key, params);

interface Sending { name: string; size: number; sent: number; state: 'wait' | 'run' | 'done' | 'skipped' | 'fail'; note?: string }
const list = reactive<Sending[]>([]);
const over = ref(false), running = ref(false);
const input = ref<HTMLInputElement | null>(null);
const queue: { file: File; row: Sending }[] = [];

const progress = (r: Sending) => (r.size ? Math.round((r.sent / r.size) * 100) : 100);
const left = computed(() => (props.room == null ? '' : s('room', { size: fmtSize(props.room) })));

function why(e: unknown, name: string) {
  const status = e instanceof ApiError ? e.status : 0;
  const key = { 413: 'tooBig', 507: 'noSpace', 429: 'tooManyUploads', 403: 'noUploads' }[status] ?? 'uploadFailed';
  return s(key, { name });
}

function add(files: FileList | File[] | null | undefined) {
  for (const file of files ?? []) {
    const row = reactive<Sending>({ name: file.webkitRelativePath || file.name, size: file.size, sent: 0, state: 'wait' });
    list.unshift(row);
    queue.push({ file, row });
  }
  if (!running.value) run();
}

async function run() {
  running.value = true;
  let any = false;
  while (queue.length) {
    const { file, row } = queue.shift()!;
    row.state = 'run';
    try {
      const up = await props.api.upload(props.dir, file, (n) => { row.sent = n; });
      row.sent = row.size;
      row.state = up.state === 'skipped' ? 'skipped' : 'done';
      any = true;
    } catch (e) {
      row.state = 'fail';
      row.note = why(e, row.name);
    }
  }
  running.value = false;
  if (any) emit('done');
}

function drop(e: DragEvent) {
  over.value = false;
  add(e.dataTransfer?.files);
}
function picked() {
  add(input.value?.files);
  if (input.value) input.value.value = '';
}
</script>

<template>
  <Card class="glim-card sp-card sp-up" :class="{ over }" :title="s('uploadTitle')" @dragover.prevent="over = true" @dragleave="over = false" @drop.prevent="drop">
    <div class="sp-drop">
      <G n="IconUpload" />
      <p class="sp-text">{{ s('uploadText') }}</p>
      <div class="sp-uprow">
        <Button size="key" :label="s('pick')" label-key="IconUpload" tone="accent" data-sp="pick" @click="input?.click()" />
        <span v-if="left" class="muted glim-num">{{ left }}</span>
      </div>
      <input ref="input" type="file" multiple hidden data-sp="files" @change="picked">
    </div>
    <div v-if="list.length" class="sp-sending">
      <div v-for="(r, i) in list" :key="i" class="sp-send" :class="r.state">
        <G :n="r.state === 'fail' ? 'IconWarning' : r.state === 'done' || r.state === 'skipped' ? 'IconCheck' : 'IconUpload'" />
        <span class="sp-nm">{{ r.state === 'fail' ? r.note : r.state === 'skipped' ? s('skipped', { name: r.name }) : r.state === 'done' ? s('sent', { name: r.name }) : s('sending', { name: r.name }) }}</span>
        <ProgressBar v-if="r.state === 'run' || r.state === 'wait'" :value="progress(r) / 100" :label="r.name" />
      </div>
    </div>
  </Card>
</template>
