<script setup lang="ts">
// The name of a saved search, all of it selected, so typing replaces the suggestion.
import { FieldRow, TextField } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { t } from '@/i18n';

defineProps<{ st: { name: string } }>();
const input = ref<InstanceType<typeof TextField> | null>(null);
onMounted(() => setTimeout(() => input.value?.input?.select(), 0));
const enter = (e: KeyboardEvent) => (e.target as Element).closest('.win')?.querySelector<HTMLElement>('.glim-modal-foot .glim-btn--accent')?.click();
</script>

<template>
  <div class="well glim-well"><FieldRow :label="t('search.nameLabel')" :hint="t('search.nameWinTip')"><TextField id="sm-name" ref="input" v-model="st.name" autocomplete="off" width="wide" @keydown.enter="enter" /></FieldRow></div>
</template>
