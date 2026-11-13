<script setup lang="ts">
import { SubCard } from '@glimstone/vue';
import { onMounted } from 'vue';
import { run } from '@/core/actions';
import { closeBtn, type WinEntry } from '@/core/windows';
import { c, stepFoot } from '@/features/config/shared';
import { DOCS } from '@/features/config/support';

const props = defineProps<{ win: WinEntry }>();
const PARTS = ['panels', 'mark', 'find', 'share', 'rights', 'mcp'];
onMounted(() => stepFoot(props.win, [
  { label: c('docs.site'), g: 'IconExternalLink', run: () => { window.open(DOCS, '_blank', 'noopener'); return false; } },
  { label: c('docs.keys'), g: 'IconKeyboard', run: () => { setTimeout(() => run('shortcuts'), 0); } },
  closeBtn(),
]));
</script>

<template>
  <SubCard :title="c(`docs.${p}.title`)" v-for="p in PARTS" :key="p"><div class="a-text"><p>{{ c(`docs.${p}.text`) }}</p></div></SubCard>
</template>
