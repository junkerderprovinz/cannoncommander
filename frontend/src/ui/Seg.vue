<script setup lang="ts" generic="V extends string | number">
// GlimStone's Selector for options written as [value, label, glyph], named by the row it stands in. A model of null
// leaves every segment unchosen. Every press is reported before the choice changes, also on the choice already made,
// because the hidden shapes and motions count exactly those.
import { Selector } from '@glimstone/vue';
import { computed, inject } from 'vue';
import { glyphComponent } from '@/core/glyphs';
import { rowLabel } from './kit';

export type SegOption<T> = readonly [value: T, label: string, glyph?: string];
const props = defineProps<{ options: readonly SegOption<V>[]; label?: string; disabled?: boolean }>();
const model = defineModel<V | null>({ required: true });
const emit = defineEmits<{ press: [value: V] }>();
const row = inject(rowLabel, () => '');

const items = computed(() => props.options.map(([v, label, g]) => ({ id: String(v), label, glyph: g ? glyphComponent(g) : undefined })));
const valueOf = (id: string) => props.options.find(([v]) => String(v) === id)![0];
const chosen = computed({
  get: () => (model.value === null ? null : String(model.value)),
  set: (id) => { model.value = valueOf(id as string); },
});

function pressed(e: MouseEvent) {
  const seg = (e.target as HTMLElement).closest<HTMLElement>('[data-sel-id]');
  if (seg && !seg.matches(':disabled')) emit('press', valueOf(seg.dataset.selId!));
}
</script>

<template>
  <Selector v-model="chosen" class="seg" :items="items" :label="label ?? row()" size="sm" activation="manual" inline button-height :disabled="disabled" @click.capture="pressed" />
</template>
