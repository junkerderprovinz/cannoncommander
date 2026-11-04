<script setup lang="ts">
// Through /mnt/diskN and /mnt/user the same file can sit on both ends of a copy; Unraid then cuts it to 0 bytes.
// This window names the files and offers the two safe ways.
import { computed } from 'vue';
import type { UnraidTransferReport } from '@/api/ops';
import { fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
import { diskName } from '@/features/jobs/label';
import Frow from '@/ui/Frow.vue';
import Seg from '@/ui/Seg.vue';

export interface SafeWay { way: 'disk' | 'share'; disk: string }
const props = defineProps<{ kind: 'copy' | 'move'; from: string; dest: string; trap: NonNullable<UnraidTransferReport['trap']>; st: SafeWay; plan: string }>();
const fromUser = computed(() => props.from.startsWith('/mnt/user'));
const verb = computed(() => t(`transfer.${props.kind}`));
const ways = computed(() => [['disk', t('transfer.diskToDisk')], ['share', t('transfer.shareToShare')]] as const);
const disks = computed(() => props.trap.targets.map((d) => [d.disk, t('transfer.diskFree', { disk: diskName(d.disk), free: fmtSize(d.free) })] as const));
</script>

<template>
  <div class="well glim-well u-warnwell">
    <div class="u-alert"><G n="IconWarning" /><div><b>{{ t('transfer.sameAlert') }}</b><p>{{ t('transfer.sameText', { disk: fromUser ? dest : from, user: fromUser ? from : dest, verb }) }}</p></div></div>
    <table class="tbl u-tbl">
      <tr v-for="f in trap.files.slice(0, 6)" :key="f.path"><td class="mono">{{ f.path }}</td><td>{{ diskName(f.device) }}</td><td class="glim-num r">{{ fmtSize(f.size) }}</td></tr>
      <tr v-if="trap.files.length > 6"><td colspan="3">{{ t('transfer.more', { n: trap.files.length - 6 }) }}</td></tr>
    </table>
  </div>
  <div class="well glim-well bk-form"><div class="eyebrow">{{ t('transfer.safeWay') }}</div>
    <Frow :label="t('transfer.wayLabel')" :tip="t('transfer.wayTip')"><Seg id="u-way" v-model="st.way" :options="ways" /></Frow>
    <div v-if="st.way === 'disk'" id="u-diskrow"><Frow :label="t('transfer.targetDisk')" :tip="t('transfer.targetDiskTip', { disk: diskName(trap.device) })"><Seg id="u-disk" v-model="st.disk" :options="disks" /></Frow></div>
    <p id="u-plan" class="u-plan">{{ plan }}</p>
  </div>
</template>
