<script setup lang="ts" generic="V extends string">
// A field-shaped trigger that opens the shared menu under it, as the prototype's dropdown does.
import { computed, ref } from 'vue';
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
</script>

<template>
  <button ref="el" type="button" class="field a-dd" :class="size" :data-val="cur?.[0]" @click.stop="open"><span><G v-if="cur?.[2]" :n="cur[2]" />{{ cur?.[1] }}</span><G n="IconExpand" /></button>
</template>
