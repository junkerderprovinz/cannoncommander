<script setup lang="ts">
// A spreadsheet as a table, with the formula of the chosen cell above it and a tab per sheet.
import { computed, onMounted, ref } from 'vue';
import { locale, t } from '@/i18n';
import { rawUrl } from '../content';
import { ext, joinPath, plural, stem } from '../kinds';
import { type Cell, colName, readCsv, readXlsx, type Sheet } from '../ooxml';
import type { REmits, RProps } from './types';

const props = defineProps<RProps>();
const emit = defineEmits<REmits & { (e: 'failed'): void }>();

const sheets = ref<Sheet[]>([]);
const tab = ref(0);
const sel = ref<[number, number]>([0, 0]);
const rows = computed(() => sheets.value[tab.value]?.rows ?? []);
const nCols = computed(() => Math.max(7, ...rows.value.map((r) => r?.length ?? 0)));
const nRows = computed(() => (props.compact ? 12 : Math.max(rows.value.length + 6, 24)));
const cell = (r: number, c: number): Cell | undefined => rows.value[r]?.[c];
const ref0 = computed(() => colName(sel.value[1]) + (sel.value[0] + 1));
const formula = computed(() => { const c = cell(...sel.value); return c?.f ?? c?.v ?? ''; });

const num = (n: number) => n.toLocaleString(locale(), { maximumFractionDigits: 2 });

onMounted(async () => {
  try {
    const res = await fetch(rawUrl(joinPath(props.folder, props.n.name)));
    if (!res.ok) throw new Error(String(res.status));
    const e = ext(props.n.name);
    sheets.value = e === 'xlsx'
      ? readXlsx(new Uint8Array(await res.arrayBuffer()), num)
      : [{ name: stem(props.n.name), rows: readCsv(await res.text(), e === 'tsv' ? '\t' : undefined) }];
    emit('info', `${t('viewer.office.sheet')}, ${plural(sheets.value.length, 'sheets')}`);
  } catch {
    emit('failed');
  }
});

defineExpose({
  keys: (_e: KeyboardEvent, combo: string) => {
    const d = ({ ArrowDown: [1, 0], ArrowUp: [-1, 0], ArrowRight: [0, 1], ArrowLeft: [0, -1] } as Record<string, [number, number]>)[combo];
    if (!d || props.compact) return;
    sel.value = [Math.max(0, sel.value[0] + d[0]), Math.max(0, Math.min(nCols.value - 1, sel.value[1] + d[1]))];
    return true;
  },
});
</script>

<template>
  <div v-if="!compact" class="cv-bar"><span class="pill glim-num cv-ref">{{ ref0 }}</span><span class="cv-fx mono">{{ formula }}</span></div>
  <div class="cv-sheet">
    <table>
      <thead><tr><th></th><th v-for="c in nCols" :key="c">{{ colName(c - 1) }}</th></tr></thead>
      <tbody>
        <tr v-for="r in nRows" :key="r">
          <th class="glim-num">{{ r }}</th>
          <td v-for="c in nCols" :key="c" :class="{ r: cell(r - 1, c - 1)?.num, 'glim-num': cell(r - 1, c - 1)?.num, b: r === 1, sel: !compact && sel[0] === r - 1 && sel[1] === c - 1 }"
            :data-rc="`${r - 1},${c - 1}`" @click="sel = [r - 1, c - 1]">{{ cell(r - 1, c - 1)?.v ?? '' }}</td>
        </tr>
      </tbody>
    </table>
  </div>
  <div v-if="!compact && sheets.length > 1" class="cv-tabs">
    <button v-for="(s, k) in sheets" :key="k" class="tab" :class="{ cur: k === tab }" :data-st="k" @click="tab = k; sel = [0, 0]">{{ s.name }}</button>
  </div>
</template>
