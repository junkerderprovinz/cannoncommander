<script setup lang="ts">
// What came from the clipboard, and the name it gets. The part before the extension starts selected.
import { FieldRow, SubCard, TextField } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { t } from '@/i18n';

const props = defineProps<{ st: { name: string }; url?: string; text?: string; folder: string }>();
const input = ref<InstanceType<typeof TextField> | null>(null);
onMounted(() => setTimeout(() => {
  const dot = props.st.name.lastIndexOf('.');
  input.value?.input?.setSelectionRange(0, dot > 0 ? dot : props.st.name.length);
}, 0));
const enter = (e: KeyboardEvent) => (e.target as Element).closest('.win')?.querySelector<HTMLElement>('.glim-modal-foot .glim-btn--accent')?.click();
</script>

<template>
  <SubCard class="pz-prev"><img v-if="url" class="pz-img" :src="url" alt=""><pre v-else class="pz-text">{{ text?.slice(0, 600) }}</pre></SubCard>
  <SubCard><FieldRow :label="t('pastefile.name')" :hint="t('pastefile.where', { path: folder })"><TextField id="pz-name" ref="input" v-model="st.name" autocomplete="off" width="wide" @keydown.enter="enter" /></FieldRow></SubCard>
</template>
