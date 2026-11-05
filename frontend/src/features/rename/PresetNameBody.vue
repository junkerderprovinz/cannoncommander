<script setup lang="ts">
import { FieldRow, TextField } from '@glimstone/vue';
import { onMounted, ref } from 'vue';
import { t } from '@/i18n';
import { pressPrimary } from '@/ui/kit';

defineProps<{ ps: { name: string } }>();
const input = ref<InstanceType<typeof TextField> | null>(null);
onMounted(() => setTimeout(() => input.value?.input?.select(), 20));
</script>

<template>
  <div class="well glim-well"><FieldRow :label="t('rename.name')" :hint="t('rename.saveTip')"><TextField id="m-pname" ref="input" v-model="ps.name" autocomplete="off" width="wide" @keydown.enter="pressPrimary" /></FieldRow></div>
</template>
