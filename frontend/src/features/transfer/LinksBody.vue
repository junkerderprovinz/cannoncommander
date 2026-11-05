<script setup lang="ts">
// The files of a transfer that are hard links, and what the transfer does to them.
import { SubCard } from '@glimstone/vue';
import type { UnraidTransferReport } from '@/api/ops';
import { fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';

defineProps<{ kind: 'copy' | 'move'; dest: string; links: NonNullable<UnraidTransferReport['links']>; linkable: boolean }>();
</script>

<template>
  <SubCard class="u-warnwell">
    <div class="u-alert"><G n="IconWarning" /><div>
      <b>{{ kind === 'copy' ? t('transfer.linksCopy') : t('transfer.linksMove') }}</b>
      <p>{{ kind === 'copy' ? t('transfer.linksCopyText', { size: fmtSize(links.extra) }) : t('transfer.linksMoveText', { dest, size: fmtSize(links.extra) }) }}</p>
    </div></div>
    <table class="tbl u-tbl">
      <tr v-for="f in links.files.slice(0, 6)" :key="f.path"><td class="mono">{{ f.path }}</td><td class="u-also">{{ t('transfer.linksAlso') }} <template v-for="(p, i) in f.others" :key="p">{{ i ? ', ' : '' }}<span class="mono">{{ p }}</span></template></td><td class="glim-num r">{{ fmtSize(f.size) }}</td></tr>
      <tr v-if="links.files.length > 6"><td colspan="3">{{ t('transfer.more', { n: links.files.length - 6 }) }}</td></tr>
    </table>
  </SubCard>
  <p v-if="!linkable" class="u-plan">{{ t('transfer.linksNo') }}</p>
</template>
