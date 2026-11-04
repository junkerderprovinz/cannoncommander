<script setup lang="ts" generic="V extends string">
// A field-shaped trigger that opens the shared menu under it. GlimStone's Select shows its label above the field and a
// darker well, so this keeps the prototype's look.
import { computed, ref } from 'vue';
import { G } from '@/core/glyphs';
import { menu } from '@/core/menu';

export type DropOption<T> = [value: T, label: string, glyph?: string];
const props = withDefaults(defineProps<{ options: DropOption<V>[]; size?: string }>(), { size: 'm' });
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
defineExpose({ el });
</script>

<template>
  <button ref="el" class="field a-dd" :class="size" :data-val="cur?.[0]" @click.stop="open"><span><G v-if="cur?.[2]" :n="cur[2]" />{{ cur?.[1] }}</span><G n="IconExpand" /></button>
</template>
