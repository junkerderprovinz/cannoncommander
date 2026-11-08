<script setup lang="ts">
// The folders every user has, above the pinned ones, as in Explorer's quick access and the places of Linux file
// managers.
import { SidebarRow } from '@glimstone/vue';
import { glyphComponent } from '@/core/glyphs';
import { host, windowsPaths } from '@/core/host';
import { menu } from '@/core/menu';
import { folderItems } from '@/files/menus';
import { nativePath } from '@/files/paths';
import { t } from '@/i18n';
import { usePanels } from '@/stores/panels';
import { type PlaceKind, places } from './store';

const F = usePanels();
const GLYPHS: Record<PlaceKind, string> = {
  home: 'IconHome', desktop: 'IconVM', documents: 'IconFileDoc', downloads: 'IconDownload', pictures: 'IconFileImage', music: 'IconFileAudio', videos: 'IconFileVideo',
};
</script>

<template>
  <template v-if="host.desktop">
    <SidebarRow v-for="p in places.folders" :key="p.path" :label="p.label || t(`places.${p.kind}`)" :glyph="glyphComponent(GLYPHS[p.kind])" :active="F.T().path === p.path"
      :data-go="p.path" :data-place="p.kind" :data-tip="nativePath(p.path, windowsPaths())" @click="F.go(p.path)" @contextmenu.prevent.stop="menu($event.clientX, $event.clientY, folderItems(p.path))" />
  </template>
</template>
