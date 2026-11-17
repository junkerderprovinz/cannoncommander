<script setup lang="ts">
// Laufwerke: every pool and array disk with how full it is and was, its temperature and its last check. All of it is
// what the server already knows, so a sleeping disk is shown with its last values and never woken.
import { FieldRow, Meter, Pill, Selector, SubCard } from '@glimstone/vue';
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import { on } from '@/api/sse';
import { type UnraidDrive, type UnraidDrives, unraid } from '@/api/unraid';
import { fmtSize } from '@/core/format';
import { G } from '@/core/glyphs';
import { locale, plural, t } from '@/i18n';
import { diskName } from '@/features/jobs/label';
import { segItems } from '@/ui/kit';
import { changeOf, chartOf, clockOf, dateOf, dayText, fillOf, FULL, fullInWeeks, PAD, type Range, RANGES, sparkOf } from './drives';
import PointBubble from './PointBubble.vue';

const emit = defineEmits<{ summary: [text: string] }>();
const data = ref<UnraidDrives | null>(null);
const picked = ref('');
const weeks = ref<Range>(8);
const box = ref<HTMLElement | null>(null);
const size = reactive({ w: 0, h: 0 });

async function load() {
  try {
    data.value = await unraid.drives();
  } catch {
    data.value = { pools: [], array: [] };
  }
}
// A disk that falls asleep or wakes up changes what the rows say; the fill level moves slowly enough for a minute.
const offSpin = on('unraid.spin', () => void load());
const every = setInterval(() => void load(), 60_000);

const pickable = computed(() => [...(data.value?.pools ?? []), ...(data.value?.array ?? []).filter((d) => d.kind !== 'parity')]);
const cur = computed(() => pickable.value.find((d) => d.name === picked.value) ?? pickable.value[0]);
watch(cur, (d) => { if (d && picked.value !== d.name) picked.value = d.name; });

const nameOf = (d: UnraidDrive) => (d.kind === 'parity' ? t('diskusage.drives.parity') + (d.name.replace(/^parity/, '') ? ' ' + d.name.replace(/^parity/, '') : '') : diskName(d.name));
const kindOf = (d: UnraidDrive) => (d.transport === 'nvme' ? 'NVMe' : d.rotational ? 'HDD' : 'SSD');
const subOf = (d: UnraidDrive) => (d.fsSize ? t('diskusage.drives.free', { size: fmtSize(d.fsSize), free: fmtSize(d.fsSize - d.used) || '0 B' }) : `${kindOf(d)}, ${fmtSize(d.size)}`);
const pct = (v: number) => t('unraid.pct', { pct: Math.round(v * 100) });
const moonTip = (d: UnraidDrive) => (d.asleepSince
  ? t('diskusage.drives.moonSince', { disk: nameOf(d), time: clockOf(d.asleepSince) })
  : t('diskusage.drives.moon', { disk: nameOf(d) }));
const parityRun = computed(() => {
  const p = data.value?.parity;
  if (!p) return null;
  const date = dateOf(p.at);
  return { text: p.exit < 0 ? t('diskusage.drives.cancelled', { date }) : t('diskusage.drives.run', { date, errors: plural(p.errors, 'diskusage.drives.errors') }), tone: p.exit === 0 && !p.errors ? 'ok' as const : 'warn' as const };
});

const summary = computed(() => {
  const d = data.value;
  if (!d) return ' ';
  const disks = d.array.filter((x) => x.kind === 'data');
  const words = { pools: plural(d.pools.length, 'diskusage.drives.pools'), disks: plural(disks.length, 'diskusage.drives.disks') };
  const asleep = [...d.pools, ...disks].filter((x) => x.asleep).length;
  return asleep ? plural(asleep, 'diskusage.drives.footAsleep', words) : t('diskusage.drives.foot', words);
});
watch(summary, (s) => emit('summary', s), { immediate: true });

