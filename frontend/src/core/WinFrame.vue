<script setup lang="ts">
// Stand-in for GlimStone's Window: notch, body and a foot whose buttons close the window unless their run says false.
import { onMounted, ref } from 'vue';
import { G } from './glyphs';
import { isMenuOpen } from './menu';
import type { FootButton, WinEntry } from './windows';

const props = defineProps<{ w: WinEntry }>();
const scrim = ref<HTMLElement | null>(null);

function press(f: FootButton) {
  if (!f.run || f.run(scrim.value!) !== false) props.w.close();
}
// A click beside an open dropdown only closes the dropdown, not the window under it.
function beside(e: MouseEvent) {
  if (e.target === scrim.value && !isMenuOpen()) props.w.close();
}

onMounted(() => {
  const el = scrim.value!;
  props.w.el = el;
  props.w.onOpen?.(el, props.w.close);
  setTimeout(() => (el.querySelector<HTMLElement>('input:not([type=file]), textarea') || el.querySelector<HTMLElement>('.win-foot .btn:last-child'))?.focus(), 0);
});
</script>

<template>
  <div ref="scrim" class="scrim glim-modal-backdrop" @mousedown="beside">
    <div class="win glim-card" :class="[{ wide: w.wide, full: w.full }, w.cls]" role="dialog" :aria-label="w.title">
      <span class="notch">{{ w.title }}</span>
      <div v-if="typeof w.body === 'string'" class="win-body" v-html="w.body"></div>
      <div v-else class="win-body"><component :is="w.body" v-bind="w.props" :close="w.close" :win="w" /></div>
      <div class="win-foot">
        <template v-for="(f, k) in w.foot" :key="k">
          <span v-if="f.start" class="start">{{ f.start }}</span>
          <button v-else class="btn" :class="{ primary: f.primary }" :disabled="f.disabled" :data-f="k" @click="press(f)"><G v-if="f.g" :n="f.g" />{{ f.label }}</button>
        </template>
      </div>
    </div>
  </div>
</template>
