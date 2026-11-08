<script setup lang="ts">
// Explorer's preview and details pane at the right of the panels: what the cursor of the active panel stands on,
// drawn by the viewer's preview, with the details the metadata adds below it.
import { Card, EmptyState, FieldRow, SubCard } from '@glimstone/vue';
import { computed, watch } from 'vue';
import { glyphComponent } from '@/core/glyphs';
import Preview from '@/features/viewer/Preview.vue';
import { t } from '@/i18n';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import { COL, columnLabel } from './columns';
import { join } from './paths';

// The preview names size, format, date and rights already; these are what a file may add.
const MORE = ['dimensions', 'duration', 'pages', 'title', 'artist', 'album', 'camera', 'taken', 'place', 'author', 'words'];

const F = usePanels();
const listing = useListing();
const k = computed(() => F.cursorEntry());
const folder = computed(() => k.value?.at || F.T().path);
watch([k, folder], () => {
  if (k.value && !k.value.up && !k.value.dir) listing.wantMeta([join(folder.value, k.value.name)], MORE.filter((id) => COL[id]?.meta));
}, { immediate: true });
const more = computed(() => (k.value && !k.value.dir ? MORE.map((id) => [columnLabel(id), COL[id]!.value(k.value!, folder.value)] as const).filter(([, v]) => v) : []));
</script>

<template>
  <Card class="sidepane glim-card" :title="t('actions.sidePane')">
    <div class="quickview">
      <EmptyState v-if="!k || k.up" nested :glyph="glyphComponent('IconEye')" :title="t('files.sidePaneEmpty')" />
      <template v-else>
        <Preview :key="folder + '/' + k.name" :n="k" :folder="folder" />
        <SubCard v-if="more.length" class="cv-details">
          <FieldRow v-for="[label, value] in more" :key="label" :label="label" :value="value" />
        </SubCard>
      </template>
    </div>
  </Card>
</template>
