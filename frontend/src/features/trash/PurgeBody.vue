<script setup lang="ts">
import { SubCard, Table } from '@glimstone/vue';
import { fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import type { TrashItem } from '@/api/ops';
import { plural, t } from '@/i18n';

defineProps<{ list: TrashItem[]; what: string; size: number; whole?: string }>();
</script>

<template>
  <SubCard :title="`${what}, ${fmtSize(size)}`">
    <Table :label="what" :columns="[{ key: 'name' }, { key: 'size', end: true }]" :rows="list.slice(0, 8).map((e) => ({ name: e.name, size: fmtSize(e.size) }))" :more="list.length > 8 ? plural(list.length - 8, 'trash.andMore') : undefined" />
    <div v-if="whole" class="ce-ask"><G n="IconEmpty" /><span>{{ whole }}</span></div>
    <div class="ce-ask"><G n="IconWarning" /><span>{{ t('trash.gone') }}</span></div>
  </SubCard>
</template>
