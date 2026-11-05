<script setup lang="ts">
import { Pill, SubCard, Table, type PillTone } from '@glimstone/vue';
import type { Comparison, CompareState } from '@/api/tools';
import { t } from '@/i18n';

defineProps<{ left: string; right: string; counts: Comparison['counts'] }>();
const ROWS: [CompareState, PillTone][] = [['same', 'ok'], ['newerL', 'warn'], ['newerR', 'warn'], ['differ', 'warn'], ['onlyL', 'neutral'], ['onlyR', 'neutral']];
</script>

<template>
  <SubCard :title="t('compare.and', { left, right })">
    <Table :label="t('compare.title')" :columns="[{ key: 'word' }, { key: 'count', end: true }]" :rows="ROWS.filter(([s]) => s === 'same' || counts[s]).map(([s, tone]) => ({ word: t(`compare.rows.${s}`), count: counts[s] ?? 0, tone: counts[s] ? tone : 'neutral' as PillTone }))">
      <template #cell="{ row, column, value }"><Pill v-if="column.key === 'count'" :tone="row.tone">{{ value }}</Pill><template v-else>{{ value }}</template></template>
    </Table>
  </SubCard>
</template>
