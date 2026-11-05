<script setup lang="ts">
// The colours: rename one, give it another shade, delete it or add a new one. A picked shade proposes its name.
import { COLOR_NAMES_DE, nearestColorName } from '@glimstone/core';
import { ColorPicker, FieldRow, IconButton, TextField } from '@glimstone/vue';
import { onMounted, reactive, ref } from 'vue';
import { glyphComponent } from '@/core/glyphs';
import { toast } from '@/core/toast';
import { cancelBtn, type WinEntry } from '@/core/windows';
import { t } from '@/i18n';
import { pickerWords } from '@/ui/kit';
import { BUILT_IN, useLabels } from '@/stores/labels';
import { stepFoot } from '@/features/config/shared';
import { files } from './counts';

const props = defineProps<{ win: WinEntry }>();
const labels = useLabels();
interface Row { id?: string; name: string; hex: string; count: number; was: string }
const rows = reactive<Row[]>(labels.colours.map((x) => ({ id: x.id, name: labels.colourName(x), hex: x.hex, count: x.count, was: labels.colourName(x) })));
const fresh = reactive({ hex: '#2ec4b6', named: '' });
const newName = ref('');
const newInput = ref<InstanceType<typeof TextField> | null>(null);
const pick = ref<{ anchor: HTMLElement; hex: string; row: Row | null } | null>(null);

function add() {
  rows.push({ name: newName.value.trim() || (nearestColorName(fresh.hex, COLOR_NAMES_DE) ?? t('labels.colour')), hex: fresh.hex, count: 0, was: '' });
  newName.value = '';
  fresh.named = '';
}
function picked(hex: string, name: string) {
  const row = pick.value?.row;
  if (row) { row.hex = hex; return; }
  fresh.hex = hex;
  // The suggestion only fills the field while nobody has typed a name of their own.
  if (!newName.value.trim() || newName.value === fresh.named) { newName.value = name; fresh.named = name; }
  newInput.value?.input?.focus();
}

async function save() {
  // A colour the app comes with keeps following the language as long as its name is the one the app gave it.
  await labels.saveColours(rows.map((r) => ({ ...(r.id ? { id: r.id } : {}), hex: r.hex,
    name: r.id && BUILT_IN.includes(r.id) && r.name.trim() === r.was ? '' : r.name.trim() || t('labels.colour') })));
  props.win.close();
  toast(t('labels.coloursSaved'));
}
onMounted(() => stepFoot(props.win, [cancelBtn(), { label: t('labels.take'), g: 'IconCheck', primary: true, run: () => { save(); return false; } }]));
</script>

<template>
  <div class="well glim-well">
    <FieldRow :label="t('labels.newColour')" :hint="t('labels.newColourTip')">
      <span class="mk-new"><button class="mk-swatch" data-colpick="new" :style="{ '--c': fresh.hex }" :data-tip="t('labels.pickNew')" :aria-label="t('labels.pickNewAria')" @click="pick = { anchor: $event.currentTarget as HTMLElement, hex: fresh.hex, row: null }"></button><TextField id="mk-new" ref="newInput" v-model="newName" :placeholder="t('labels.newColourPh')" autocomplete="off" width="medium" @keydown.enter.prevent="add" /><IconButton :tip="t('labels.add')" :glyph="glyphComponent('IconAdd')" data-coladd @click="add" /></span>
    </FieldRow>
    <div id="mk-list">
      <div v-for="(r, i) in rows" :key="i" class="mk-row">
        <button class="mk-swatch" :data-colpick="i" :style="{ '--c': r.hex }" :data-tip="t('labels.pickShade')" :aria-label="t('labels.shadeFor', { name: r.name })" @click="pick = { anchor: $event.currentTarget as HTMLElement, hex: r.hex, row: r }"></button>
        <TextField v-model="r.name" :data-colname="i" :aria-label="t('labels.nameFor', { name: r.name })" />
        <span class="muted">{{ files(r.count) }}</span>
        <IconButton :tip="t('labels.removeColour')" :glyph="glyphComponent('IconTrash')" :data-coldel="i" @click="rows.splice(i, 1)" />
      </div>
      <div v-if="!rows.length" class="muted mk-none">{{ t('labels.noColours') }}</div>
    </div>
  </div>
  <ColorPicker v-if="pick" :model-value="pick.hex" :anchor="pick.anchor" :names="COLOR_NAMES_DE" :text="pickerWords()" @apply="(hex, name) => picked(hex, name ?? '')" @close="pick = null" />
</template>
