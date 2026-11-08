<script setup lang="ts">
// The folders used last, folded into one row until someone opens it, so the pinned folders stay in view.
import { SidebarRow } from '@glimstone/vue';
import { computed, ref, watch } from 'vue';
import { G, glyphComponent } from '@/core/glyphs';
import { host, rootName, windowsPaths } from '@/core/host';
import { menu } from '@/core/menu';
import { recentFolders } from '@/files/frequent';
import { folderItems } from '@/files/menus';
import { nativePath, split } from '@/files/paths';
import { t } from '@/i18n';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import { useUser } from '@/stores/user';
import { places, visited } from './store';

const KEY = 'cannoncommander-recent-open';
const F = usePanels(), listing = useListing(), user = useUser();
const open = ref((() => { try { return localStorage.getItem(KEY) === '1'; } catch { return false; } })());
watch(open, (on) => { try { localStorage.setItem(KEY, on ? '1' : '0'); } catch { /* it opens folded next time */ } });

const rows = computed(() => {
  void visited.value;
  const shown = ['/', ...places.folders.map((p) => p.path), ...user.bookmarks.map((b) => b.path)];
  return recentFolders(6, shown).map((path) => ({ path, name: listing.placeLabel(path) || split(path).pop() || rootName() }));
});
</script>

<template>
  <template v-if="host.desktop && rows.length">
    <SidebarRow :label="t('places.recent')" :glyph="glyphComponent('IconClock')" data-recent @click="open = !open">
      <template #after>
        <span class="tree-tog" :class="{ open }" :aria-label="open ? t('side.collapse') : t('side.expand')"><G n="IconExpand" /></span>
      </template>
    </SidebarRow>
    <template v-if="open">
      <SidebarRow v-for="r in rows" :key="r.path" :depth="1" :label="r.name" :glyph="glyphComponent('IconFolder')" :active="F.T().path === r.path"
        :data-go="r.path" :data-tip="nativePath(r.path, windowsPaths())" @click="F.reach(r.path)" @contextmenu.prevent.stop="menu($event.clientX, $event.clientY, folderItems(r.path))" />
    </template>
  </template>
</template>
