<script setup lang="ts">
// Settings: a column of tiles beside the content, the tab selector turned on its side, and only the content scrolls.
import { computed } from 'vue';
import { labelOf } from '@/core/actions';
import { G } from '@/core/glyphs';
import { go, orderedTabs, settingsTabs } from '@/core/views';
import { t } from '@/i18n';

const props = defineProps<{ arg?: unknown }>();
const tabs = computed(() => orderedTabs(settingsTabs));
const cur = computed(() => tabs.value.find((x) => x.id === props.arg) ?? tabs.value[0]);
</script>

<template>
  <div v-if="!cur" class="page-inner">
    <section class="card glim-card"><span class="notch">{{ t('settings.title') }}</span><div class="card-body"><p class="muted">{{ t('settings.none') }}</p></div></section>
  </div>
  <div v-else class="settings">
    <nav class="tiles" role="tablist" aria-orientation="vertical" :aria-label="t('settings.title')">
      <button v-for="tab in tabs" :key="tab.id" class="stile" :class="{ cur: tab.id === cur.id }" role="tab" :aria-selected="tab.id === cur.id" draggable="true" :data-stab="tab.id" :data-tip="labelOf(tab)" @click="go('settings', tab.id)"><G :n="tab.g" /><span>{{ labelOf(tab) }}</span></button>
    </nav>
    <div id="settings-body" class="settings-body"><div id="settings-col" class="settings-col"><component :is="cur.component" /></div></div>
  </div>
</template>
