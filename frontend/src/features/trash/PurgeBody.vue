<script setup lang="ts">
import { fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import type { TrashItem } from '@/api/ops';
import { t } from '@/i18n';

defineProps<{ list: TrashItem[]; what: string; size: number }>();
</script>

<template>
  <div class="well glim-well"><div class="eyebrow">{{ what }}, {{ fmtSize(size) }}</div>
    <table class="tbl">
      <tr v-for="e in list.slice(0, 8)" :key="e.id"><td>{{ e.name }}</td><td class="glim-num r">{{ fmtSize(e.size) }}</td></tr>
      <tr v-if="list.length > 8"><td colspan="2">{{ t('trash.andMore', { n: list.length - 8 }) }}</td></tr>
    </table>
    <div class="ce-ask"><G n="IconWarning" /><span>{{ t('trash.gone') }}</span></div>
  </div>
</template>
