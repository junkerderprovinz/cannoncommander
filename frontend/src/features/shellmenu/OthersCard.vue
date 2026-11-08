<script setup lang="ts">
// The rows other programs put into the system's menu, to hide those that are never wanted.
import { Button, Card, EmptyState, FieldRow, ListRow, TextField, Toggle } from '@glimstone/vue';
import { computed, onMounted, ref } from 'vue';
import { G, glyphComponent } from '@/core/glyphs';
import { t } from '@/i18n';
import { hideEntries, loadEntries, shell, type ShellEntry, warn } from './shell';

const others = ref<ShellEntry[] | null>(null);
const filter = ref('');
const found = computed(() => {
  const words = filter.value.trim().toLowerCase();
  return (others.value ?? []).filter((e) => e.label.toLowerCase().includes(words));
});
const anyHidden = computed(() => (others.value ?? []).some((e) => e.hidden));

onMounted(async () => {
  try {
    others.value = await loadEntries();
  } catch (e) {
    others.value = [];
    warn(e);
  }
});

async function hideOthers(keys: string[]) {
  const before = others.value;
  others.value = (others.value ?? []).map((e) => ({ ...e, hidden: keys.includes(e.key) }));
  try {
    await hideEntries(keys);
  } catch (e) {
    others.value = before;
    warn(e);
  }
}

const flip = (entry: ShellEntry, shown: boolean) => hideOthers((others.value ?? []).filter((e) => (e.key === entry.key ? !shown : e.hidden)).map((e) => e.key));
</script>

<template>
  <Card v-if="shell.menu" :title="t('shellmenu.othersTitle')" :hint="t(shell.menu === 'native' ? 'shellmenu.othersHintWindows' : 'shellmenu.othersHintLinux')">
    <FieldRow v-if="(others?.length ?? 0) > 8" :label="t('shellmenu.search')"><TextField v-model="filter" :aria-label="t('shellmenu.search')" width="grow" /></FieldRow>
    <div v-if="found.length" class="sm-others">
      <ListRow v-for="e in found" :key="e.key" :name="e.label" :class="{ 'sm-off': e.hidden }" :data-entry="e.key">
        <template #tile><img v-if="e.icon" :src="e.icon" alt="" class="glyph-picture"><G v-else n="IconApp" /></template>
        <template #actions><Toggle :model-value="!e.hidden" :label="t('shellmenu.show', { name: e.label })" hide-label @update:model-value="(on: boolean) => flip(e, on)" /></template>
      </ListRow>
    </div>
    <EmptyState v-else-if="others && !others.length" nested :glyph="glyphComponent('IconMore')" :title="t('shellmenu.noneSeen')" :hint="t('shellmenu.noneSeenTip')" />
    <EmptyState v-else-if="others" nested :glyph="glyphComponent('IconSearch')" :title="t('shellmenu.noMatch')" />
    <FieldRow :label="t('shellmenu.showAll')" :hint="t('shellmenu.showAllTip')"><Button :label="t('shellmenu.showAllButton')" label-key="IconEye" :disabled="!anyHidden" @click="hideOthers([])" /></FieldRow>
  </Card>
</template>
