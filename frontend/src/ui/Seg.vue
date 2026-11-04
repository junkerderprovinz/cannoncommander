<script setup lang="ts" generic="V extends string | number">
// A row of choices with the prototype's raised segments. A model of null leaves every segment unpressed.
import { G } from '@/core/glyphs';

export type SegOption<T> = readonly [value: T, label: string, glyph?: string];
defineProps<{ options: readonly SegOption<V>[] }>();
const model = defineModel<V | null>({ required: true });
const emit = defineEmits<{ press: [value: V] }>();

function press(v: V) {
  model.value = v;
  emit('press', v);
}
</script>

<template>
  <div class="seg">
    <button v-for="o in options" :key="o[0]" type="button" :aria-pressed="o[0] === model" :data-v="o[0]" @click="press(o[0])"><G v-if="o[2]" :n="o[2]" />{{ o[1] }}</button>
  </div>
</template>
