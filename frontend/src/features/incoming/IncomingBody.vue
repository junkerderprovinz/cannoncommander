<script setup lang="ts">
import { FieldRow, ListRow, PathField, Pill, SubCard } from '@glimstone/vue';
import { fmtSize } from '@/core/format';
import { G, glyphOf } from '@/core/glyphs';
import { t } from '@/i18n';
import { pathField } from '@/ui/picker';

const SHOWN = 8;
defineProps<{ files: File[]; st: { path: string } }>();
</script>

<template>
  <SubCard :title="files.length === 1 ? t('incoming.one') : t('incoming.many', { n: files.length })">
    <div class="ci-list">
      <ListRow v-for="(f, k) in files.slice(0, SHOWN)" :key="k" :name="f.name">
        <template #tile><G :n="glyphOf(f)" /></template>
        <template #badges><Pill class="glim-num">{{ fmtSize(f.size) || '0 B' }}</Pill></template>
      </ListRow>
    </div>
    <p v-if="files.length > SHOWN" class="muted">{{ t('incoming.more', { n: files.length - SHOWN }) }}</p>
  </SubCard>
  <SubCard>
    <FieldRow :label="t('incoming.target')" :hint="t('incoming.targetTip')"><PathField v-model="st.path" v-bind="pathField('ci-target')" /></FieldRow>
  </SubCard>
</template>
