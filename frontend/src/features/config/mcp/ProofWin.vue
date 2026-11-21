<script setup lang="ts">
// A step that hands out more than reading: allowing a key to write, or replacing it. The server wants a fresh sign-in
// for it, so the password question may come first, and a replaced key shows once, until it is confirmed as copied.
import { CopyField, FieldRow, SubCard } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { ApiError } from '@/api/client';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { c, fail, stepFoot } from '@/features/config/shared';
import { withPassword } from '@/signin/kit';
import { copyWords } from '@/ui/kit';

const props = defineProps<{
  win: WinEntry; text: string[]; verb: string; glyph: string;
  // Resolves to a key to show once, or to nothing.
  run: () => Promise<string | void>;
  done: () => void;
}>();
const secret = ref('');

async function go() {
  let key: string | void;
  try {
    key = await withPassword(props.run);
  } catch (e) {
    return fail(e instanceof ApiError ? e.message : c('notSaved'));
  }
  if (!key) {
    props.done();
    props.win.close();
    return;
  }
  secret.value = key;
  props.win.locked = true;
  stepFoot(props.win, [{ label: c('done'), g: 'IconCheck', primary: true, run: () => { props.win.locked = false; props.done(); } }]);
}

onMounted(() => stepFoot(props.win, [cancelBtn(), { label: props.verb, g: props.glyph, primary: true, run: () => { go(); return false; } }]));
</script>

<template>
  <template v-if="secret">
    <SubCard class="a-text"><p>{{ c('mcp.copyNow') }}</p></SubCard>
    <SubCard><FieldRow :label="c('mcp.newKey')"><CopyField width="grow" :value="secret" :label="c('mcp.newKey')" :text="copyWords()" /></FieldRow></SubCard>
  </template>
  <SubCard v-else class="a-text"><p v-for="(p, k) in text" :key="k">{{ p }}</p></SubCard>
</template>
