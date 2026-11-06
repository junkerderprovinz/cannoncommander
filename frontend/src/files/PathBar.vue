<script setup lang="ts">
// The path as buttons. A separator opens the folders below the part on its left, as Explorer's arrows do; a click
// beside the buttons turns the row into a field to type the path.
import { computed, nextTick, ref, watch } from 'vue';
import { run } from '@/core/actions';
import { menu } from '@/core/menu';
import { prefs } from '@/core/prefs';
import { byName, t } from '@/i18n';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import { split } from './paths';

const props = defineProps<{ i: number }>();
const F = usePanels();
const listing = useListing();
const tab = computed(() => F.T(props.i));
const parts = computed(() => split(tab.value.path));
const hasDirs = computed(() => listing.lists.get(tab.value.path)?.items.some((k) => k.dir) ?? false);
const under = (k: number) => '/' + parts.value.slice(0, k).join('/');

async function psub(e: MouseEvent, folder: string) {
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect(), here = tab.value.path;
  const dirs = (await listing.tree(folder).catch(() => []))
    .filter((k) => prefs.showHidden || !k.name.startsWith('.'))
    .sort((a, b) => byName(a.name, b.name));
  menu(r.left, r.bottom + 4, dirs.map((k) => {
    const onPath = here === k.path || here.startsWith(k.path + '/');
    return { label: k.label || k.name, g: onPath ? 'IconFolderOpen' : 'IconFolder', run: () => F.go(k.path, props.i) };
  }));
}

function onBar(e: MouseEvent) {
  if ((e.target as Element).closest('.pseg, .psep, .pmore')) return;
  F.active = props.i;
  run('gotoPath');
}
async function more() {
  tab.value.pathOpen = !tab.value.pathOpen;
  await nextTick();
  document.querySelectorAll('.pathbar.open').forEach((p) => { p.scrollLeft = p.scrollWidth; });
}

const input = ref<HTMLInputElement | null>(null);
watch(() => tab.value.editPath, async (on) => {
  if (!on) return;
  await nextTick();
  input.value?.focus();
  input.value?.select();
}, { immediate: true });
function key(e: KeyboardEvent) {
  if (e.key === 'Enter') F.go((e.target as HTMLInputElement).value.trim() || '/', props.i);
  if (e.key === 'Escape') tab.value.editPath = false;
}
</script>

<template>
  <input v-if="tab.editPath" :id="`pathedit-${i}`" ref="input" class="pathbar pathedit mono" :value="tab.path" :aria-label="t('files.path')" @keydown="key" @blur="tab.editPath = false">
  <div v-else class="pathbar" :class="{ open: tab.pathOpen }" data-pathbar :data-tip="t('files.pathTip')" @click="onBar">
    <button v-if="parts.length > 1" class="pmore" data-pathmore :aria-label="t('files.pathMore')" @click="more">…</button>
    <button class="pseg" data-go="/" @click="F.go('/', i)">{{ t('files.server') }}</button>
    <template v-for="(part, k) in parts" :key="k">
      <button class="psep" :data-psub="under(k)" :aria-label="t('files.foldersIn', { name: split(under(k)).pop() || t('files.server') })" @click="psub($event, under(k))">›</button>
      <button class="pseg" :class="{ last: k === parts.length - 1 }" :data-go="under(k + 1)" @click="F.go(under(k + 1), i)">{{ part.replace('::', '') }}</button>
    </template>
    <button v-if="hasDirs" class="psep" :data-psub="tab.path" :aria-label="t('files.foldersIn', { name: parts.at(-1) || t('files.server') })" @click="psub($event, tab.path)">›</button>
  </div>
</template>
