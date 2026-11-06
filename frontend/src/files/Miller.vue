<script setup lang="ts">
// Column view: the folders above the current one stay visible as columns to the left, and a folder under the cursor
// already shows its contents on the right, so the arrows walk the tree sideways.
import { computed, watchEffect } from 'vue';
import type { Item } from '@/api/client';
import { G, glyphOf } from '@/core/glyphs';
import { prefs } from '@/core/prefs';
import { byName } from '@/i18n';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import type { Entry } from './entries';
import ListItem from './ListItem.vue';
import { join, split } from './paths';

const props = defineProps<{ i: number; list: Entry[] }>();
const F = usePanels();
const listing = useListing();
const tab = computed(() => F.T(props.i));

const parts = computed(() => split(tab.value.path).filter((p) => !p.includes('::')));
const ancestors = computed(() => parts.value.map((p, j) => ['/' + parts.value.slice(0, j).join('/'), p] as const).slice(-2));
const peek = computed(() => {
  const cur = props.list[tab.value.cursor];
  return cur?.dir ? join(cur.at || tab.value.path, cur.name) : null;
});

watchEffect(() => {
  for (const f of [...ancestors.value.map(([folder]) => folder), peek.value]) if (f && !listing.lists.has(f)) listing.load(f).catch(() => {});
});

const kids = (folder: string): Item[] => (listing.lists.get(folder)?.items ?? [])
  .filter((k) => prefs.showHidden || !k.name.startsWith('.'))
  .sort((a, b) => (b.dir ? 1 : 0) - (a.dir ? 1 : 0) || byName(a.name, b.name));
const label = (folder: string, k: Item) => listing.placeLabel(join(folder, k.name)) || k.name;
</script>

<template>
  <div v-for="[folder, on] in ancestors" :key="folder" class="mcol">
    <div v-for="k in kids(folder)" :key="k.name" class="mitem" :class="{ on: k.name === on }" :data-go="k.dir ? join(folder, k.name) : undefined">
      <G :n="glyphOf(k)" /><span>{{ label(folder, k) }}</span><G v-if="k.dir" n="IconExpand" />
    </div>
  </div>
  <div class="mcol cur">
    <ListItem v-for="(k, idx) in list" :key="(k.at || '') + k.name" :k="k" :idx="idx" view="columns" :path="tab.path"
      :marked="tab.marked.has(k.name)" :cursor="idx === tab.cursor" :cut="F.clip?.cut && F.clip.from === tab.path && F.clip.items.includes(k.name)" :mark="F.marks[i]?.[k.name]" />
  </div>
  <div v-if="peek" class="mcol">
    <div v-for="k in kids(peek)" :key="k.name" class="mitem" :data-go="k.dir ? join(peek, k.name) : undefined">
      <G :n="glyphOf(k)" /><span>{{ label(peek, k) }}</span><G v-if="k.dir" n="IconExpand" />
    </div>
  </div>
</template>
