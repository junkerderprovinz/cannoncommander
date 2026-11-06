<script setup lang="ts">
import { IconButton } from '@glimstone/vue';
import { computed } from 'vue';
import { G, glyphComponent } from '@/core/glyphs';
import { plural, t } from '@/i18n';
import { usePanels } from '@/stores/panels';
import { useSearch } from '@/stores/search';

const props = defineProps<{ i: number }>();
const F = usePanels(), search = useSearch();
const s = computed(() => search.byId(F.T(props.i).smart));

function leave() {
  F.T(props.i).smart = null;
  F.active = props.i;
}
</script>

<template>
  <div v-if="s" class="smart-bar"><G n="IconFilter" /><span>{{ t('search.bar') }} <b>{{ s.name }}</b>{{ plural(F.entries(i).length, 'search.barRest', { path: s.query.in }) }}</span><IconButton :tip="t('search.barClose')" :glyph="glyphComponent('IconClose')" data-smart-close @click="leave" /></div>
</template>
