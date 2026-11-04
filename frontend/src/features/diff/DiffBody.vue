<script setup lang="ts">
// Two files side by side, changed lines tinted, and a jump from one changed place to the next.
import { computed, onMounted, ref } from 'vue';
import type { Diff, DiffRow, DiffSide } from '@/api/tools';
import { fmtSize, fmtTime } from '@/core/format';
import { G, glyphOf } from '@/core/glyphs';
import { closeBtn, type WinEntry } from '@/core/windows';
import { plural } from '@/ui/kit';
import { parent, split } from '@/files/paths';
import { t } from '@/i18n';
import { countOf, hunksOf, inline, type Piece } from './diff';

const props = defineProps<{ diff: Diff; win: WinEntry; close: () => void }>();
const rows = computed(() => props.diff.rows);
const hunks = computed(() => hunksOf(rows.value));
const at = ref(-1);
const body = ref<HTMLElement | null>(null);

const nameOf = (s: DiffSide) => split(s.path).pop() || s.path;
const CLS: Record<DiffRow['t'], [string, string]> = { eq: ['', ''], chg: ['chg', 'chg'], del: ['del', 'gap'], add: ['gap', 'add'] };

function cells(r: DiffRow): [Piece[], Piece[]] {
  const a = r.l != null ? props.diff.left.lines[r.l]! : '', b = r.r != null ? props.diff.right.lines[r.r]! : '';
  if (r.t === 'chg') return inline(a, b);
  return [[{ text: a }], [{ text: b }]];
}
const lines = computed(() => rows.value.map((r) => ({ r, pieces: cells(r), cls: CLS[r.t] })));

function summary() {
  const n = hunks.value.length;
  if (!n) return ' ';
  return t('diff.summary', { places: plural(n, 'diff.places'), chg: countOf(rows.value, 'chg'), del: countOf(rows.value, 'del'), add: countOf(rows.value, 'add') });
}
function foot(start: string) {
  const none = !hunks.value.length;
  props.win.foot = [{ start }, closeBtn(),
    { label: t('diff.prev'), g: 'IconMoveUp', disabled: none, run: () => { jump(-1); return false; } },
    { label: t('diff.next'), g: 'IconMoveDown', primary: true, disabled: none, run: () => { jump(1); return false; } }];
}
function jump(d: number) {
  const n = hunks.value.length;
  if (!n) return;
  at.value = (at.value + d + n) % n;
  body.value?.querySelector(`[data-bk-row="${hunks.value[at.value]}"]`)?.scrollIntoView({ block: 'center' });
  foot(t('diff.at', { n: at.value + 1, all: n }));
}
function key(e: KeyboardEvent) {
  if (e.key === 'F7' || (e.key === 'ArrowDown' && e.altKey)) { e.preventDefault(); jump(e.shiftKey ? -1 : 1); }
  if (e.key === 'ArrowUp' && e.altKey) { e.preventDefault(); jump(-1); }
}

foot(summary());
onMounted(() => {
  props.win.el?.addEventListener('keydown', key);
  setTimeout(() => body.value?.focus(), 0);
});
</script>

<template>
  <div class="bk-dheads">
    <div v-for="s in [diff.left, diff.right]" :key="s.path" class="bk-dhead">
      <span class="bk-tile"><G :n="glyphOf({ name: nameOf(s) })" /></span><span class="bk-name">{{ nameOf(s) }}</span><span class="bk-path muted mono"><bdi>{{ parent(s.path) }}</bdi></span>
      <span class="pill glim-num">{{ fmtSize(s.size) }}</span><span class="pill glim-num">{{ fmtTime(Date.parse(s.mtime)) }}</span>
    </div>
  </div>
  <div class="well bk-fill bk-dwell">
    <div v-if="!hunks.length" class="bk-same"><span class="bk-dot"></span>{{ t('diff.same') }}</div>
    <div id="d-body" ref="body" class="bk-dbody mono" :class="{ hex: diff.binary }" tabindex="0">
      <div v-for="(x, i) in lines" :key="i" class="bk-dl" :class="{ cur: at >= 0 && hunks[at] === i }" :data-bk-row="i">
        <span class="bk-ln glim-num">{{ x.r.l == null ? '' : x.r.l + 1 }}</span><span class="bk-code" :class="x.cls[0]"><template v-for="(p, k) in x.pieces[0]" :key="k"><mark v-if="p.mark">{{ p.text }}</mark><template v-else>{{ p.text }}</template></template><template v-if="!x.pieces[0].length || !x.pieces[0].some((p) => p.text)">{{ ' ' }}</template></span>
        <span class="bk-ln glim-num">{{ x.r.r == null ? '' : x.r.r + 1 }}</span><span class="bk-code" :class="x.cls[1]"><template v-for="(p, k) in x.pieces[1]" :key="k"><mark v-if="p.mark">{{ p.text }}</mark><template v-else>{{ p.text }}</template></template><template v-if="!x.pieces[1].length || !x.pieces[1].some((p) => p.text)">{{ ' ' }}</template></span>
      </div>
    </div>
  </div>
</template>
