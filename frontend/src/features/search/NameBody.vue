<script setup lang="ts">
// The name of a saved search, all of it selected, so typing replaces the suggestion.
import { onMounted, ref } from 'vue';
import { t } from '@/i18n';
import Frow from '@/ui/Frow.vue';

defineProps<{ st: { name: string } }>();
const input = ref<HTMLInputElement | null>(null);
onMounted(() => setTimeout(() => input.value?.select(), 0));
const enter = (e: KeyboardEvent) => (e.target as Element).closest('.win')?.querySelector<HTMLElement>('.win-foot .btn.primary')?.click();
</script>

<template>
  <div class="well"><Frow :label="t('search.nameLabel')" :tip="t('search.nameWinTip')"><input id="sm-name" ref="input" v-model="st.name" class="field w" autocomplete="off" @keydown.enter="enter"></Frow></div>
</template>
