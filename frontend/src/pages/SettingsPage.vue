<script lang="ts">
// The last tab stays open when the page comes back without one named, as the gear button does.
const remembered: { id?: string } = {};
</script>

<script setup lang="ts">
// Settings: a column of tiles beside the content, the tab selector turned on its side, and only the content scrolls.
// A long press lifts a tile to carry it to another place in the column; the order is kept like any other pref.
import { Card } from '@glimstone/vue';
import { computed, nextTick, ref, watch } from 'vue';
import { labelOf } from '@/core/actions';
import { G } from '@/core/glyphs';
import { setPref } from '@/core/prefs';
import { go, orderedTabs, settingsTabs } from '@/core/views';
import { t } from '@/i18n';
import { useReorder } from '@/ui/reorder';

const props = defineProps<{ arg?: unknown }>();
const tabs = computed(() => orderedTabs(settingsTabs).filter((x) => !x.hidden?.()));
// A link to a tile left out here opens Allgemein.
const cur = computed(() => {
  const want = props.arg ?? remembered.id;
  const left = settingsTabs.some((x) => x.id === want && x.hidden?.());
  return tabs.value.find((x) => x.id === (left ? 'general' : want)) ?? tabs.value[0];
});
watch(cur, (c) => { remembered.id = c?.id; }, { immediate: true });

const tiles = ref<HTMLElement | null>(null);
watch(() => cur.value?.id, async () => {
  await nextTick();
  tiles.value?.querySelector('.stile.cur')?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
}, { immediate: true });

const drag = useReorder({ ids: () => tabs.value.map((x) => x.id), list: tiles, attr: 'data-stab', arm: 'hold', onReorder: (ids) => setPref('tabOrder', ids) });
const shown = computed(() => drag.order.value.map((id) => tabs.value.find((x) => x.id === id)!));

function open(e: MouseEvent, id: string) {
  if (!drag.endsDrag(e)) go('settings', id);
}
</script>

<template>
  <div v-if="!cur" class="page-inner">
    <Card :title="t('settings.title')"><p class="muted">{{ t('settings.none') }}</p></Card>
  </div>
  <div v-else class="settings">
    <nav ref="tiles" class="tiles" role="tablist" aria-orientation="vertical" :aria-label="t('settings.title')">
      <button v-for="tab in shown" :key="tab.id" class="stile" :class="[drag.look(tab.id), { cur: tab.id === cur.id }]" role="tab" :aria-selected="tab.id === cur.id" :data-stab="tab.id" :data-tip="labelOf(tab)"
        @click="open($event, tab.id)" @pointerdown="drag.press($event, tab.id)"><G :n="tab.g" /><span>{{ labelOf(tab) }}</span></button>
    </nav>
    <div id="settings-body" class="settings-body"><div id="settings-col" class="settings-col"><component :is="cur.component" :key="cur.id" /></div></div>
  </div>
</template>