const chart = computed(() => (cur.value ? chartOf(cur.value, weeks.value, size.w || 560, size.h || 240) : null));
const change = computed(() => {
  const c = cur.value ? changeOf(cur.value.history, weeks.value) : null;
  if (c == null) return '';
  const words = plural(weeks.value, 'diskusage.drives.weeks');
  return t('diskusage.drives.change', { change: (c >= 0 ? '+' : '−') + (fmtSize(Math.abs(c)) || '0 B'), weeks: words });
});
const outlook = computed(() => {
  const n = cur.value ? fullInWeeks(cur.value, weeks.value) : null;
  return n == null ? '' : plural(n, 'diskusage.drives.fullIn');
});
const rangeItems = computed(() => segItems(RANGES.map((w) => [w, plural(w, 'diskusage.drives.weeks')] as const)));

interface Fact { label: string; text: string; tone?: 'ok' | 'warn'; moon?: boolean }
const facts = computed<Fact[]>(() => {
  const d = cur.value;
  if (!d) return [];
  const out: Fact[] = [d.asleep
    ? { label: t('diskusage.drives.facts.state'), text: d.asleepSince ? t('diskusage.drives.asleepSince', { time: clockOf(d.asleepSince) }) : t('diskusage.drives.asleep'), moon: true }
    : d.status && d.status !== 'DISK_OK' ? { label: t('diskusage.drives.facts.state'), text: d.status, tone: 'warn' } : { label: t('diskusage.drives.facts.state'), text: t('diskusage.drives.running') }];
  const fs = (d.fsType || '').toUpperCase() + (d.profile ? `, ${d.profile}` : '');
  if (d.kind === 'pool') {
    const z = d.zfs;
    if (z?.scrub) out.push({ label: t('diskusage.drives.facts.scrub'), text: t('diskusage.drives.run', { date: dateOf(z.scrub.at), errors: plural(z.scrub.errors, 'diskusage.drives.errors') }), tone: z.scrub.errors ? 'warn' : 'ok' });
    if (z?.fragmentation != null) out.push({ label: t('diskusage.drives.facts.frag'), text: pct(z.fragmentation / 100) });
    if (z?.compressRatio) out.push({ label: t('diskusage.drives.facts.compression'), text: `${z.compressRatio.toLocaleString(locale(), { maximumFractionDigits: 2 })} ×` });
    if (fs) out.push({ label: t('diskusage.drives.facts.fs'), text: fs });
    return out;
  }
  if (parityRun.value) out.push({ label: t('diskusage.drives.facts.parity'), ...parityRun.value });
  if (fs) out.push({ label: t('diskusage.drives.facts.fs'), text: fs });
  out.push({ label: t('diskusage.drives.facts.drive'), text: `${kindOf(d)}, ${fmtSize(d.size)}` });
  return out;
});

const hover = ref<{ x: number; y: number; i: number } | null>(null);
function move(e: PointerEvent) {
  const c = chart.value, svg = e.currentTarget as SVGSVGElement;
  if (!c) return;
  const r = svg.getBoundingClientRect(), x = e.clientX - r.left;
  let i = 0;
  c.pts.forEach((p, k) => { if (Math.abs(p.x - x) < Math.abs(c.pts[i]!.x - x)) i = k; });
  hover.value = { x: r.left + c.pts[i]!.x, y: r.top + c.pts[i]!.y, i };
}
const hovered = computed(() => (hover.value && chart.value ? chart.value.pts[hover.value.i] : undefined));

let ro: ResizeObserver | null = null;
onMounted(() => {
  void load();
  ro = new ResizeObserver(() => { const r = box.value?.getBoundingClientRect(); if (r) Object.assign(size, { w: r.width, h: r.height }); });
  if (box.value) ro.observe(box.value);
});
onBeforeUnmount(() => {
  offSpin();
  clearInterval(every);
  ro?.disconnect();
});
</script>

