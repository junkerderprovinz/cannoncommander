<script setup lang="ts">
// Nach Muster markieren: names in the open folder that match one of a few patterns, counted as they are typed.
import { computed, onMounted, ref, watchEffect } from 'vue';
import { t } from '@/i18n';
import { usePanels } from '@/stores/panels';
import Frow from '@/ui/Frow.vue';
import { pressPrimary } from '@/ui/kit';
import Seg from '@/ui/Seg.vue';
import Sw from '@/ui/Sw.vue';

export interface PatternState { pattern: string; folders: boolean; mode: 'add' | 'replace'; hits: string[]; total: number }
const props = defineProps<{ st: PatternState }>();
const PRESETS = ['*.jpg *.png', '*.mkv *.mp4', '*.iso *.img', '*.zip *.7z *.zst', '*.pdf *.docx'];
const modes = computed(() => [['add', t('rename.add')], ['replace', t('rename.replaceMarks')]] as const);

// A pattern without * or ? matches anywhere in the name, as people type it into a search field.
function globRe(pattern: string) {
  const parts = pattern.trim().split(/[\s;]+/).filter(Boolean);
  if (!parts.length) return null;
  const one = (p: string) => {
    const body = p.replace(/[.+^${}()|[\]\\]/g, '\\$&').replace(/\*/g, '.*').replace(/\?/g, '.');
    return /[*?]/.test(p) ? `^${body}$` : body;
  };
  return new RegExp(parts.map(one).join('|'), 'i');
}

const F = usePanels();
watchEffect(() => {
  const list = F.entries().filter((k) => !k.up), re = globRe(props.st.pattern);
  props.st.total = list.length;
  props.st.hits = re ? list.filter((k) => (props.st.folders || !k.dir) && re.test(k.name)).map((k) => k.name) : [];
});

const input = ref<HTMLInputElement | null>(null);
onMounted(() => setTimeout(() => input.value?.select(), 20));
</script>

<template>
  <div class="well bk-form">
    <Frow :label="t('rename.mask')" :tip="t('rename.patternTip')"><input id="p-pat" ref="input" v-model="st.pattern" class="field grow mono" spellcheck="false" autocomplete="off" @keydown.enter="pressPrimary"></Frow>
    <div class="bk-tokens"><button v-for="p in PRESETS" :key="p" class="bk-chip mono" type="button" :data-bk-pre="p" @click="st.pattern = p">{{ p }}</button></div>
    <Frow :label="t('rename.withFolders')"><Sw id="p-dirs" v-model="st.folders" /></Frow>
    <Frow :label="t('rename.marking')" :tip="t('rename.markingTip')"><Seg id="p-mode" v-model="st.mode" :options="modes" /></Frow>
  </div>
</template>
