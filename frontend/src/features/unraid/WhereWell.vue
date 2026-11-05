<script setup lang="ts">
// Where an entry really lies, for Eigenschaften: the disks and pools under the user share, and the other names of a
// file with hard links.
import { FieldRow, Pill, SubCard } from '@glimstone/vue';
import { computed } from 'vue';
import { fmtSize } from '@/core/format';
import { toast } from '@/core/toast';
import { t } from '@/i18n';
import CopyBtn from '@/ui/CopyBtn.vue';
import { diskName } from '@/features/jobs/label';
import { useUnraid } from './state';

const props = defineProps<{ path: string }>();
const U = useUnraid();
const w = computed(() => U.where(props.path));
const pools = computed(() => U.pools);

</script>

<template>
  <SubCard :title="t('unraid.where.title')" v-if="w" class="bk-form u-where">
    <FieldRow :label="t('unraid.where.on')" :hint="t('unraid.where.onTip')">
      <span class="bk-val u-pills">
        <template v-if="w.on.length"><Pill v-for="o in w.on" :key="o.device" class="glim-num" :tone="pools.includes(o.device) ? 'state' : 'neutral'">{{ diskName(o.device) }}{{ w.files > 1 ? `, ${fmtSize(o.size) || '0 B'}` : '' }}</Pill></template>
        <span v-else class="muted">{{ t('unraid.where.nothing') }}</span>
      </span>
    </FieldRow>
    <FieldRow v-if="!w.dir" :label="t('unraid.where.links')" :hint="t('unraid.where.linksTip')">
      <span v-if="w.links?.paths.length" class="bk-val u-links">
        <span v-for="p in w.links.paths" :key="p" class="bk-pathv"><span class="bk-path mono" :title="p"><bdi>{{ p }}</bdi></span>
          <CopyBtn :text="p" :tip="t('unraid.where.copyPath')" :done="t('unraid.where.copied')" /></span>
      </span>
      <span v-else class="bk-val">{{ t('unraid.where.linksNone') }}</span>
    </FieldRow>
  </SubCard>
</template>
