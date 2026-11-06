<script setup lang="ts">
import { computed } from 'vue';
import { actions, labelOf } from '@/core/actions';
import { Kbd, SubCard } from '@glimstone/vue';
import { byText, t } from '@/i18n';

const FIXED = ['tab', 'enter', 'insert', 'space', 'ctrlClick', 'shiftClick', 'rightClick', 'f10', 'esc'];
const rows = computed(() => [
  ...FIXED.map((k) => [t(`shortcuts.fixed.${k}.label`), t(`shortcuts.fixed.${k}.key`)]),
  ...Object.values(actions).filter((a) => a.key).sort((a, b) => byText(a.menu || '', b.menu || '')).map((a) => [labelOf(a), a.key!]),
]);
</script>

<template>
  <SubCard><div class="kbd-grid"><div v-for="([label, key], n) in rows" :key="n"><span>{{ label }}</span><Kbd>{{ key }}</Kbd></div></div></SubCard>
</template>
