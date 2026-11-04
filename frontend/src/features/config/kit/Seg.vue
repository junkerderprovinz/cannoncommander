<script setup lang="ts" generic="V extends string | number">
// A row of choices. GlimStone's Selector sits lower in a well and makes every segment as wide as the widest, so this
// keeps the prototype's raised segments. Every press is reported, also on the choice already made, because the hidden
// shapes and motions count exactly those.
import { G } from '@/core/glyphs';

export type SegOption<T> = [value: T, label: string, glyph?: string];
defineProps<{ options: SegOption<V>[] }>();
const model = defineModel<V>({ required: true });
const emit = defineEmits<{ press: [value: V] }>();

function press(v: V) {
  emit('press', v);
  model.value = v;
}
</script>

<template>
  <div class="seg">
    <button v-for="o in options" :key="o[0]" :aria-pressed="o[0] === model" :data-v="o[0]" @click="press(o[0])"><G v-if="o[2]" :n="o[2]" />{{ o[1] }}</button>
  </div>
</template>
