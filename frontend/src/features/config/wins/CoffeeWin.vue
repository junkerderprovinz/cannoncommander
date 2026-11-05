<script setup lang="ts">
import { onMounted } from 'vue';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { c, stepFoot } from '@/features/config/shared';

const props = defineProps<{ win: WinEntry }>();
onMounted(() => stepFoot(props.win, [cancelBtn(), { label: c('info.toCoffee'), g: 'IconCoffee', primary: true, run: () => { window.open('https://buymeacoffee.com/junkerderprovinz', '_blank', 'noopener'); } }]));
</script>

<template>
  <div class="well glim-well a-text"><p>{{ c('info.coffee1') }}</p><p>{{ c('info.coffee2') }}</p></div>
</template>
