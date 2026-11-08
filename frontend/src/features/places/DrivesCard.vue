<script setup lang="ts">
// The computer's drives in a card of their own: sticks, discs, disk images and network shares, each with an eject button
// where it can be ejected. A volume Linux has not mounted yet is mounted by the click that opens it.
import { Card, SidebarRow } from '@glimstone/vue';
import { G, glyphComponent } from '@/core/glyphs';
import { host, windowsPaths } from '@/core/host';
import { menu } from '@/core/menu';
import { driveGlyph, driveName } from '@/files/drives';
import { nativePath } from '@/files/paths';
import { t } from '@/i18n';
import { usePanels } from '@/stores/panels';
import { driveMenu, eject, openVolume } from './actions';
import { places, type Volume } from './store';

const F = usePanels();
const tip = (v: Volume) => v.remote || (v.path ? nativePath(v.path, windowsPaths()) : t('places.notMounted'));
</script>

<template>
  <Card v-if="host.desktop && places.volumes.length" class="side-card side-drives" :title="t('places.drives')">
    <div id="drives">
      <SidebarRow v-for="v in places.volumes" :key="v.id" :label="driveName(v)" :glyph="glyphComponent(driveGlyph(v.kind))" :active="!!v.path && F.T().path === v.path"
        :class="{ unmounted: !v.path }" :data-drive="v.id" :data-tip="tip(v)" @click="openVolume(v)" @contextmenu.prevent.stop="menu($event.clientX, $event.clientY, driveMenu(v))">
        <template #after>
          <button v-if="v.eject" class="unpin" :data-eject="v.id" :data-tip="t('places.eject')" :aria-label="t('places.ejectLabel', { name: driveName(v) })" @click.stop="eject(v)"><G n="IconEject" /></button>
        </template>
      </SidebarRow>
    </div>
  </Card>
</template>
