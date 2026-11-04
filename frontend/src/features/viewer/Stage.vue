<script setup lang="ts">
// One file in its renderer. A renderer that cannot read its file hands over to the hex view, and a PDF the server
// cannot draw opens in the browser's own reader.
import { computed, ref, watch } from 'vue';
import { rawUrl } from './content';
import { type FileNode, joinPath, viewKind } from './kinds';
import { RENDERERS } from './renderers';
import type { KeyHandler } from './renderers/types';

const props = defineProps<{ n: FileNode; folder: string; sibs: FileNode[]; compact?: boolean; autoplay?: boolean }>();
const emit = defineEmits<{ (e: 'info', text: string): void; (e: 'go', to: FileNode, play?: boolean): void }>();

const failed = ref(false);
watch(() => props.n, () => { failed.value = false; });
const kind = computed(() => viewKind(props.n));
const inner = ref<{ keys?: KeyHandler } | null>(null);
const renderer = computed(() => (failed.value ? RENDERERS.file : RENDERERS[kind.value]));

defineExpose({ keys: (e: KeyboardEvent, combo: string) => inner.value?.keys?.(e, combo) });
</script>

<template>
  <iframe v-if="failed && kind === 'pdf'" class="cv-pdf" :src="rawUrl(joinPath(folder, n.name))" :title="n.name"></iframe>
  <component :is="renderer" v-else ref="inner" :key="n.name + failed" :n="n" :folder="folder" :sibs="sibs" :compact="compact" :autoplay="autoplay"
    @info="emit('info', $event)" @go="(to: FileNode, play?: boolean) => emit('go', to, play)" @failed="failed = true" />
</template>
