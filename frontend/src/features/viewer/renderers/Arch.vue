<script setup lang="ts">
// What an archive holds, read through the server as a folder whose path ends in "::".
import { computed, onMounted, ref } from 'vue';
import { api } from '@/api/client';
import { closeWin } from '@/core/windows';
import { fmtSize } from '@/core/format';
import { G, glyphOf } from '@/core/glyphs';
import { inArchive } from '@/files/paths';
import { t } from '@/i18n';
import { usePanels } from '@/stores/panels';
import { ext, joinPath, plural } from '../kinds';
import type { REmits, RProps } from './types';

const props = defineProps<RProps>();
const emit = defineEmits<REmits>();

// Enough to see what is in there without walking an 18 GB backup to its end.
const LIMIT = 2000;
interface Row { path: string; dir: boolean; size: number }
const list = ref<Row[] | null>(null);
const more = ref(false);
const failed = ref(false);
const root = computed(() => joinPath(props.folder, props.n.name) + '::');
const shown = computed(() => (props.compact ? list.value?.slice(0, 10) : list.value) || []);
const hidden = computed(() => (list.value?.length ?? 0) - shown.value.length);

async function walk(path: string, pre: string, out: Row[]) {
  const got = await api.fsList(path);
  for (const k of got?.items || []) {
    if (out.length >= LIMIT) { more.value = true; return; }
    out.push({ path: pre + k.name + (k.dir ? '/' : ''), dir: k.dir, size: k.size });
    if (k.dir) await walk(joinPath(path, k.name), pre + k.name + '/', out);
  }
}

onMounted(async () => {
  const out: Row[] = [];
  try {
    await walk(root.value, '', out);
  } catch {
    if (!out.length) { failed.value = true; return; }
  }
  list.value = out;
  const files = out.filter((x) => !x.dir);
  emit('info', `${ext(props.n.name).toUpperCase()}, ${plural(files.length, 'files')}${more.value ? '+' : ''}, ${t('viewer.arch.unpacked', { size: fmtSize(files.reduce((a, x) => a + x.size, 0)) })}`);
});

function browse() {
  closeWin();
  usePanels().go(root.value);
}
</script>

<template>
  <div v-if="!compact && !inArchive(folder)" class="cv-bar"><span class="grow"></span><button id="cv-browse" class="btn" @click="browse"><G n="IconFolderOpen" />{{ t('viewer.arch.browse') }}</button></div>
  <div v-if="failed" class="empty"><G n="IconArchive" /><div>{{ t('viewer.arch.unreadable') }}</div></div>
  <div v-else class="cv-scroll">
    <div class="cv-mini">
      <div v-for="x in shown" :key="x.path" class="cv-mrow" :class="{ dir: x.dir }">
        <span class="tile"><G :n="x.dir ? 'IconFolder' : glyphOf({ name: x.path })" /></span><span class="t mono">{{ x.path }}</span>
        <span v-if="x.dir"></span><span v-else class="pill glim-num">{{ fmtSize(Math.round(x.size)) }}</span>
      </div>
    </div>
    <div v-if="hidden > 0 || more" class="muted cv-more">{{ more ? t('viewer.arch.limit', { n: LIMIT.toLocaleString() }) : t('viewer.and', { rest: plural(hidden, 'moreEntries') }) }}</div>
  </div>
</template>
