<script setup lang="ts">
import { onMounted } from 'vue';
import { run } from '@/core/actions';
import { closeBtn, type WinEntry } from '@/core/windows';
import { c, setFoot } from '../kit/kit';

const props = defineProps<{ win: WinEntry }>();
const PARTS = ['panels', 'mark', 'find', 'share', 'rights'];
onMounted(() => setFoot(props.win, [{ label: c('docs.keys'), g: 'IconKeyboard', run: () => { setTimeout(() => run('shortcuts'), 0); } }, closeBtn()]));
</script>

<template>
  <div v-for="p in PARTS" :key="p" class="well"><div class="eyebrow">{{ c(`docs.${p}.title`) }}</div><div class="a-text"><p>{{ c(`docs.${p}.text`) }}</p></div></div>
</template>
