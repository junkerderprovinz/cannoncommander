<script setup lang="ts">
// The files of a transfer that are hard links, and what the transfer does to them.
import { SubCard, Table } from '@glimstone/vue';
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
    <Table class="u-tbl" :label="t('transfer.linksAlso')" :columns="[{ key: 'path' }, { key: 'others' }, { key: 'size', end: true }]" :rows="links.files.slice(0, 6).map((f) => ({ path: f.path, others: f.others, size: fmtSize(f.size) }))" :more="links.files.length > 6 ? t('transfer.more', { n: links.files.length - 6 }) : undefined">
      <template #cell="{ row, column, value }">
        <template v-if="column.key === 'others'">{{ t('transfer.linksAlso') }} <template v-for="(p, i) in row.others" :key="p">{{ i ? ', ' : '' }}<span class="mono">{{ p }}</span></template></template>
        <template v-else>{{ value }}</template>
      </template>
    </Table>
  </SubCard>
  <p v-if="!linkable" class="u-plan">{{ t('transfer.linksNo') }}</p>
</template>
