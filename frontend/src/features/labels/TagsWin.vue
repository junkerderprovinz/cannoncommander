<script setup lang="ts">
// The tags: rename one, delete it or add a new one before any file carries it.
import { FieldRow, IconButton, TextField } from '@glimstone/vue';
import { onMounted, reactive, ref } from 'vue';
import { G, glyphComponent } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { t } from '@/i18n';
import { useLabels } from '@/stores/labels';
import { stepFoot } from '@/features/config/shared';
import { files } from './counts';

const props = defineProps<{ win: WinEntry }>();
const labels = useLabels();
const rows = reactive(labels.tags.map((x) => ({ from: x.name as string | undefined, to: x.name, count: x.count })));
const fresh = ref('');

function add() {
  const v = fresh.value.trim().replace(/^#/, '');
  if (!v) return;
  if (!rows.some((r) => r.to === v)) rows.push({ from: undefined, to: v, count: 0 });
  fresh.value = '';
}
async function save() {
  await labels.saveTags(rows.map((r) => ({ ...(r.from ? { from: r.from } : {}), name: r.to.trim().replace(/^#/, '') || r.from || '' })).filter((r) => r.name));
  props.win.close();
  toast(t('labels.tagsSaved'));
}
onMounted(() => stepFoot(props.win, [cancelBtn(), { label: t('labels.take'), g: 'IconCheck', primary: true, run: () => { save(); return false; } }]));
</script>

<template>
  <div class="well glim-well">
    <FieldRow :label="t('labels.newTag')" :hint="t('labels.newTagTip')"><TextField id="mk-new" v-model="fresh" :placeholder="t('labels.newTagPh')" autocomplete="off" width="medium" @keydown.enter.prevent="add" /></FieldRow>
    <div id="mk-list">
      <div v-for="(r, i) in rows" :key="i" class="mk-row">
        <G n="IconHash" /><TextField v-model="r.to" :data-tagname="i" :aria-label="t('labels.tagAria', { name: r.from || r.to })" /><span class="muted">{{ files(r.count) }}</span>
        <IconButton :tip="t('labels.removeTag')" :glyph="glyphComponent('IconTrash')" :data-tagdel="i" @click="rows.splice(i, 1)" />
      </div>
      <div v-if="!rows.length" class="muted mk-none">{{ t('labels.noTags') }}</div>
    </div>
  </div>
</template>
