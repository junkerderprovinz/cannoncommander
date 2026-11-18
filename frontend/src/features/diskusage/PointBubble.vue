<script setup lang="ts">
// The app's bubble at the pointer, for a treemap tile or a day of the chart, which have more to say than one data-tip
// line. It flips above the pointer where the window ends.
import { nextTick, ref, watch } from 'vue';

const props = defineProps<{ at: { x: number; y: number } | null }>();
const el = ref<HTMLElement | null>(null);
const pos = ref({ left: 0, top: 0 });

watch(() => props.at && { ...props.at }, async (at) => {
  if (!at) return;
  pos.value = { left: at.x + 12, top: at.y + 18 };
  await nextTick();
  const b = el.value?.getBoundingClientRect();
  if (!b) return;
  pos.value = {
    left: Math.max(8, Math.min(at.x - b.width / 2, innerWidth - b.width - 8)),
    top: at.y + b.height + 22 > innerHeight ? at.y - b.height - 14 : at.y + 18,
  };
}, { immediate: true });
</script>

<template>
  <Teleport to="body">
    <div v-if="at" ref="el" class="bubble du-tip" role="tooltip" :style="{ left: `${pos.left}px`, top: `${pos.top}px` }"><slot /></div>
  </Teleport>
</template>
