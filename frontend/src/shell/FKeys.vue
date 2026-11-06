<script setup lang="ts">
// The F-key bar follows the key map, so keys of your own show up here too.
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { actions, labelOf, run } from '@/core/actions';
import { Button, Kbd } from '@glimstone/vue';
import { FKEYS, keymap } from '@/core/keymap';
import { t } from '@/i18n';

const keys = computed(() => FKEYS.map((k) => {
  const id = k === 'F10' ? 'menuBar' : keymap[k];
  const a = id ? actions[id] : undefined;
  return a && { k, id: id!, g: a.g!, label: keymap[k] === 'shortcuts' ? t('fkeys.help') : labelOf(a) };
}).filter((x) => !!x));

// Where the labels of a language do not fit, the bar drops its glyphs before it cuts a word.
const bar = ref<HTMLElement>();
const tight = ref(false);
async function fit() {
  tight.value = false;
  await nextTick();
  tight.value = [...(bar.value?.querySelectorAll<HTMLElement>('.glim-btn-label') ?? [])].some((l) => l.scrollWidth > l.clientWidth + 1 || l.scrollHeight > l.clientHeight + 1);
}
const seen = new ResizeObserver(() => { fit(); });
onMounted(() => { if (bar.value) seen.observe(bar.value); });
onBeforeUnmount(() => seen.disconnect());
watch(() => keys.value.map((f) => f.label).join(), fit);
</script>

<template>
  <footer id="fkeys" ref="bar" class="fkeys glim-card" :class="{ tight }">
    <Button v-for="f in keys" :key="f.k" :label="f.label" :label-key="f.g" :data-run="f.id" @click="run(f.id)"><template #before><Kbd>{{ f.k }}</Kbd></template></Button>
  </footer>
</template>
