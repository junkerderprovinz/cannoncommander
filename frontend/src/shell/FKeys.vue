<script setup lang="ts">
// The F-key bar follows the key map, so keys of your own show up here too.
import { computed } from 'vue';
import { actions, labelOf, run } from '@/core/actions';
import { G } from '@/core/glyphs';
import Kbd from '@/core/Kbd.vue';
import { FKEYS, keymap } from '@/core/keymap';
import { t } from '@/i18n';

const keys = computed(() => FKEYS.map((k) => {
  const id = k === 'F10' ? 'menuBar' : keymap[k];
  const a = id ? actions[id] : undefined;
  return a && { k, id: id!, g: a.g!, label: keymap[k] === 'shortcuts' ? t('fkeys.help') : labelOf(a) };
}).filter((x) => !!x));
</script>

<template>
  <footer id="fkeys" class="fkeys glim-card">
    <button v-for="f in keys" :key="f.k" class="btn" :data-run="f.id" @click="run(f.id)"><Kbd>{{ f.k }}</Kbd><G :n="f.g" />{{ f.label }}</button>
  </footer>
</template>
