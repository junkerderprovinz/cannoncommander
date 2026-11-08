<script setup lang="ts">
// The tile Kontextmenü: the app's own menus row by row, each to hide or to move within its group, and in the desktop
// app the rows other programs put into the system's menu.
import { Button, Card, FieldRow, IconButton, ListRow, Toggle } from '@glimstone/vue';
import { defineAsyncComponent } from 'vue';
import { G, glyphComponent } from '@/core/glyphs';
import { host } from '@/core/host';
import { t } from '@/i18n';
import { customised, groups, isHidden, type MenuKind, move, resetMenu, rowOf, setHidden } from './custom';

// The container build leaves the system's half out.
const OthersCard = import.meta.env.DESKTOP ? defineAsyncComponent(() => import('./OthersCard.vue')) : null;

const KINDS: MenuKind[] = ['row', 'list'];
const shownGroups = (kind: MenuKind) => groups(kind).map((g) => g.map(rowOf).filter((r) => r !== null)).filter((g) => g.length);
</script>

<template>
  <Card v-for="kind in KINDS" :key="kind" :title="t(`shellmenu.${kind}Title`)" :hint="t(`shellmenu.${kind}Hint`)">
    <div class="sm-groups">
      <div v-for="(g, gi) in shownGroups(kind)" :key="gi" class="sm-group">
        <ListRow v-for="(r, i) in g" :key="r.id" :name="r.label" :class="{ 'sm-off': isHidden(kind, r.id) }" :data-menu-row="`${kind}:${r.id}`">
          <template #tile><G :n="r.g" /></template>
          <template #actions>
            <IconButton class="sm-move" :glyph="glyphComponent('IconMoveUp')" :tip="t('shellmenu.up', { name: r.label })" :disabled="i === 0" @click="move(kind, r.id, -1)" />
            <IconButton class="sm-move" :glyph="glyphComponent('IconMoveDown')" :tip="t('shellmenu.down', { name: r.label })" :disabled="i === g.length - 1" @click="move(kind, r.id, 1)" />
            <Toggle :model-value="!isHidden(kind, r.id)" :label="t('shellmenu.show', { name: r.label })" hide-label @update:model-value="(on: boolean) => setHidden(kind, r.id, !on)" />
          </template>
        </ListRow>
      </div>
    </div>
    <FieldRow :label="t('shellmenu.reset')" :hint="t('shellmenu.resetTip')"><Button :label="t('shellmenu.resetButton')" label-key="IconResetArrow" :disabled="!customised(kind)" @click="resetMenu(kind)" /></FieldRow>
  </Card>
  <OthersCard v-if="OthersCard && host.desktop" />
</template>
