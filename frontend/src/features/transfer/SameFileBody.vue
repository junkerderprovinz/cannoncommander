<script setup lang="ts">
// Through /mnt/diskN and /mnt/user the same file can sit on both ends of a copy; Unraid then cuts it to 0 bytes.
// This window names the files and offers the two safe ways.
import { segItems } from '@/ui/kit';
import { FieldRow, Selector, SubCard, Table } from '@glimstone/vue';
import { computed } from 'vue';
import type { UnraidTransferReport } from '@/api/ops';
import { fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import { plural, t } from '@/i18n';
import { diskName } from '@/features/jobs/label';

export interface SafeWay { way: 'disk' | 'share'; disk: string }
const props = defineProps<{ kind: 'copy' | 'move'; from: string; dest: string; trap: NonNullable<UnraidTransferReport['trap']>; st: SafeWay; plan: string }>();
const fromUser = computed(() => props.from.startsWith('/mnt/user'));
const verb = computed(() => t(`transfer.${props.kind}`));
const ways = computed(() => [['disk', t('transfer.diskToDisk')], ['share', t('transfer.shareToShare')]] as const);
const disks = computed(() => props.trap.targets.map((d) => [d.disk, t('transfer.diskFree', { disk: diskName(d.disk), free: fmtSize(d.free) })] as const));
</script>

<template>
  <SubCard class="u-warnwell">
    <div class="u-alert"><G n="IconWarning" /><div><b>{{ t('transfer.sameAlert') }}</b><p>{{ t('transfer.sameText', { disk: fromUser ? dest : from, user: fromUser ? from : dest, verb }) }}</p></div></div>
    <Table class="u-tbl" :label="t('transfer.wayLabel')" :columns="[{ key: 'path' }, { key: 'disk' }, { key: 'size', end: true }]" :rows="trap.files.slice(0, 6).map((f) => ({ path: f.path, disk: diskName(f.device), size: fmtSize(f.size) }))" :more="trap.files.length > 6 ? plural(trap.files.length - 6, 'transfer.more') : undefined" />
  </SubCard>
  <SubCard :title="t('transfer.safeWay')" class="bk-form">
    <FieldRow :label="t('transfer.wayLabel')" :hint="t('transfer.wayTip')"><Selector id="u-way" v-model="st.way" :items="segItems(ways)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow>
    <div v-if="st.way === 'disk'" id="u-diskrow"><FieldRow :label="t('transfer.targetDisk')" :hint="t('transfer.targetDiskTip', { disk: diskName(trap.device) })"><Selector id="u-disk" v-model="st.disk" :items="segItems(disks)" size="sm" activation="manual" inline button-height pairs-on-phone /></FieldRow></div>
    <p id="u-plan" class="u-plan">{{ plan }}</p>
  </SubCard>
</template>
