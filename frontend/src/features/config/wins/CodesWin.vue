<script setup lang="ts">
// The recovery codes, shown once. The window stays until they are confirmed as kept, so a stray Esc cannot lose them.
import { Button } from '@glimstone/vue';
import { onMounted } from 'vue';
import type { WinEntry } from '@/core/windows';
import { c, stepFoot } from '@/features/config/shared';
import { copyText } from '@/ui/kit';

const props = defineProps<{ win: WinEntry; codes: string[]; done: () => void }>();

onMounted(() => {
  props.win.locked = true;
  stepFoot(props.win, [{ label: c('security.kept'), g: 'IconCheck', primary: true, wayOut: true, run: () => { props.win.locked = false; props.done(); } }]);
});
</script>

<template>
  <div class="well glim-well a-text"><p>{{ c('security.codesOnce') }}</p><p>{{ c('security.codesUse') }}</p></div>
  <div class="well glim-well"><div class="a-codes mono glim-num"><span v-for="code in codes" :key="code">{{ code }}</span></div><div class="a-foot"><Button :label="c('security.copyAll')" label-key="IconCopy" @click="copyText(codes.join('\n'))" /></div></div>
</template>
