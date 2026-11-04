<script setup lang="ts">
// A Word document as formatted text on a sheet of paper. Without an office server there are no real pages, so the
// paper simply grows with the text.
import DOMPurify from 'dompurify';
import { onMounted, ref } from 'vue';
import { t } from '@/i18n';
import { rawUrl } from '../content';
import { joinPath } from '../kinds';
import type { REmits, RProps } from './types';

const props = defineProps<RProps>();
const emit = defineEmits<REmits & { (e: 'failed'): void }>();
const html = ref<string | null>(null);

onMounted(async () => {
  try {
    const res = await fetch(rawUrl(joinPath(props.folder, props.n.name)));
    if (!res.ok) throw new Error(String(res.status));
    const { convertToHtml } = await import('mammoth');
    const out = await convertToHtml({ arrayBuffer: await res.arrayBuffer() }, { styleMap: ["p[style-name='Title'] => h1:fresh", "p[style-name='Subtitle'] => p.lead:fresh"] });
    html.value = DOMPurify.sanitize(out.value);
    emit('info', t('viewer.office.doc'));
  } catch {
    emit('failed');
  }
});
</script>

<template>
  <div v-if="compact" class="cv-paper"><div class="pg" v-html="html ?? ''"></div></div>
  <div v-else class="cv-pages"><div class="cv-paper flow"><div class="pg" v-html="html ?? ''"></div></div></div>
</template>
