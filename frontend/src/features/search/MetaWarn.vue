<script setup lang="ts">
// A field test the search does not know finds nothing, so the panel says which one and what it would know.
import { computed } from 'vue';
import { G } from '@/core/glyphs';
import { prefs } from '@/core/prefs';
import { t } from '@/i18n';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';

const props = defineProps<{ i: number }>();
const F = usePanels(), listing = useListing();
const unknown = computed(() => {
  const tab = F.T(props.i);
  return tab.filter && tab.deep !== false ? listing.search(tab.path, tab.filter, !!prefs.showHidden).unknown ?? [] : [];
});
</script>

<template>
  <div v-if="unknown.length" class="smart-bar meta-warn"><G n="IconInfo" /><span>{{ t('search.unknownFields', { fields: unknown.map((f) => t('search.unknownField', { f })).join(', ') }) }}</span></div>
</template>
