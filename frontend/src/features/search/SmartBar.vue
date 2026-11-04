<script setup lang="ts">
import { computed } from 'vue';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
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
  <div v-if="s" class="smart-bar"><G n="IconFilter" /><span>{{ t('search.bar') }} <b>{{ s.name }}</b>{{ t('search.barRest', { n: F.entries(i).length, path: s.query.in }) }}</span><button class="btn icon" data-smart-close :data-tip="t('search.barClose')" :aria-label="t('search.barClose')" @click="leave"><G n="IconClose" /></button></div>
</template>
