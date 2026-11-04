<script setup lang="ts">
// Draws the open context menu with GlimStone's ContextMenu. Its rows follow the actions live, so a ticked box or a
// label in another language shows at once.
import { ContextMenu } from '@glimstone/vue';
import { computed, onBeforeUnmount, onMounted } from 'vue';
import { t } from '@/i18n';
import { closeMenu, menuState, toGlim } from './menu';

const items = computed(() => (menuState.list ? toGlim(menuState.list) : []));

// ContextMenu puts its menus straight on the body without a way to name them; the prototype's steps and the shim
// find them as #menu and #submenu.
let observer: MutationObserver | undefined;
onMounted(() => {
  observer = new MutationObserver((changes) => {
    for (const c of changes) {
      for (const n of c.addedNodes) {
        if (n instanceof HTMLElement && n.classList.contains('glim-menu') && !n.id) n.id = document.getElementById('menu') ? 'submenu' : 'menu';
      }
    }
  });
  observer.observe(document.body, { childList: true });
});
onBeforeUnmount(() => observer?.disconnect());
</script>

<template>
  <ContextMenu v-if="menuState.list" :key="menuState.seq" :x="menuState.x" :y="menuState.y" :items="items" :above="menuState.above"
    :opener="menuState.anchor" :label="t('menus.label')" @close="closeMenu" />
</template>
