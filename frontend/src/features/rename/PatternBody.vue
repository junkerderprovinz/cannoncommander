<script setup lang="ts">
// Nach Muster markieren: names in the open folder that match one of a few patterns, counted as they are typed.
import { FieldRow, Selector, TextField, Toggle } from '@glimstone/vue';
import { computed, onMounted, ref, watchEffect } from 'vue';
import { t } from '@/i18n';
import { usePanels } from '@/stores/panels';
import { pressPrimary, segItems } from '@/ui/kit';

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

const input = ref<InstanceType<typeof TextField> | null>(null);
onMounted(() => setTimeout(() => input.value?.input?.select(), 20));
</script>

<template>
  <div class="well glim-well bk-form">
    <FieldRow :label="t('rename.mask')" :hint="t('rename.patternTip')"><TextField id="p-pat" ref="input" v-model="st.pattern" spellcheck="false" autocomplete="off" width="grow" mono @keydown.enter="pressPrimary" /></FieldRow>
    <div class="bk-tokens"><button v-for="p in PRESETS" :key="p" class="bk-chip mono" type="button" :data-bk-pre="p" @click="st.pattern = p">{{ p }}</button></div>
    <FieldRow :label="t('rename.withFolders')"><Toggle id="p-dirs" v-model="st.folders" /></FieldRow>
    <FieldRow :label="t('rename.marking')" :hint="t('rename.markingTip')"><Selector id="p-mode" v-model="st.mode" :items="segItems(modes)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
  </div>
</template>
