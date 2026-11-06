<script setup lang="ts">
// The menu of the empty search field: the saved searches and a few examples of what the field understands. While
// someone types it steps aside, so the hits stay visible.
import { IconButton } from '@glimstone/vue';
import { computed, nextTick, onMounted } from 'vue';
import type { SavedSearch } from '@/api/tools';
import { run } from '@/core/actions';
import { num } from '@/ui/kit';
import { G, glyphComponent } from '@/core/glyphs';
import { localQuery } from '@/files/fields';
import { t } from '@/i18n';
import { usePanels } from '@/stores/panels';
import { useSearch } from '@/stores/search';
import { examples } from './drop';
import { openSaved, savedMenu } from './saved';

const props = defineProps<{ i: number }>();
const F = usePanels(), search = useSearch();
const tab = computed(() => F.T(props.i));

// The counts are the current ones; a search through file contents is a job, so it keeps the count it had.
onMounted(() => { for (const s of search.saved) if (!s.query.text?.trim()) search.refresh(s); });
const count = (s: SavedSearch) => {
  const h = search.hits.get(s.id);
  return h && (h.done || h.total) ? num(h.total) : '';
};
const tip = (s: SavedSearch) => t('search.rowTip', { names: localQuery(s.query.name) || t('search.allNames'), path: s.query.in });
const words = () => ({ tag: t('search.exampleWords.tag'), word: t('search.exampleWords.word') });

async function example(q: string) {
  Object.assign(tab.value, { filter: q, cursor: 0 });
  await nextTick();
  const input = document.querySelector<HTMLInputElement>(`#qf-${props.i}`);
  input?.focus();
  input?.setSelectionRange(q.length, q.length);
}
</script>

<template>
  <div class="qdrop glim-card" role="menu" @mousedown.prevent>
    <div class="glim-eyebrow">{{ t('search.menuSaved') }}<IconButton :tip="t('search.menuNew')" :glyph="glyphComponent('IconAdd')" data-run="search" @click="run('search')" /></div>
    <button v-for="s in search.saved" :key="s.id" class="qrow" :class="{ cur: tab.smart === s.id }" :data-smart="s.id" :data-tip="tip(s)"
      @click="openSaved(s)" @contextmenu.prevent.stop="savedMenu(s, $event.clientX, $event.clientY)"><G n="IconFilter" /><span>{{ s.name }}</span><span class="glim-num">{{ count(s) }}</span></button>
    <div v-if="!search.saved.length" class="qd-none muted">{{ t('search.menuNone') }}</div>
    <div class="glim-eyebrow">{{ t('search.menuExamples') }}</div>
    <button v-for="[q, say] in examples()" :key="say" class="qrow qex" :data-example="q" @click="example(q)"><b>{{ q }}</b><span class="muted">{{ t(`search.examples.${say}`, words()) }}</span></button>
  </div>
</template>
