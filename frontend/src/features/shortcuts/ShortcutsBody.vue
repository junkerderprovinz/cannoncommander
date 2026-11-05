<script setup lang="ts">
import { computed } from 'vue';
import { actions, labelOf } from '@/core/actions';
import { Kbd } from '@glimstone/vue';
import { t } from '@/i18n';

const FIXED = ['tab', 'enter', 'insert', 'space', 'ctrlClick', 'shiftClick', 'rightClick', 'f10', 'esc'];
const rows = computed(() => [
  ...FIXED.map((k) => [t(`shortcuts.fixed.${k}.label`), t(`shortcuts.fixed.${k}.key`)]),
  ...Object.values(actions).filter((a) => a.key).sort((a, b) => (a.menu || '').localeCompare(b.menu || '')).map((a) => [labelOf(a), a.key!]),
]);
</script>

<template>
  <div class="well glim-well"><div class="kbd-grid"><div v-for="([label, key], n) in rows" :key="n"><span>{{ label }}</span><Kbd class="kbd">{{ key }}</Kbd></div></div></div>
</template>
