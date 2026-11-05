<script setup lang="ts">
// Before a copy into a user share, the question Unraid cannot ask: will the disk it is about to pick hold the largest
// file? Unraid picks the disk when a file is created and cannot know its final size.
import { Button } from '@glimstone/vue';
import { computed } from 'vue';
import type { UnraidTransferReport } from '@/api/ops';
import { fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import { split } from '@/files/paths';
import { t } from '@/i18n';
import { diskName } from '@/features/jobs/label';
import { shareRel } from './paths';

const props = defineProps<{ space: NonNullable<UnraidTransferReport['space']>; dest: string }>();
const emit = defineEmits<{ dest: [path: string] }>();
const s = computed(() => props.space);
const why = computed(() => t(`transfer.space.${s.value.problem}`, {
  share: s.value.share, floor: fmtSize(s.value.floor), total: fmtSize(s.value.total), room: fmtSize(s.value.room),
  disk: diskName(s.value.disk), free: fmtSize(s.value.free), pool: diskName(s.value.pool),
  alloc: t(`transfer.space.alloc.${s.value.allocator}`), name: split(s.value.biggest?.path || '').pop() || '', size: fmtSize(s.value.biggest?.size),
}));
</script>

<template>
  <div class="u-alert u-space"><G n="IconWarning" /><div><b>{{ t('transfer.space.title') }}</b><p>{{ why }}</p>
    <p v-if="s.problem === 'bigFile'" class="muted">{{ t('transfer.space.tip') }}</p>
    <Button v-if="s.roomy" :label="t('transfer.space.onDisk', { disk: diskName(s.roomy), free: fmtSize(s.roomyFree) })" label-key="IconDatabase" :data-space-disk="s.roomy" @click="emit('dest', `/mnt/${s.roomy}/${shareRel(dest)}`)" />
  </div></div>
</template>
