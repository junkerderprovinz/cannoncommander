<script setup lang="ts">
// Linux has no dialog of its own to pick a program, so this lists every program the desktop shows in its menus.
import { EmptyState, ListRow, TextField } from '@glimstone/vue';
import { computed, onMounted, ref } from 'vue';
import { call, q } from '@/api/client';
import { G, glyphComponent } from '@/core/glyphs';
import { t } from '@/i18n';
import { type App, type Apps, openWith, warn } from './apps';

const props = defineProps<{ path: string; close: () => void }>();
const apps = ref<App[] | null>(null);
const filter = ref('');
const shown = computed(() => {
  const words = filter.value.trim().toLowerCase();
  return (apps.value ?? []).filter((a) => a.name.toLowerCase().includes(words));
});

onMounted(async () => {
  try {
    apps.value = (await call<Apps>('/openwith' + q({ path: props.path, every: 'true' }))).body?.apps ?? [];
  } catch (e) {
    apps.value = [];
    warn(e);
  }
});

function pick(app: App) {
  props.close();
  void openWith(props.path, app, true);
}
</script>

<template>
  <div class="ow-choose">
    <TextField v-model="filter" :label="t('openwith.search')" width="fill" />
    <div v-if="shown.length" class="ow-apps">
      <ListRow v-for="a in shown" :key="a.id" :name="a.name" :data-app="a.id" @click="pick(a)">
        <template #tile><img v-if="a.icon" :src="a.icon" alt="" class="glyph-picture" /><G v-else n="IconApp" /></template>
      </ListRow>
    </div>
    <EmptyState v-else-if="apps" nested :glyph="glyphComponent('IconApp')" :title="t('openwith.none')" />
  </div>
</template>

