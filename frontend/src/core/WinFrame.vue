<script setup lang="ts">
// One window of the stack, drawn by GlimStone's Window. A locked window ignores Escape and a click beside it.
import { windowFocusTarget } from '@glimstone/core';
import { Window, type WindowAction } from '@glimstone/vue';
import { computed, onMounted } from 'vue';
import { t } from '@/i18n';
import { glyphComponent } from './glyphs';
import { type FootButton, isWayOut, type WinEntry } from './windows';

const props = defineProps<{ w: WinEntry }>();

const start = computed(() => props.w.foot.find((f) => f.start !== undefined)?.start);
const actions = computed<WindowAction[]>(() => props.w.foot.filter((f) => f.start === undefined).map((f) => ({
  label: f.label!,
  labelKey: null,
  glyph: f.g ? glyphComponent(f.g) : undefined,
  primary: f.primary,
  disabled: f.disabled,
  wayOut: isWayOut(f),
  onClick: () => { press(f); return false; },
})));
const text = computed(() => ({ cancel: t('core.cancel'), close: t('core.close') }));

function press(f: FootButton) {
  if (!f.run || f.run(props.w.el!) !== false) props.w.close();
}
function asked() {
  if (!props.w.locked) props.w.close();
}

onMounted(() => {
  const card = document.querySelector<HTMLElement>(`[data-win="${props.w.id}"]`)!;
  props.w.el = card.parentElement;
  props.w.onOpen?.(props.w.el!, props.w.close);
  // A body that fills in the foot as it mounts has no buttons yet when Window looks for the one to focus.
  setTimeout(() => windowFocusTarget(card)?.focus(), 0);
});
</script>

<template>
  <Window class="win" :class="w.cls" :data-win="w.id" :title="w.title" :actions="actions" :start="start" :size="w.full ? 'full' : w.wide ? 'wide' : 'normal'" :text="text" @close="asked">
    <div v-if="typeof w.body === 'string'" v-html="w.body"></div>
    <component :is="w.body" v-else v-bind="w.props" :close="w.close" :win="w" />
  </Window>
</template>
