<script setup lang="ts">
// Everything without a better view gets the start of its bytes, as a lister would show it.
import { EmptyState, InfoBubble, Pill } from '@glimstone/vue';
import { computed, onMounted, ref } from 'vue';
import { plural, t } from '@/i18n';
import { fsHex } from '../content';
import { joinPath } from '../kinds';
import { hexRows, kindOfBytes } from '../hex';
import type { REmits, RProps } from './types';

const props = defineProps<RProps>();
const emit = defineEmits<REmits>();
const bytes = ref<Uint8Array | null>(null);
const failed = ref(false);
const rows = computed(() => (bytes.value ? hexRows(bytes.value) : []));

onMounted(async () => {
  try {
    bytes.value = (await fsHex(joinPath(props.folder, props.n.name), 0, props.compact ? 192 : 512)).bytes;
    emit('info', t(`viewer.hex.${kindOfBytes(bytes.value)}`));
  } catch {
    failed.value = true;
  }
});
</script>

<template>
  <div v-if="!compact" class="cv-bar">
    <Pill>{{ plural(bytes?.length ?? 0, 'viewer.hex.first') }}</Pill>
    <InfoBubble :tip="t('viewer.hex.tip')" />
  </div>
  <EmptyState v-if="failed" nested :title="t('viewer.failed')" />
  <div v-else class="cv-scroll cv-hex mono">
    <div v-for="r in rows" :key="r.o" class="hx"><span class="o">{{ r.o }}</span><span class="h">{{ r.h }}</span><span class="a">{{ r.a }}</span></div>
  </div>
</template>
