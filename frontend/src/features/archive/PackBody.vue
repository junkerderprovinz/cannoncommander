<script setup lang="ts">
// Verpacken: name, format, compression and password of the archive, and the folder it lands in.
import { PasswordField } from '@glimstone/vue';
import { computed, watch, watchEffect } from 'vue';
import { fmtSize } from '@/core/format';
import type { Entry } from '@/files/entries';
import { join } from '@/files/paths';
import { t } from '@/i18n';
import PathField from '@/ui/PathField.vue';
import FinalPath from '@/ui/FinalPath.vue';
import Frow from '@/ui/Frow.vue';
import { eyeWords, plural } from '@/ui/kit';
import Seg from '@/ui/Seg.vue';
import { clean, folderState, freeName, namesIn } from '@/ui/tools';

export interface PackState { items: Entry[]; name: string; format: string; level: string; password: string; dest: string; formats: string[]; file: string }
const props = defineProps<{ st: PackState }>();

const total = computed(() => props.st.items.reduce((a, k) => a + (k.size || 0), 0));
const formats = computed(() => props.st.formats.map((f) => [f, f] as const));
const levels = computed(() => ['0', '3', '6', '9'].map((l) => [l, t(`archive.levels.${l}`)] as const));
const tar = computed(() => props.st.format.startsWith('tar'));
watch(tar, (no) => { if (no) props.st.password = ''; });

const ok = folderState(() => props.st.dest);
const wanted = computed(() => `${props.st.name.trim()}.${props.st.format}`);
watchEffect(() => { props.st.file = ok.value && props.st.name.trim() ? freeName(namesIn(clean(props.st.dest)), wanted.value) : ''; });
</script>

<template>
  <div class="well glim-well bk-form"><div class="eyebrow">{{ plural(st.items.length, 'ui.objects') }}, {{ fmtSize(total) || '0 B' }}</div>
    <Frow :label="t('archive.name')"><span class="bk-namef"><input id="k-name" v-model="st.name" class="glim-input grow" spellcheck="false" autocomplete="off"><span id="k-ext" class="pill mono">.{{ st.format }}</span></span></Frow>
    <Frow :label="t('archive.format')" :tip="t('archive.formatTip')"><Seg id="k-fmt" v-model="st.format" :options="formats" /></Frow>
    <Frow :label="t('archive.level')" :tip="t('archive.levelTip')"><Seg id="k-level" v-model="st.level" :options="levels" /></Frow>
    <Frow :label="t('archive.password')" :tip="t('archive.passwordTip')"><PasswordField id="k-pw" v-model="st.password" class="grow" :placeholder="t('archive.noPassword')" :disabled="tar" autocomplete="new-password" :text="eyeWords()" /></Frow>
  </div>
  <div class="well glim-well bk-form"><div class="eyebrow">{{ t('archive.target') }}</div>
    <Frow :label="t('archive.folder')"><PathField id="k-dest" v-model="st.dest" /></Frow>
    <Frow :label="t('archive.savedAs')">
      <span id="k-final" class="bk-finalw">
        <span v-if="ok === false" class="pill fault">{{ t('ui.noFolder') }}</span>
        <span v-else-if="!st.name.trim()" class="pill fault">{{ t('ui.noName') }}</span>
        <template v-else-if="st.file"><FinalPath :path="join(clean(st.dest), st.file)" /><span v-if="st.file !== wanted" class="pill warn" :data-tip="t('archive.takenTip')">{{ t('ui.taken') }}</span></template>
      </span>
    </Frow>
  </div>
</template>
