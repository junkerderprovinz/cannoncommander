<script setup lang="ts">
// The recovery codes, shown once. The window stays until they are confirmed as kept, so a stray Esc cannot lose them.
import { onMounted } from 'vue';
import type { WinEntry } from '@/core/windows';
import Btn from '../kit/Btn.vue';
import { c, copy, setFoot } from '../kit/kit';

const props = defineProps<{ win: WinEntry; codes: string[]; done: () => void }>();

onMounted(() => {
  props.win.locked = true;
  setFoot(props.win, [{ label: c('security.kept'), g: 'IconCheck', primary: true, run: () => { props.win.locked = false; props.done(); } }]);
});
</script>

<template>
  <div class="well a-text"><p>{{ c('security.codesOnce') }}</p><p>{{ c('security.codesUse') }}</p></div>
  <div class="well"><div class="a-codes mono glim-num"><span v-for="code in codes" :key="code">{{ code }}</span></div><div class="a-foot"><Btn :label="c('security.copyAll')" g="IconCopy" @click="copy(codes.join('\n'))" /></div></div>
</template>
