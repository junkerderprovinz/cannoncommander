<script setup lang="ts">
import type { Comparison, CompareState } from '@/api/tools';
import { t } from '@/i18n';

defineProps<{ left: string; right: string; counts: Comparison['counts'] }>();
const ROWS: [CompareState, string][] = [['same', 'ok'], ['newerL', 'warn'], ['newerR', 'warn'], ['differ', 'warn'], ['onlyL', ''], ['onlyR', '']];
</script>

<template>
  <div class="well glim-well"><div class="eyebrow">{{ t('compare.and', { left, right }) }}</div>
    <table class="tbl">
      <template v-for="[s, tone] in ROWS" :key="s">
        <tr v-if="s === 'same' || counts[s]"><td>{{ t(`compare.rows.${s}`) }}</td><td class="r"><span class="pill glim-num" :class="counts[s] && tone ? tone : ''">{{ counts[s] ?? 0 }}</span></td></tr>
      </template>
    </table>
  </div>
</template>
