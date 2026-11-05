<script setup lang="ts">
// The rights repair before it runs: who will own the files, which rights they get, how many there are and which
// folders keep their own owners.
import { FieldRow, Pill, SubCard, Table } from '@glimstone/vue';
import { computed } from 'vue';
import type { PermsScan } from '@/api/unraid';
import { fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';

export interface PermsState { path: string; name: string; scan: PermsScan | null }
const props = defineProps<{ st: PermsState }>();

const blocked = computed(() => props.st.scan?.blocked);
const plural = (n: number, one: string, many: string) => t(n === 1 ? one : many, { n });
const affects = computed(() => {
  const s = props.st.scan;
  if (!s) return t('unraid.perms.counting');
  const what = t('unraid.perms.affectsValue', { files: plural(s.files, 'unraid.perms.fileOne', 'unraid.perms.fileMany'), dirs: plural(s.dirs + 1, 'unraid.perms.dirOne', 'unraid.perms.dirMany') });
  return s.partial ? t('unraid.perms.atLeast', { what }) : what;
});
const kept = computed(() => props.st.scan?.kept ?? []);
</script>

<template>
  <SubCard v-if="blocked" class="u-warnwell">
    <div class="u-alert"><G n="IconShieldOn" /><div><b>{{ t('unraid.perms.blockedTitle') }}</b>
      <p>{{ blocked === 'archive' ? t('unraid.perms.blockedArchive') : t('unraid.perms.blockedText', { path: st.path, own: t(`unraid.perms.reason.${blocked}`) }) }}</p></div></div>
  </SubCard>
  <template v-else>
    <SubCard :title="t('unraid.perms.folder')" class="bk-form">
      <FieldRow :label="t('unraid.perms.folder')"><span class="bk-val bk-pathv"><span class="bk-path mono" :title="st.path"><bdi>{{ st.path }}</bdi></span></span></FieldRow>
      <FieldRow :label="t('unraid.perms.owner')" :hint="t('unraid.perms.ownerTip')"><span class="bk-val mono">{{ t('unraid.perms.ownerValue') }}</span></FieldRow>
      <FieldRow :label="t('unraid.perms.rights')" :hint="t('unraid.perms.rightsTip')"><span class="bk-val mono">{{ t('unraid.perms.rightsValue') }}</span></FieldRow>
      <FieldRow :label="t('unraid.perms.affects')"><span class="bk-val glim-num">{{ affects }}</span></FieldRow>
    </SubCard>
    <SubCard class="u-warnwell">
      <div class="u-alert"><G n="IconWarning" /><div><b>{{ t('unraid.perms.warnTitle') }}</b>
        <p>{{ t('unraid.perms.warnText', { name: st.name }) }}{{ st.scan ? t(kept.length ? 'unraid.perms.keptSome' : 'unraid.perms.keptNone') : '' }}</p></div></div>
      <Table v-if="kept.length" class="u-tbl" :label="t('unraid.perms.folder')" :columns="[{ key: 'path' }, { key: 'reason' }, { key: 'size', end: true }]" :rows="kept.map((k) => ({ path: k.path, reason: t(`unraid.perms.reason.${k.reason}`), size: k.size != null ? fmtSize(k.size) || '0 B' : '' }))">
        <template #cell="{ column, value }"><Pill v-if="column.key === 'reason'">{{ value }}</Pill><template v-else>{{ value }}</template></template>
      </Table>
    </SubCard>
  </template>
</template>
