<script setup lang="ts">
import { FieldRow, ListRow, PathField, Pill, SubCard } from '@glimstone/vue';
import { fmtSize } from '@/core/format';
import { G, glyphOf } from '@/core/glyphs';
import { plural, t } from '@/i18n';
import { pathField } from '@/ui/picker';

const SHOWN = 8;
defineProps<{ files: File[]; st: { path: string } }>();
</script>

<template>
  <SubCard :title="plural(files.length, 'incoming.files')">
    <div class="ci-list">
      <ListRow v-for="(f, k) in files.slice(0, SHOWN)" :key="k" :name="f.name">
        <template #tile><G :n="glyphOf(f)" /></template>
        <template #badges><Pill class="glim-num">{{ fmtSize(f.size) || '0 B' }}</Pill></template>
      </ListRow>
    </div>
    <p v-if="files.length > SHOWN" class="muted">{{ plural(files.length - SHOWN, 'incoming.more') }}</p>
  </SubCard>
  <SubCard>
    <FieldRow :label="t('incoming.target')" :hint="t('incoming.targetTip')"><PathField v-model="st.path" v-bind="pathField('ci-target')" /></FieldRow>
  </SubCard>
</template>
