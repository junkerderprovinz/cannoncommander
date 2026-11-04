<script lang="ts">
// The last tab stays open when the page comes back without one named, as the gear button does.
const remembered: { id?: string } = {};
</script>

<script setup lang="ts">
// Settings: a column of tiles beside the content, the tab selector turned on its side, and only the content scrolls.
// A tile can be dragged to another place in the column; the order is kept like any other pref.
import { computed, nextTick, ref, watch } from 'vue';
import { labelOf } from '@/core/actions';
import { G } from '@/core/glyphs';
import { setPref } from '@/core/prefs';
import { go, orderedTabs, settingsTabs } from '@/core/views';
import { t } from '@/i18n';

const props = defineProps<{ arg?: unknown }>();
const tabs = computed(() => orderedTabs(settingsTabs));
const cur = computed(() => tabs.value.find((x) => x.id === (props.arg ?? remembered.id)) ?? tabs.value[0]);
watch(cur, (c) => { remembered.id = c?.id; }, { immediate: true });

const tiles = ref<HTMLElement | null>(null);
watch(() => cur.value?.id, async () => {
  await nextTick();
  tiles.value?.querySelector('.stile.cur')?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
}, { immediate: true });

const dragging = ref<string | null>(null);
const order = ref<string[] | null>(null);
const shown = computed(() => (order.value ? order.value.map((id) => tabs.value.find((x) => x.id === id)!) : tabs.value));

function start(e: DragEvent, id: string) {
  dragging.value = id;
  order.value = tabs.value.map((x) => x.id);
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move';
}
function over(e: DragEvent, id: string) {
  const ids = order.value;
  if (!dragging.value || !ids || id === dragging.value) return;
  e.preventDefault();
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
  const from = ids.indexOf(dragging.value);
  ids.splice(from, 1);
  const at = ids.indexOf(id);
  ids.splice(e.clientY < r.top + r.height / 2 ? at : at + 1, 0, dragging.value);
}
function end() {
  if (!dragging.value || !order.value) return;
  const ids = order.value;
  dragging.value = null;
  order.value = null;
  setPref('tabOrder', ids);
}
</script>

<template>
  <div v-if="!cur" class="page-inner">
    <section class="card glim-card"><span class="notch">{{ t('settings.title') }}</span><div class="card-body"><p class="muted">{{ t('settings.none') }}</p></div></section>
  </div>
  <div v-else class="settings">
    <nav ref="tiles" class="tiles" role="tablist" aria-orientation="vertical" :aria-label="t('settings.title')">
      <button v-for="tab in shown" :key="tab.id" class="stile" :class="{ cur: tab.id === cur.id, dragging: tab.id === dragging }" role="tab" :aria-selected="tab.id === cur.id" draggable="true" :data-stab="tab.id" :data-tip="labelOf(tab)"
        @click="go('settings', tab.id)" @dragstart="start($event, tab.id)" @dragover="over($event, tab.id)" @dragend="end"><G :n="tab.g" /><span>{{ labelOf(tab) }}</span></button>
    </nav>
    <div id="settings-body" class="settings-body"><div id="settings-col" class="settings-col"><component :is="cur.component" :key="cur.id" /></div></div>
  </div>
</template>
