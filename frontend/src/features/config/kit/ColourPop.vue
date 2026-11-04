<script setup lang="ts">
// The colour picker beside a swatch: a saturation and value square, a hue bar and the hex. It paints live while it is
// dragged and saves when the pointer lets go. GlimStone's ColorPicker draws other handles and a larger hex field, so
// the markup stays the prototype's while the colour maths is GlimStone's.
import { hexToHsv, hsvToHex } from '@glimstone/core';
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue';
import Frow from './Frow.vue';
import { keysFirst } from './kit';

const props = defineProps<{ anchor: HTMLElement; hex: string }>();
const emit = defineEmits<{ live: [hex: string]; done: [hex: string]; close: [] }>();

const start = hexToHsv(props.hex) ?? { h: 0, s: 0, v: 0 };
const h = ref(start.h), s = ref(start.s), v = ref(start.v);
const cur = computed(() => hsvToHex(h.value, s.value, v.value).toUpperCase());
const hexText = ref(cur.value);
const pop = ref<HTMLElement | null>(null);
const at = ref({ left: 0, top: 0 });

function drag(e: PointerEvent, apply: (x: number, y: number) => void) {
  const el = e.currentTarget as HTMLElement;
  el.setPointerCapture(e.pointerId);
  const move = (ev: PointerEvent) => {
    const b = el.getBoundingClientRect();
    apply(Math.max(0, Math.min(1, (ev.clientX - b.left) / b.width)), Math.max(0, Math.min(1, (ev.clientY - b.top) / b.height)));
    hexText.value = cur.value;
    emit('live', cur.value);
  };
  move(e);
  el.onpointermove = move;
  el.onpointerup = () => { el.onpointermove = null; emit('done', cur.value); };
}
const onSv = (e: PointerEvent) => drag(e, (x, y) => { s.value = x; v.value = 1 - y; });
const onHue = (e: PointerEvent) => drag(e, (x) => { h.value = Math.min(359.9, x * 360); });

function typed() {
  if (!/^#[0-9a-f]{6}$/i.test(hexText.value)) return;
  const got = hexToHsv(hexText.value)!;
  h.value = got.h;
  s.value = got.s;
  v.value = got.v;
  emit('live', hexText.value.toUpperCase());
  emit('done', hexText.value.toUpperCase());
}

const away = (e: Event) => {
  if (e.type === 'mousedown' && (pop.value?.contains(e.target as Node) || props.anchor.contains(e.target as Node))) return;
  emit('close');
};
let stopKeys = () => {};
onMounted(async () => {
  emit('live', cur.value);
  await nextTick();
  const r = props.anchor.getBoundingClientRect(), p = pop.value!;
  at.value = {
    left: Math.min(r.left, innerWidth - p.offsetWidth - 12),
    top: r.bottom + p.offsetHeight + 16 > innerHeight ? r.top - p.offsetHeight - 8 : r.bottom + 8,
  };
  setTimeout(() => {
    addEventListener('mousedown', away, true);
    addEventListener('resize', away);
    // Escape closes the picker only: the page under it stays open.
    stopKeys = keysFirst((e) => { if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); emit('close'); } });
  }, 0);
});
onBeforeUnmount(() => {
  removeEventListener('mousedown', away, true);
  removeEventListener('resize', away);
  stopKeys();
});
</script>

<template>
  <Teleport to="body">
    <div ref="pop" class="a-cpop glim-card" :style="{ left: at.left + 'px', top: at.top + 'px' }">
      <div class="a-sv" :style="{ background: `linear-gradient(to top, #000, transparent), linear-gradient(to right, #fff, hsl(${h} 100% 50%))` }" @pointerdown="onSv"><i :style="{ left: s * 100 + '%', top: (1 - v) * 100 + '%', background: cur }"></i></div>
      <div class="a-hue" @pointerdown="onHue"><i :style="{ left: h / 3.6 + '%' }"></i></div>
      <Frow label="Hex"><input v-model="hexText" class="field s mono" maxlength="7" spellcheck="false" @input="typed"></Frow>
    </div>
  </Teleport>
</template>
