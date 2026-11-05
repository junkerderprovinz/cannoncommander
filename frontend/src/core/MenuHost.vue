<script setup lang="ts">
// Draws the open context menu with GlimStone's ContextMenu. Its rows follow the actions live, so a ticked box or a
// label in another language shows at once.
import { ContextMenu } from '@glimstone/vue';
import { computed } from 'vue';
import { t } from '@/i18n';
import { closeMenu, menuState, toGlim } from './menu';

const items = computed(() => (menuState.list ? toGlim(menuState.list) : []));
</script>

<template>
  <ContextMenu v-if="menuState.list" id="menu" :key="menuState.seq" :x="menuState.x" :y="menuState.y" :items="items" :above="menuState.above"
    :opener="menuState.anchor" :label="t('menus.label')" @close="closeMenu" />
</template>
