<script setup lang="ts">
// A document open for editing in the office suite, in a tab of its own.
import { Button, Card } from '@glimstone/vue';
import { computed, ref } from 'vue';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
import { download, officeName } from './index';
import OfficeFrame from './renderers/OfficeFrame.vue';

const props = defineProps<{ arg?: unknown }>();
const path = computed(() => String(props.arg || ''));
const name = computed(() => path.value.split('/').pop() || '');
const failed = ref(false);
document.title = name.value || document.title;
</script>

<template>
  <Card class="cv-officepage glim-card" :title="name">
    <div v-if="failed" class="empty">
      <G n="IconWarning" /><div>{{ t('viewer.office.unreachable', { name: officeName() }) }}</div>
      <Button :label="t('viewer.download')" label-key="IconDownload" @click="download({ name, size: 0, time: 0 }, path.slice(0, -name.length - 1) || '/')" />
    </div>
    <OfficeFrame v-else :path="path" edit @failed="failed = true" />
  </Card>
</template>
