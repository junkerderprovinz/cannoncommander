<script setup lang="ts">
// A colour picker in the house style: a saturation and brightness field, a hue strip, the hex value and the name it
// proposes. The maths and the German colour names are GlimStone's, the handles and the hex field stay the prototype's.
import { COLOR_NAMES_DE, hexToHsv, hsvToHex, nearestColorName } from '@glimstone/core';
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';

const props = defineProps<{ anchor: HTMLElement; hex: string }>();
const emit = defineEmits<{ done: [hex: string, name: string]; close: [] }>();

const start = hexToHsv(props.hex) ?? { h: 0, s: 0, v: 0.5 };
const h = ref(start.h), s = ref(start.s), v = ref(start.v);
const cur = computed(() => hsvToHex(h.value, s.value, v.value));
const name = computed(() => nearestColorName(cur.value, COLOR_NAMES_DE) ?? '');
const hexText = ref(cur.value);
const pop = ref<HTMLElement | null>(null), sv = ref<HTMLElement | null>(null), hue = ref<HTMLElement | null>(null);
const at = ref({ left: 0, top: 0 });

// Pointer capture keeps the drag going when the pointer leaves the field, with mouse, pen and finger alike.
function drag(e: PointerEvent, apply: (x: number, y: number) => void) {
  e.preventDefault();
  const el = e.currentTarget as HTMLElement;
  el.focus();
  el.setPointerCapture(e.pointerId);
  const move = (ev: PointerEvent) => {
    const b = el.getBoundingClientRect();
    apply(Math.max(0, Math.min(1, (ev.clientX - b.left) / b.width)), Math.max(0, Math.min(1, (ev.clientY - b.top) / b.height)));
    hexText.value = cur.value;
  };
  move(e);
  el.onpointermove = (ev) => { if (el.hasPointerCapture(ev.pointerId)) move(ev); };
}
const onSv = (e: PointerEvent) => drag(e, (x, y) => { s.value = x; v.value = 1 - y; });
const onHue = (e: PointerEvent) => drag(e, (x) => { h.value = Math.min(359, x * 360); });

function typed() {
  const got = /^#[0-9a-f]{6}$/i.test(hexText.value.trim()) && hexToHsv(hexText.value.trim().toLowerCase());
  if (!got) return;
  h.value = got.h;
  s.value = got.s;
  v.value = got.v;
}
function ok() {
  emit('done', cur.value, name.value);
  emit('close');
}

function key(e: KeyboardEvent) {
  if (e.key === 'Escape') { e.stopPropagation(); e.preventDefault(); emit('close'); return; }
  if (!pop.value?.contains(e.target as Node)) return;
  if (e.key === 'Enter') { e.preventDefault(); e.stopPropagation(); ok(); return; }
  const step = e.shiftKey ? 0.1 : 0.02;
  const dx = ({ ArrowLeft: -1, ArrowRight: 1 } as Record<string, number>)[e.key] || 0, dy = ({ ArrowUp: 1, ArrowDown: -1 } as Record<string, number>)[e.key] || 0;
  if (!dx && !dy) return;
  if (e.target === sv.value) { s.value = Math.max(0, Math.min(1, s.value + dx * step)); v.value = Math.max(0, Math.min(1, v.value + dy * step)); }
  else if (e.target === hue.value) h.value = (h.value + (dx || dy) * step * 360 + 360) % 360;
  else return;
  e.preventDefault();
  hexText.value = cur.value;
}
const outside = (e: MouseEvent) => { if (!pop.value?.contains(e.target as Node) && e.target !== props.anchor) emit('close'); };

onMounted(async () => {
  await nextTick();
  const r = props.anchor.getBoundingClientRect(), box = pop.value!.getBoundingClientRect();
  at.value = {
    left: Math.max(8, Math.min(r.left, innerWidth - box.width - 8)),
    top: r.bottom + box.height + 8 > innerHeight ? Math.max(8, r.top - box.height - 6) : r.bottom + 6,
  };
  // On the window, so the picker's Escape is gone before the window under it hears the key.
  addEventListener('keydown', key, true);
  document.addEventListener('mousedown', outside, true);
  sv.value?.focus();
});
onBeforeUnmount(() => {
  removeEventListener('keydown', key, true);
  document.removeEventListener('mousedown', outside, true);
});
</script>

<template>
  <Teleport to="body">
    <div ref="pop" class="cpick glim-card" :style="{ '--h': h, '--c': cur, left: at.left + 'px', top: at.top + 'px' }">
      <div ref="sv" class="cp-sv" tabindex="0" role="slider" :aria-label="t('labels.picker.sv')" :aria-valuetext="t('labels.picker.svValue', { s: Math.round(s * 100), v: Math.round(v * 100) })" @pointerdown="onSv"><i class="cp-knob" :style="{ left: s * 100 + '%', top: (1 - v) * 100 + '%' }"></i></div>
      <div ref="hue" class="cp-hue" tabindex="0" role="slider" :aria-label="t('labels.picker.hue')" aria-valuemin="0" aria-valuemax="359" :aria-valuenow="Math.round(h)" @pointerdown="onHue"><i class="cp-knob" :style="{ left: h / 3.6 + '%' }"></i></div>
      <div class="cp-foot"><i class="cp-now"></i><span class="cp-name">{{ name }}</span><input v-model="hexText" class="field cp-hex mono" maxlength="7" :aria-label="t('labels.picker.value')" spellcheck="false" @input="typed"></div>
      <div class="cp-act"><button class="btn primary cp-ok" @click="ok"><G n="IconCheck" />{{ t('labels.take') }}</button></div>
    </div>
  </Teleport>
</template>
