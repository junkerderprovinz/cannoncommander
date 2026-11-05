<script setup lang="ts">
import { Pill, type PillTone } from '@glimstone/vue';
import type { Comparison, CompareState } from '@/api/tools';
import { t } from '@/i18n';

defineProps<{ left: string; right: string; counts: Comparison['counts'] }>();
const ROWS: [CompareState, PillTone][] = [['same', 'ok'], ['newerL', 'warn'], ['newerR', 'warn'], ['differ', 'warn'], ['onlyL', 'neutral'], ['onlyR', 'neutral']];
</script>

<template>
  <div class="well glim-well"><div class="eyebrow">{{ t('compare.and', { left, right }) }}</div>
    <table class="tbl">
      <template v-for="[s, tone] in ROWS" :key="s">
        <tr v-if="s === 'same' || counts[s]"><td>{{ t(`compare.rows.${s}`) }}</td><td class="r"><Pill class="glim-num" :tone="counts[s] ? tone : 'neutral'">{{ counts[s] ?? 0 }}</Pill></td></tr>
      </template>
    </table>
  </div>
</template>
