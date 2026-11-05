<script setup lang="ts">
// The column window: every column under its group, a search over them, and the choice applied together.
import { Checkbox, TextField } from '@glimstone/vue';
import { computed, ref } from 'vue';
import { G } from '@/core/glyphs';
import { t } from '@/i18n';
import { COLUMN_GROUPS, COLUMNS, columnLabel } from './columns';

const props = defineProps<{ on: Set<string> }>();
const q = ref('');
const groups = computed(() => {
  const needle = q.value.trim().toLowerCase();
  return COLUMN_GROUPS.map(([id]) => {
    const name = t(`columnGroups.${id}`);
    const hit = name.toLowerCase().includes(needle);
    return { id, name, cols: COLUMNS.filter((c) => c.group === id && (!needle || hit || columnLabel(c.id).toLowerCase().includes(needle))) };
  }).filter((g) => g.cols.length);
});
const flip = (id: string) => (props.on.has(id) ? props.on.delete(id) : props.on.add(id));
</script>

<template>
  <TextField id="col-q" v-model="q" type="search" :placeholder="t('files.columnsSearch')" autocomplete="off" width="grow" />
  <div class="well glim-well">
    <div class="col-cols">
      <section v-for="gr in groups" :key="gr.id" class="col-group" data-colgroup>
        <div class="eyebrow">{{ gr.name }}</div>
        <Checkbox v-for="c in gr.cols" :key="c.id" class="col-check" :data-col="c.id" :label="columnLabel(c.id)" :disabled="c.fixed" :model-value="on.has(c.id)" @update:model-value="flip(c.id)" />
      </section>
    </div>
    <div v-if="!groups.length" id="col-none" class="empty"><G n="IconSearch" /><div>{{ t('files.columnsNone') }}</div></div>
  </div>
</template>
