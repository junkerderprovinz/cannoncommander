<script setup lang="ts">
// Where an entry really lies, for Eigenschaften: the disks and pools under the user share, and the other names of a
// file with hard links.
import { computed } from 'vue';
import { fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { t } from '@/i18n';
import CopyBtn from '@/ui/CopyBtn.vue';
import { diskName } from '@/features/jobs/label';
import Frow from '@/ui/Frow.vue';
import { useUnraid } from './state';

const props = defineProps<{ path: string }>();
const U = useUnraid();
const w = computed(() => U.where(props.path));
const pools = computed(() => U.pools);

</script>

<template>
  <div v-if="w" class="well glim-well bk-form u-where"><div class="eyebrow">{{ t('unraid.where.title') }}</div>
    <Frow :label="t('unraid.where.on')" :tip="t('unraid.where.onTip')">
      <span class="bk-val u-pills">
        <template v-if="w.on.length"><span v-for="o in w.on" :key="o.device" class="pill glim-num" :class="{ state: pools.includes(o.device) }">{{ diskName(o.device) }}{{ w.files > 1 ? `, ${fmtSize(o.size) || '0 B'}` : '' }}</span></template>
        <span v-else class="muted">{{ t('unraid.where.nothing') }}</span>
      </span>
    </Frow>
    <Frow v-if="!w.dir" :label="t('unraid.where.links')" :tip="t('unraid.where.linksTip')">
      <span v-if="w.links?.paths.length" class="bk-val u-links">
        <span v-for="p in w.links.paths" :key="p" class="bk-pathv"><span class="bk-path mono" :title="p"><bdi>{{ p }}</bdi></span>
          <CopyBtn :text="p" :tip="t('unraid.where.copyPath')" :done="t('unraid.where.copied')" /></span>
      </span>
      <span v-else class="bk-val">{{ t('unraid.where.linksNone') }}</span>
    </Frow>
  </div>
</template>
