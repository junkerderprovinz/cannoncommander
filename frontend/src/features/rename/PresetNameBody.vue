<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { t } from '@/i18n';
import Frow from '@/ui/Frow.vue';
import { pressPrimary } from '@/ui/kit';

defineProps<{ ps: { name: string } }>();
const input = ref<HTMLInputElement | null>(null);
onMounted(() => setTimeout(() => input.value?.select(), 20));
</script>

<template>
  <div class="well"><Frow :label="t('rename.name')" :tip="t('rename.saveTip')"><input id="m-pname" ref="input" v-model="ps.name" class="field w" autocomplete="off" @keydown.enter="pressPrimary"></Frow></div>
</template>
