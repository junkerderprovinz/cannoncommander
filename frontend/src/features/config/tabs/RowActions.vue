<script setup lang="ts">
// The buttons GlimStone's ConnectionRow takes as `actions` from its next version, put first into the row's own bar
// until then. It stands right after its ConnectionRow, and the row's `actions` replace it one for one.
import { Button } from '@glimstone/vue';
import { onBeforeUnmount, onMounted, ref, shallowRef } from 'vue';

export interface RowAction {
  id: string;
  label: string;
  labelKey: string | null;
  busy?: boolean;
  tone?: 'ok' | 'danger';
  title?: string;
  disabled?: boolean;
  onClick: () => void;
}

defineProps<{ actions: RowAction[] }>();

const marker = ref<HTMLElement | null>(null);
const bar = shallowRef<HTMLElement | null>(null);
onMounted(() => {
  const own = marker.value?.previousElementSibling?.querySelector('.glim-connection-actions');
  if (!own) return;
  const box = Object.assign(document.createElement('span'), { className: 'a-row-actions' });
  own.prepend(box);
  bar.value = box;
});
onBeforeUnmount(() => bar.value?.remove());
</script>

<template>
  <span ref="marker" hidden />
  <Teleport v-if="bar" :to="bar">
    <Button v-for="a in actions" :key="a.id" :label="a.label" :label-key="a.labelKey" :busy="a.busy" :tone="a.tone" :title="a.title"
      :disabled="a.disabled" :compact-below="640" :data-connection="a.id" @click="a.onClick" />
  </Teleport>
</template>
