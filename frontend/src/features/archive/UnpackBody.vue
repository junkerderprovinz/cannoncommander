<script setup lang="ts">
// Entpacken: the archives, or what is picked inside one, and where it all lands.
import { computed, watchEffect } from 'vue';
import { fmtSize } from '@/core/format';
import type { Entry } from '@/files/entries';
import { join } from '@/files/paths';
import { t } from '@/i18n';
import PwField from '@/features/config/kit/PwField.vue';
import BkPath from '@/ui/BkPath.vue';
import FinalPath from '@/ui/FinalPath.vue';
import Frow from '@/ui/Frow.vue';
import { plural } from '@/ui/kit';
import Seg from '@/ui/Seg.vue';
import Sw from '@/ui/Sw.vue';
import { clean, folderState } from '@/ui/tools';
import { stripArchive } from './names';

export interface UnpackState {
  inside: boolean; items: Entry[]; archive: string; dest: string; own: boolean; clash: 'skip' | 'both' | 'replace';
  password: string; locked: boolean; ok: boolean; size: number | null;
}
const props = defineProps<{ st: UnpackState }>();
const clashes = computed(() => [['skip', t('archive.skip'), 'IconKeepRight'], ['both', t('archive.both'), 'IconKeepBoth'], ['replace', t('archive.replace'), 'IconKeepLeft']] as const);
const ok = folderState(() => props.st.dest);
watchEffect(() => { props.st.ok = !!ok.value; });
const head = computed(() => (props.st.inside ? t('archive.fromArchive', { objects: plural(props.st.items.length, 'ui.objects'), name: props.st.archive }) : plural(props.st.items.length, 'archive.archives')));
</script>

<template>
  <div class="well"><div class="eyebrow">{{ head }}</div>
    <table class="tbl">
      <tr v-for="k in st.items.slice(0, 5)" :key="k.name"><td>{{ k.name }}</td><td class="glim-num r">{{ fmtSize(k.size) }}</td></tr>
      <tr v-if="st.items.length > 5"><td colspan="2" class="muted">{{ t('archive.more', { n: st.items.length - 5 }) }}</td></tr>
    </table>
  </div>
  <div class="well bk-form"><div class="eyebrow">{{ t('archive.target') }}</div>
    <Frow :label="t('archive.folder')"><BkPath id="x-dest" v-model="st.dest" /></Frow>
    <Frow :label="t('archive.own')" :tip="t('archive.ownTip')"><Sw id="x-own" v-model="st.own" /></Frow>
    <Frow :label="t('archive.clash')" :tip="t('archive.clashTip')"><Seg id="x-clash" v-model="st.clash" :options="clashes" /></Frow>
    <Frow v-if="st.locked" :label="t('archive.unpackPassword')" :tip="t('archive.unpackPasswordTip')"><PwField id="x-pw" v-model="st.password" size="m" /></Frow>
    <Frow :label="t('archive.lands')">
      <span id="x-final" class="bk-finalw">
        <span v-if="ok === false" class="pill fault">{{ t('ui.noFolder') }}</span>
        <template v-else-if="st.own"><FinalPath :path="join(clean(st.dest), stripArchive(st.inside ? st.archive : st.items[0]!.name))" /><span v-if="!st.inside && st.items.length > 1" class="pill">{{ t('archive.more', { n: st.items.length - 1 }) }}</span></template>
        <FinalPath v-else :path="clean(st.dest)" />
      </span>
    </Frow>
  </div>
</template>
