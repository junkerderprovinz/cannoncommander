<script setup lang="ts">
// GlimStone's Toggle, named by the row it stands in. Toggle leaves attributes on its frame, while an id or a data hook
// belongs on the switch itself, which is what a script asks for its state.
import { Toggle } from '@glimstone/vue';
import { type ComponentPublicInstance, computed, inject, onMounted, onUpdated, ref, useAttrs } from 'vue';
import { rowLabel } from './kit';

defineOptions({ inheritAttrs: false });
const props = defineProps<{ label?: string; disabled?: boolean }>();
const model = defineModel<boolean>({ required: true });
const attrs = useAttrs();
const row = inject(rowLabel, () => '');
const name = computed(() => props.label ?? row());
const toggle = ref<ComponentPublicInstance | null>(null);

function handOn() {
  const track: HTMLElement | null | undefined = toggle.value?.$el.querySelector('[role=switch]');
  if (!track) return;
  for (const [k, v] of Object.entries(attrs)) {
    if (!k.startsWith('on') && k !== 'class') track.setAttribute(k, String(v));
  }
}
onMounted(handOn);
onUpdated(handOn);
</script>

<template>
  <Toggle ref="toggle" v-model="model" :label="name" hide-label :disabled="disabled" :class="$attrs.class" />
</template>
