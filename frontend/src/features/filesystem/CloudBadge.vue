<script setup lang="ts">
// The state of a file a cloud provider such as OneDrive keeps, where Explorer shows it in its status column.
import { G } from '@/core/glyphs';
import type { Entry } from '@/files/entries';
import { t } from '@/i18n';

defineProps<{ k: Entry }>();
const GLYPH: Record<string, string> = { online: 'IconCloud', local: 'IconCheck', pinned: 'IconPin' };
</script>

<template>
  <span v-if="k.cloud && GLYPH[k.cloud]" class="fs-cloud" :class="k.cloud" :data-tip="t(`filesystem.cloud.${k.cloud}`)" :aria-label="t(`filesystem.cloud.${k.cloud}`)"><G :n="GLYPH[k.cloud]!" /></span>
</template>

<style scoped>
.fs-cloud { display: inline-flex; flex: none; margin-inline-start: .35rem; color: var(--carbon-text-muted); }
.fs-cloud > svg { width: .95rem; height: .95rem; }
.fs-cloud.pinned, .fs-cloud.local { color: var(--status-ok-text); }
</style>
