<script setup lang="ts">
import { ToastViewport } from '@glimstone/vue';
import { computed } from 'vue';
import { t } from '@/i18n';
import { dropToast, holdToast, releaseToast, type ToastKind, toasts } from './toast';

const SEVERITY = { ok: 'success', warn: 'warn', fault: 'fail' } as const satisfies Record<ToastKind, string>;
const shown = computed(() => toasts.map((x) => ({
  id: String(x.id),
  message: x.text,
  severity: SEVERITY[x.kind],
  action: x.act && { label: x.act.label, onClick: x.act.run },
})));
const byId = (id: string) => toasts.find((x) => String(x.id) === id);
const hold = (id: string) => { const x = byId(id); if (x) holdToast(x); };
const release = (id: string) => { const x = byId(id); if (x) releaseToast(x); };
</script>

<template>
  <ToastViewport :toasts="shown" :dismiss-label="t('core.close')" @dismiss="dropToast(Number($event))" @enter="hold" @leave="release" @focusin="hold" @focusout="release" />
</template>