<template>
  <div class="du-drives">
    <div class="du-col">
      <SubCard v-for="p in data?.pools ?? []" :key="p.name" :title="t('diskusage.drives.pool', { name: nameOf(p) })">
        <template v-for="d in [p]" :key="d.name">
          <button type="button" class="du-drow" :class="{ sel: cur?.name === d.name, asleep: d.asleep }" :data-drive="d.name" :aria-pressed="cur?.name === d.name" @click="picked = d.name">
            <span class="du-dname"><G n="IconZFS" /><span class="du-dtext"><span class="n">{{ nameOf(d) }}<i v-if="d.asleep" class="u-moon" :data-tip="moonTip(d)"><G n="IconMoon" /></i></span><span class="sub">{{ subOf(d) }}</span></span></span>
            <svg v-if="sparkOf(d)" class="du-spark" viewBox="0 0 100 28" preserveAspectRatio="none" aria-hidden="true"><path class="area" :d="sparkOf(d)!.area" /><path :d="sparkOf(d)!.line" vector-effect="non-scaling-stroke" /></svg><span v-else></span>
            <span class="du-fill"><Meter quiet size="medium" :threshold="FULL" :value="fillOf(d)" :label="t('diskusage.drives.used', { name: nameOf(d) })" /><span class="glim-num">{{ pct(fillOf(d)) }}</span></span>
            <span class="du-temp glim-num" :class="{ stale: !!d.tempAt }" :data-tip="d.tempAt ? t('diskusage.drives.tempAt', { time: clockOf(d.tempAt) }) : undefined">{{ d.temp != null ? `${d.temp} °C` : '' }}</span>
          </button>
        </template>
        <div v-for="m in p.members ?? []" :key="m.name" class="du-drow member" :class="{ asleep: m.asleep }" :data-member="m.name">
          <span class="du-dname"><G n="IconDatabase" /><span class="du-dtext"><span class="n">{{ m.device || nameOf(m) }}</span><span class="sub">{{ kindOf(m) }}</span></span></span>
          <span></span><span></span>
          <span class="du-temp glim-num" :class="{ stale: !!m.tempAt }">{{ m.temp != null ? `${m.temp} °C` : '' }}</span>
        </div>
      </SubCard>
      <SubCard v-if="data?.array.length" :title="t('diskusage.drives.array')" :hint="t('diskusage.drives.hint')">
        <template v-for="d in data.array" :key="d.name">
          <div v-if="d.kind === 'parity'" class="du-drow member" :class="{ asleep: d.asleep }" :data-member="d.name">
            <span class="du-dname"><G n="IconShieldOn" /><span class="du-dtext"><span class="n">{{ nameOf(d) }}<i v-if="d.asleep" class="u-moon" :data-tip="moonTip(d)"><G n="IconMoon" /></i></span><span class="sub">{{ subOf(d) }}</span></span></span>
            <span></span>
            <span class="du-note">{{ parityRun?.text }}</span>
            <span class="du-temp glim-num" :class="{ stale: !!d.tempAt }" :data-tip="d.tempAt ? t('diskusage.drives.tempAt', { time: clockOf(d.tempAt) }) : undefined">{{ d.temp != null ? `${d.temp} °C` : '' }}</span>
          </div>
          <button v-else type="button" class="du-drow" :class="{ sel: cur?.name === d.name, asleep: d.asleep }" :data-drive="d.name" :aria-pressed="cur?.name === d.name" @click="picked = d.name">
            <span class="du-dname"><G n="IconDatabase" /><span class="du-dtext"><span class="n">{{ nameOf(d) }}<i v-if="d.asleep" class="u-moon" :data-tip="moonTip(d)"><G n="IconMoon" /></i></span><span class="sub">{{ subOf(d) }}</span></span></span>
            <svg v-if="sparkOf(d)" class="du-spark" viewBox="0 0 100 28" preserveAspectRatio="none" aria-hidden="true"><path class="area" :d="sparkOf(d)!.area" /><path :d="sparkOf(d)!.line" vector-effect="non-scaling-stroke" /></svg><span v-else></span>
            <span class="du-fill"><Meter quiet size="medium" :threshold="FULL" :value="fillOf(d)" :label="t('diskusage.drives.used', { name: nameOf(d) })" /><span class="glim-num">{{ pct(fillOf(d)) }}</span></span>
            <span class="du-temp glim-num" :class="{ stale: !!d.tempAt }" :data-tip="d.tempAt ? t('diskusage.drives.tempAt', { time: clockOf(d.tempAt) }) : undefined">{{ d.temp != null ? `${d.temp} °C` : '' }}</span>
          </button>
        </template>
      </SubCard>
    </div>
    <div v-if="cur" class="du-col">
      <section class="glim-subcard du-chart">
        <div class="du-chart-head"><h3 class="glim-subcard-title"><span>{{ t('diskusage.drives.history', { name: nameOf(cur) }) }}</span></h3><Selector id="du-range" v-model="weeks" :items="rangeItems" :label="t('diskusage.drives.range')" size="sm" inline /></div>
        <div ref="box" class="du-chart-box">
          <svg v-if="chart" role="img" :aria-label="t('diskusage.drives.chartLabel', { name: nameOf(cur), weeks: plural(weeks, 'diskusage.drives.weeks') })" :viewBox="`0 0 ${size.w || 560} ${size.h || 240}`" @pointermove="move" @pointerleave="hover = null">
            <g v-for="tk in chart.ticks" :key="tk.label"><line class="grid" :x1="PAD.l" :x2="(size.w || 560) - PAD.r" :y1="tk.y" :y2="tk.y" /><text class="axis glim-num" :x="PAD.l - 8" :y="tk.y + 4" text-anchor="end">{{ pct(tk.label / 100) }}</text></g>
            <text v-for="(mk, i) in chart.marks" :key="mk.day" class="axis" :x="mk.x" :y="(size.h || 240) - 4" :text-anchor="mk.anchor">{{ i === chart.marks.length - 1 ? t('format.today') : dayText(mk.day, { day: '2-digit', month: '2-digit' }) }}</text>
            <path class="area" :d="chart.area" /><path class="line" :d="chart.line" />
            <g v-if="hovered"><line class="cross" :x1="hovered.x" :x2="hovered.x" :y1="PAD.t" :y2="(size.h || 240) - PAD.b" /><circle class="dot" r="4.5" :cx="hovered.x" :cy="hovered.y" /></g>
          </svg>
          <p v-else class="du-nohistory">{{ t('diskusage.drives.noHistory') }}</p>
        </div>
        <div class="du-chart-foot"><span>{{ cur.asleep && cur.asleepSince ? t('diskusage.drives.asOf', { time: clockOf(cur.asleepSince), name: nameOf(cur) }) : t('diskusage.drives.perDay') }}</span><span class="glim-num">{{ change }}</span></div>
        <p v-if="outlook" class="du-outlook">{{ outlook }}</p>
      </section>
      <SubCard :title="t('diskusage.drives.details', { name: nameOf(cur) })">
        <div class="du-facts">
          <FieldRow v-for="f in facts" :key="f.label" :label="f.label"><Pill :tone="f.tone"><G v-if="f.moon" n="IconMoon" />{{ f.text }}</Pill></FieldRow>
        </div>
      </SubCard>
    </div>
    <PointBubble :at="hover">
      <template v-if="hovered && cur">
        <b>{{ dayText(hovered.day) }}</b>
        <span>{{ t('diskusage.drives.chartTip', { pct: pct(hovered.v), free: fmtSize(cur.fsSize * (1 - hovered.v)) || '0 B' }) }}</span>
      </template>
    </PointBubble>
  </div>
</template>
