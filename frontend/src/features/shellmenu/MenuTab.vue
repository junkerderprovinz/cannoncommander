<script setup lang="ts">
// The tile Kontextmenü: the app's own menus row by row, each to hide or to move within its group, and in the desktop
// app the rows other programs put into the system's menu, to hide those that are never wanted.
import { Button, Card, EmptyState, FieldRow, IconButton, ListRow, TextField, Toggle } from '@glimstone/vue';
import { computed, onMounted, ref } from 'vue';
import { G, glyphComponent } from '@/core/glyphs';
import { host } from '@/core/host';
import { t } from '@/i18n';
import { customised, groups, isHidden, type MenuKind, move, resetMenu, rowOf, setHidden } from './custom';
import { hideEntries, loadEntries, shell, type ShellEntry, warn } from './shell';

const KINDS: MenuKind[] = ['row', 'list'];
const shownGroups = (kind: MenuKind) => groups(kind).map((g) => g.map(rowOf).filter((r) => r !== null)).filter((g) => g.length);

const others = ref<ShellEntry[] | null>(null);
const filter = ref('');
const found = computed(() => {
  const words = filter.value.trim().toLowerCase();
  return (others.value ?? []).filter((e) => e.label.toLowerCase().includes(words));
});
const anyHidden = computed(() => (others.value ?? []).some((e) => e.hidden));

onMounted(async () => {
  if (!host.desktop) return;
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
  <Card v-for="kind in KINDS" :key="kind" :title="t(`shellmenu.${kind}Title`)" :hint="t(`shellmenu.${kind}Hint`)">
    <div class="sm-groups">
      <div v-for="(g, gi) in shownGroups(kind)" :key="gi" class="sm-group">
        <ListRow v-for="(r, i) in g" :key="r.id" :name="r.label" :class="{ 'sm-off': isHidden(kind, r.id) }" :data-menu-row="`${kind}:${r.id}`">
          <template #tile><G :n="r.g" /></template>
          <template #actions>
            <IconButton :glyph="glyphComponent('IconMoveUp')" :tip="t('shellmenu.up', { name: r.label })" :disabled="i === 0" @click="move(kind, r.id, -1)" />
            <IconButton :glyph="glyphComponent('IconMoveDown')" :tip="t('shellmenu.down', { name: r.label })" :disabled="i === g.length - 1" @click="move(kind, r.id, 1)" />
            <Toggle :model-value="!isHidden(kind, r.id)" :label="t('shellmenu.show', { name: r.label })" hide-label @update:model-value="(on: boolean) => setHidden(kind, r.id, !on)" />
          </template>
        </ListRow>
      </div>
    </div>
    <FieldRow :label="t('shellmenu.reset')" :hint="t('shellmenu.resetTip')"><Button :label="t('shellmenu.resetButton')" label-key="IconResetArrow" :disabled="!customised(kind)" @click="resetMenu(kind)" /></FieldRow>
  </Card>
  <Card v-if="host.desktop && shell.menu" :title="t('shellmenu.othersTitle')" :hint="t(shell.menu === 'native' ? 'shellmenu.othersHintWindows' : 'shellmenu.othersHintLinux')">
    <TextField v-if="(others?.length ?? 0) > 8" v-model="filter" :label="t('shellmenu.search')" width="grow" />
    <div v-if="found.length" class="sm-others">
      <ListRow v-for="e in found" :key="e.key" :name="e.label" :tip="e.key.replace(/^(verb|text):/, '')" :class="{ 'sm-off': e.hidden }" :data-entry="e.key">
        <template #tile><img v-if="e.icon" :src="e.icon" alt="" class="glyph-picture"><G v-else n="IconApp" /></template>
        <template #actions><Toggle :model-value="!e.hidden" :label="t('shellmenu.show', { name: e.label })" hide-label @update:model-value="(on: boolean) => flip(e, on)" /></template>
      </ListRow>
    </div>
    <EmptyState v-else-if="others && !others.length" nested :glyph="glyphComponent('IconMore')" :title="t('shellmenu.noneSeen')" :hint="t('shellmenu.noneSeenTip')" />
    <EmptyState v-else-if="others" nested :glyph="glyphComponent('IconSearch')" :title="t('shellmenu.noMatch')" />
    <FieldRow :label="t('shellmenu.showAll')" :hint="t('shellmenu.showAllTip')"><Button :label="t('shellmenu.showAllButton')" label-key="IconEye" :disabled="!anyHidden" @click="hideOthers([])" /></FieldRow>
  </Card>
</template>
