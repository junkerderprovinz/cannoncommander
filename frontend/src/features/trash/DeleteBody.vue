<script setup lang="ts">
// The sentence and the button say what will happen, so nobody deletes for good by accident.
import { FieldRow, SubCard, Toggle } from '@glimstone/vue';
import { computed } from 'vue';
import { fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import type { Entry } from '@/files/entries';
import { plural, t } from '@/i18n';
import ItemChips from '@/features/transfer/ItemChips.vue';

// bin names the trash of the system in the desktop app, which keeps entries as long as the system decides.
const props = defineProps<{ items: Entry[]; days: number; bin?: string; st: { trash: boolean } }>();
const size = computed(() => fmtSize(props.items.reduce((a, k) => a + (k.size || 0), 0)) || '0 B');
</script>

<template>
  <div class="xdel"><span class="xbin"><G n="IconTrash" /></span>
    <div><b>{{ items.length === 1 ? t('trash.deleteOne', { name: items[0]!.name }) : plural(items.length, 'trash.deleteMany') }}</b><span id="xwhere" class="glim-num">{{ !st.trash ? t('trash.whereGone', { size }) : bin ? t('trash.whereSystem', { size, bin }) : t('trash.whereTrash', { size, days }) }}</span></div></div>
  <ItemChips v-if="items.length > 1" :items="items" :max="6" />
  <SubCard><FieldRow :label="t('trash.toTrash')" :hint="bin ? t('trash.toTrashTipSystem', { bin }) : t('trash.toTrashTip', { days })"><Toggle id="totrash" v-model="st.trash" /></FieldRow></SubCard>
</template>
