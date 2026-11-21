<script setup lang="ts">
// The sentence and the button say what will happen, so nobody deletes for good by accident.
import { Checkbox } from '@glimstone/vue';
import { computed } from 'vue';
import { fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import type { Entry } from '@/files/entries';
import { t } from '@/i18n';
import { objects } from '@/features/jobs/label';

// bin names the trash of the system in the desktop app, which keeps entries as long as the system decides.
const props = defineProps<{ items: Entry[]; days: number; bin?: string; st: { trash: boolean } }>();
const size = computed(() => fmtSize(props.items.reduce((a, k) => a + (k.size || 0), 0)) || '0 B');
</script>

<template>
  <div class="xsheet">
    <span class="xsheet-tile"><G n="IconTrash" /></span>
    <span class="xsheet-say">
      <b>{{ st.trash ? t('trash.askTrash') : t('trash.askForever') }}</b>
      <span>{{ items.length === 1 ? items[0]!.name : objects(items.length) }}</span>
    </span>
  </div>
  <p id="xwhere" class="xnote glim-num">{{ !st.trash ? t('trash.whereGone', { size }) : bin ? t('trash.whereSystem', { size, bin }) : t('trash.whereTrash', { size, days }) }}</p>
  <Checkbox id="totrash" v-model="st.trash" class="xtrash" :label="t('trash.toTrash')" :title="bin ? t('trash.toTrashTipSystem', { bin }) : t('trash.toTrashTip', { days })" />
</template>
