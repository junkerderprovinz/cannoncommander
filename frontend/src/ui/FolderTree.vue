<script setup lang="ts">
import { computed, reactive } from 'vue';
import type { TreeItem } from '@/api/types';
import { G } from '@/core/glyphs';
import { split } from '@/files/paths';
import { t } from '@/i18n';
import { useListing } from '@/stores/listing';
import Frow from './Frow.vue';

const props = defineProps<{ st: { path: string } }>();
const listing = useListing();
const open = reactive(new Set(['/', ...split(props.st.path).map((_, k, a) => '/' + a.slice(0, k + 1).join('/'))]));
listing.tree('/').catch(() => {});
open.forEach((p) => listing.tree(p).catch(() => {}));

const rows = computed(() => {
  const out: { k: TreeItem; depth: number }[] = [];
  const walk = (path: string, depth: number) => {
    for (const k of listing.trees.get(path) ?? []) {
      out.push({ k, depth });
      if (open.has(k.path)) walk(k.path, depth + 1);
    }
  };
  walk('/', 0);
  return out;
});
function toggle(p: string) {
  if (open.has(p)) open.delete(p);
  else { open.add(p); listing.tree(p).catch(() => {}); }
}
const glyph = (k: TreeItem) => (k.path === '/boot' ? 'IconFlash' : k.remote || k.path === '/mnt/remotes' ? 'IconCloud' : k.total ? 'IconZFS' : open.has(k.path) ? 'IconFolderOpen' : 'IconFolder');
</script>

<template>
  <Frow :label="t('ui.folder')"><input id="a-pick-path" v-model="st.path" class="field w mono"></Frow>
  <div id="a-tree" class="well a-tree">
    <div v-for="{ k, depth } in rows" :key="k.path" class="nav-row tree-row" :class="{ cur: st.path === k.path }" :style="{ paddingLeft: `${0.3 + depth * 0.85}rem` }" :data-pick="k.path" @click="st.path = k.path">
      <button v-if="k.hasDirs" class="tree-tog" :class="{ open: open.has(k.path) }" :aria-label="open.has(k.path) ? t('side.collapse') : t('side.expand')" @click.stop="toggle(k.path)"><G n="IconExpand" /></button>
      <span v-else class="tree-tog"></span>
      <G :n="glyph(k)" /><span>{{ k.label || k.name }}</span>
    </div>
  </div>
</template>
