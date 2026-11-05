<script setup lang="ts">
// The panel foot on the array: how the share keeps its files, or a warning on a disk share, and the cache filter.
import { Pill } from '@glimstone/vue';
import { computed } from 'vue';
import { run } from '@/core/actions';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
import { diskName } from '@/features/jobs/label';
import { usePanels } from '@/stores/panels';
import { useUnraid } from './state';
import { moverPool, shareBadge } from './store';

const props = defineProps<{ i: number }>();
const F = usePanels();
const U = useUnraid();
const tab = computed(() => F.T(props.i));
const w = computed(() => U.place(tab.value.path));

const badge = computed(() => {
  const at = w.value;
  if (!at?.share) return null;
  if (at.vol !== 'user') {
    const asleep = U.asleep(at.vol);
    return { label: t('unraid.diskShare'), glyph: asleep ? 'IconMoon' : 'IconDatabase', tip: t('unraid.diskShareTip', { disk: diskName(at.vol) }) + (asleep ? t('unraid.diskAsleep') : '') };
  }
  const sh = U.share(at.share);
  if (!sh) return null;
  const pool = moverPool(sh);
  const waiting = pool ? U.where(tab.value.path)?.on.find((o) => o.device === pool) : undefined;
  return shareBadge(sh, U.st?.mover?.schedule, waiting);
});
</script>

<template>
  <Pill v-if="badge" class="u-store" :data-tip="badge.tip"><G :n="badge.glyph" />{{ badge.label }}</Pill>
  <button v-if="tab.cacheOnly" class="glim-pill glim-pill--state u-filter" data-run="cacheOnly" :data-tip="t('unraid.filterOff')" @click="run('cacheOnly')">{{ t('unraid.cachePill') }}<G n="IconClose" /></button>
</template>
