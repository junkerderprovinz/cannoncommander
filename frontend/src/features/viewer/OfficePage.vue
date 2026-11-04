<script setup lang="ts">
// A document open for editing in the office suite, in a tab of its own.
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
  <section class="cv-officepage glim-card">
    <span class="notch">{{ name }}</span>
    <div v-if="failed" class="empty">
      <G n="IconWarning" /><div>{{ t('viewer.office.unreachable', { name: officeName() }) }}</div>
      <button class="btn" @click="download({ name, size: 0, time: 0 }, path.slice(0, -name.length - 1) || '/')"><G n="IconDownload" />{{ t('viewer.download') }}</button>
    </div>
    <OfficeFrame v-else :path="path" edit @failed="failed = true" />
  </section>
</template>
