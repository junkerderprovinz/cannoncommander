<script setup lang="ts">
// The path as buttons. A separator opens the folders below the part on its left, as Explorer's arrows do; a click
// beside the buttons turns the row into a field to type the path.
import { computed, nextTick, ref, watch } from 'vue';
import { run } from '@/core/actions';
import { extend } from '@/core/extend';
import { host, rootName, windowsPaths } from '@/core/host';
import { menu } from '@/core/menu';
import { prefs } from '@/core/prefs';
import { byName, t } from '@/i18n';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import { useUser } from '@/stores/user';
import { shake } from '@/ui/kit';
import { shown } from './entries';
import { type Crumb, crumbs } from './crumbs';
import { nativePath, split, typedPath } from './paths';

const props = defineProps<{ i: number }>();
const F = usePanels();
const listing = useListing();
const tab = computed(() => F.T(props.i));
// A network share the computer mounted deep down, such as one gvfs mounted, where the path bar starts.
const share = computed(() => {
  const vol = host.desktop ? listing.lists.get(tab.value.path)?.volume : undefined;
  return vol?.kind === 'network' && split(vol.path).length > 1 && !vol.path.startsWith('/net/') ? vol.path : undefined;
});
const items = computed(() => crumbs(tab.value.path, share.value));
const hasDirs = computed(() => listing.lists.get(tab.value.path)?.items.some((k) => k.dir) ?? false);
const above = (k: number) => (k > 0 ? items.value[k - 1]!.path : '/');
// In the desktop app a folder goes by the name the computer gives it where it has one, as a drive or a network share
// does, and a drive on Windows is C: otherwise, as in Explorer.
function crumbName(c: Crumb, k: number) {
  if (c.network) return t('files.network');
  const named = host.desktop ? listing.placeLabel(c.path) : undefined;
  if (named) return named;
  return k > 0 || !windowsPaths() ? c.part.replace('::', '') : c.part === 'net' ? t('files.network') : c.part + ':';
}
const aboveName = (k: number) => (k > 0 ? crumbName(items.value[k - 1]!, k - 1) : rootName());

async function psub(e: MouseEvent, folder: string) {
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect(), here = tab.value.path;
  const dirs = (await listing.tree(folder).catch(() => []))
    .filter(shown(prefs.showHidden, prefs.showSystem))
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
  if (e.key === 'Enter') {
    const typed = (e.target as HTMLInputElement).value.trim();
    if (extend.pathWords.some((fn) => fn(typed, props.i))) tab.value.editPath = false;
    else F.go(host.desktop ? typedPath(typed, { windows: windowsPaths(), home: useUser().me.home || '/' }) : typed || '/', props.i).then((ok) => { if (!ok) shake(input.value); });
  }
  if (e.key === 'Escape') tab.value.editPath = false;
}
</script>

<template>
  <input v-if="tab.editPath" :id="`pathedit-${i}`" ref="input" class="pathbar pathedit mono" :value="nativePath(tab.path, windowsPaths())" :aria-label="t('files.path')" @keydown="key" @blur="tab.editPath = false">
  <div v-else class="pathbar" :class="{ open: tab.pathOpen }" data-pathbar :data-tip="t('files.pathTip')" @click="onBar">
    <button v-if="items.length > 1" class="pmore" data-pathmore :aria-label="t('files.pathMore')" @click="more">…</button>
    <button class="pseg" data-go="/" @click="F.go('/', i)">{{ rootName() }}</button>
    <template v-for="(c, k) in items" :key="c.path">
      <button class="psep" :data-psub="above(k)" :aria-label="t('files.foldersIn', { name: aboveName(k) })" @click="psub($event, above(k))">›</button>
      <button class="pseg" :class="{ last: k === items.length - 1 }" :data-go="c.path" @click="F.go(c.path, i)">{{ crumbName(c, k) }}</button>
    </template>
    <button v-if="hasDirs" class="psep" :data-psub="tab.path" :aria-label="t('files.foldersIn', { name: aboveName(items.length) })" @click="psub($event, tab.path)">›</button>
  </div>
</template>
