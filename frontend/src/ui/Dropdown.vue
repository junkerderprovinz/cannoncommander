<script setup lang="ts" generic="V extends string">
// GlimStone's select trigger over the shared context menu, so the options are the same rows as every other menu. The
// wheel steps through them while the trigger has the focus (rule 14).
import { enableWheelStep, stepIndex } from '@glimstone/core';
import { IconExpand } from '@glimstone/vue';
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { G } from '@/core/glyphs';
import { menu } from '@/core/menu';

export type DropOption<T> = readonly [value: T, label: string, glyph?: string];
const props = withDefaults(defineProps<{ options: readonly DropOption<V>[]; size?: string }>(), { size: 'm' });
const model = defineModel<V>({ required: true });
const el = ref<HTMLButtonElement | null>(null);
const cur = computed(() => props.options.find((o) => o[0] === model.value) ?? props.options[0]);

function open() {
  const r = el.value!.getBoundingClientRect();
  menu(r.left, r.bottom + 6, props.options.map(([v, label, g]) => ({
    label, g, radio: true, checked: () => v === model.value,
    run: () => { model.value = v; el.value?.focus(); },
  })));
}

let stopWheel = () => {};
onMounted(() => {
  stopWheel = enableWheelStep(el.value!, (delta) => {
    const at = props.options.findIndex((o) => o[0] === model.value);
    model.value = props.options[stepIndex(props.options.length, at, delta)]![0];
  });
});
onBeforeUnmount(() => stopWheel());
defineExpose({ el });
</script>

<template>
  <button ref="el" type="button" class="glim-select-trigger" :class="size" :data-val="cur?.[0]" :data-dd="JSON.stringify(options)" @click.stop="open">
    <span v-if="cur?.[2]" class="glim-select-glyph"><G :n="cur[2]" /></span>
    <span class="glim-select-value"><span>{{ cur?.[1] }}</span></span>
    <span class="glim-select-indicator"><IconExpand /></span>
  </button>
</template>
