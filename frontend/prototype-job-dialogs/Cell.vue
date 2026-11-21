<script setup lang="ts">
// Prototype: one window in one state, drawn the way the chosen variant would draw it.
import { Badge, Checkbox, Pill, Toggle } from '@glimstone/vue';
import { computed, ref } from 'vue';
import { G } from '@/core/glyphs';
import { currentFile, type Dialog, fmtNumber, fmtSeconds, JOBS, type Lang, setDetails, speedAt, type State, texts, ui } from './data';
import Flow from './Flow.vue';
import Meter from './Meter.vue';
import Versions from './Versions.vue';
import Win, { type FootButton } from './Win.vue';

const props = defineProps<{ variant: string; dialog: Dialog; state: State; lang: Lang }>();
const t = computed(() => texts(props.lang));
const job = computed(() => JOBS[props.dialog]);
const name = computed(() => (props.dialog === 'delete' ? `3 ${t.value.items}` : job.value.name));
const keep = ref(false);
const all = ref(false);
const deleting = computed(() => props.dialog === 'delete');
const crumbs = (path: string) => path.split('/').filter(Boolean).slice(2).join(' › ');
const where = computed(() => ({ from: crumbs(job.value.from) || job.value.from, to: deleting.value ? t.value.trash : crumbs(job.value.to) }));
const finished = computed(() => ['done', 'errors', 'cancelled'].includes(props.state));
const working = computed(() => !['confirm', 'conflict'].includes(props.state));
const file = computed(() => (props.state === 'running' ? currentFile(job.value.files) : job.value.run.current));

// A running job creeps on by itself so the graph has something to draw.
const pct = computed(() => {
  const base = job.value.run.pct;
  switch (props.state) {
    case 'running': return Math.min(base + ui.tick * 0.2, base + 45);
    case 'done': case 'errors': return 100;
    case 'cancelled': return Math.round(base / 2);
    case 'confirm': return 0;
    default: return base;
  }
});
const speed = computed(() => speedAt(job.value.speed, pct.value));
const gb = (mb: number) => `${fmtNumber(mb / 1024, 1, props.lang)} GB`;
const facts = computed(() => {
  const j = job.value;
  if (deleting.value) return `${Math.round((3 * pct.value) / 100)} / 3 ${t.value.items}`;
  const done = `${gb((j.totalMB * pct.value) / 100)} / ${j.total}`;
  if (props.state !== 'running') return done;
  return `${done} · ${fmtNumber(speed.value, 0, props.lang)} MB/s · ${t.value.left} ${fmtSeconds((j.totalMB * (100 - pct.value)) / 100 / speed.value, props.lang)}`;
});
const tone = computed(() => ({ running: 'active', paused: 'warn', conflict: 'warn', done: 'ok', errors: 'fail', cancelled: 'neutral', confirm: 'neutral' })[props.state] as 'active');
const resultLine = computed(() => ({ done: t.value.doneLine, errors: t.value.errorsLine, cancelled: t.value.cancelledLine } as Record<string, Record<Dialog, string>>)[props.state]?.[props.dialog] ?? '');
const flow = computed(() => ({ from: where.value.from, to: where.value.to, toGlyph: deleting.value ? 'IconTrash' : undefined, file: file.value, moving: props.state === 'running', paused: props.state === 'paused' }));

const foot = computed<FootButton[]>(() => {
  const x = t.value, A = props.variant === 'A', E = props.variant === 'E';
  switch (props.state) {
    case 'confirm':
      return [{ label: x.cancel, g: 'IconClose' }, { label: deleting.value ? x.toTrash : x.dialog[props.dialog], g: job.value.glyph, primary: true }];
    case 'running':
      if (E) return [{ label: x.background, g: 'IconWindowMinimise' }];
      return A ? [{ label: x.background, g: 'IconWindowMinimise' }, { label: x.cancel, g: 'IconClose' }]
        : [{ label: x.background, g: 'IconWindowMinimise' }, { label: x.pause, g: 'IconPause' }, { label: x.cancel, g: 'IconClose' }];
    case 'paused':
      if (E) return [{ label: x.background, g: 'IconWindowMinimise' }, { label: x.resume, g: 'IconPlay', primary: true }];
      return [{ label: x.background, g: 'IconWindowMinimise' }, { label: x.cancel, g: 'IconClose' }, { label: x.resume, g: 'IconPlay', primary: true }];
    case 'conflict':
      return [{ label: x.cancel, g: 'IconClose' }, { label: x.replace, g: 'IconSync' }, { label: x.skip, g: 'IconForward' }, { label: x.both, g: 'IconCopy', primary: true }];
    case 'done':
      return A ? [{ label: x.close, g: 'IconClose', primary: true }] : [{ label: x.open, g: 'IconFolderOpen' }, { label: x.close, g: 'IconClose', primary: true }];
    case 'errors':
      return A ? [{ label: x.close, g: 'IconClose', primary: true }] : [{ label: x.retry, g: 'IconRefresh' }, { label: x.close, g: 'IconClose', primary: true }];
    default:
      return A ? [{ label: x.close, g: 'IconClose', primary: true }] : [{ label: x.goOn, g: 'IconPlay' }, { label: x.close, g: 'IconClose', primary: true }];
  }
});
const title = computed(() => (props.state === 'conflict' && props.variant === 'A' ? (props.lang === 'de' ? 'Konflikte' : 'Conflicts') : t.value.dialog[props.dialog]));
</script>

<template>
  <Win :title="title" :foot="foot">
    <template v-if="variant === 'G' && working" #start>
      <button class="meter-toggle meter-toggle--foot" :aria-expanded="ui.details" @click="setDetails(!ui.details)"><G n="IconExpand" />{{ ui.details ? t.lessDetails : t.moreDetails }}</button>
    </template>
    <template v-else-if="state !== 'confirm' && state !== 'conflict' && variant !== 'G'" #start><Checkbox v-model="keep" :label="t.keep" /></template>
    <template #default>
      <!-- Every variant asks about a clash only when one happens, the same way. -->
      <template v-if="state === 'conflict'">
        <div class="cf-head"><b class="cf-file">{{ job.clash.name }}</b><Badge tone="fail" size="small">{{ t.taken }}</Badge></div>
        <Versions :clash="job.clash" :t="t" />
        <Checkbox v-model="all" :label="t.forAll" />
      </template>

      <!-- A: the windows as they are on the release branch, without the questions that come too early -->
      <template v-else-if="variant === 'A'">
        <template v-if="state === 'confirm' && !deleting">
          <div class="pa-chips"><Pill>{{ name }}</Pill><Pill class="glim-num">{{ job.total }}</Pill></div>
          <div class="pa-row"><span>{{ t.to }}</span><span class="pa-field mono">{{ job.to }}</span></div>
        </template>
        <template v-else-if="state === 'confirm'">
          <div class="pa-list"><div v-for="n in job.items" :key="n" class="pa-item"><G n="IconFile" />{{ n }}</div></div>
          <div class="pa-row"><span>{{ t.trashToggle }}</span><Toggle :model-value="true" :label="t.trashToggle" /></div>
          <p class="muted small">{{ t.trashHint }}</p>
        </template>
        <template v-else>
          <div class="jf-head"><b>{{ name }}</b><Pill class="glim-num">{{ Math.round(pct) }} %</Pill></div>
          <div v-if="finished" class="muted small">{{ resultLine }}</div>
          <Meter :dialog="dialog" :state="state" :pct="pct" :lang="lang" :t="t" :label="name">
            <template v-if="!finished" #lead><Flow compact v-bind="flow" /></template>
            <template v-if="!finished" #facts><div class="muted small glim-num">{{ facts }}</div></template>
          </Meter>
        </template>
      </template>

      <!-- B: one column, the route as one sentence -->
      <template v-else-if="variant === 'B'">
        <div class="pb-sentence"><G :n="job.glyph" /><span><b>{{ name }}</b> <span class="muted">{{ deleting ? t.sentence.delete : t.sentence[dialog] }}</span> <b v-if="!deleting">{{ where.to }}</b></span></div>
        <p v-if="state === 'confirm' && deleting" class="muted small">{{ t.trashHint }}</p>
        <template v-else-if="state !== 'confirm'">
          <div class="pb-big glim-num"><span>{{ Math.round(pct) }} %</span><Badge :tone="tone" size="small">{{ t.badge[state as 'running'] }}</Badge></div>
          <div v-if="finished" class="muted small">{{ resultLine }}</div>
          <Meter :dialog="dialog" :state="state" :pct="pct" :lang="lang" :t="t" :label="name">
            <template v-if="!finished" #lead><Flow compact v-bind="flow" /></template>
            <template v-if="!finished" #facts><div class="muted small glim-num">{{ facts }}</div></template>
          </Meter>
          <div v-if="state === 'errors'" class="pb-failed"><span v-for="f in job.failed" :key="f" class="small"><span class="mono">{{ f }}</span> · {{ t.error }}</span></div>
        </template>
      </template>

      <!-- C: two zones, where it goes on top and how far it is below -->
      <template v-else-if="variant === 'C'">
        <div class="pc-route" :class="{ calm: ui.details && working }">
          <div class="pc-what"><G :n="job.glyph" /><b>{{ name }}</b><span class="muted glim-num">{{ job.total }}</span></div>
          <Flow v-bind="flow" :moving="flow.moving && !ui.details" />
        </div>
        <Toggle v-if="state === 'confirm' && deleting" :model-value="true" :label="t.trashToggle" />
        <div v-else-if="state !== 'confirm'" class="pc-progress">
          <div class="jf-head"><Badge :tone="tone" size="small">{{ t.badge[state as 'running'] }}</Badge><span class="muted small">{{ finished ? resultLine : '' }}</span></div>
          <Meter :dialog="dialog" :state="state" :pct="pct" :lang="lang" :t="t" :label="name">
            <template #facts><div class="pc-stats">
              <div><span class="muted small">{{ t.progress }}</span><b class="glim-num">{{ Math.round(pct) }} %</b></div>
              <div><span class="muted small">{{ deleting ? t.files : t.size }}</span><b class="glim-num">{{ deleting ? `${Math.round(3 * pct / 100)} / 3` : gb(job.totalMB * pct / 100) }}</b></div>
              <div><span class="muted small">{{ t.speed }}</span><b class="glim-num">{{ state === 'running' && !deleting ? `${fmtNumber(speed, 0, lang)} MB/s` : '…' }}</b></div>
              <div><span class="muted small">{{ t.left }}</span><b class="glim-num">{{ state === 'running' && !deleting ? fmtSeconds(job.totalMB * (100 - pct) / 100 / speed, lang) : '…' }}</b></div>
            </div></template>
          </Meter>
          <div v-if="state === 'errors'" class="pb-failed"><span v-for="f in job.failed" :key="f" class="small"><span class="mono">{{ f }}</span> · {{ t.error }}</span></div>
        </div>
      </template>

      <!-- D: one line and a bar, everything else behind Mehr Details -->
      <template v-else-if="variant === 'D'">
        <div class="pd-line"><b>{{ state === 'confirm' ? t.ask[dialog] : name }}</b><span v-if="working" class="glim-num muted">{{ Math.round(pct) }} %</span></div>
        <template v-if="state === 'confirm'">
          <div class="pd-chip"><G :n="job.glyph" /><span>{{ name }}</span><template v-if="!deleting"><G n="IconForward" class="pd-sep" /><span>{{ where.to }}</span></template></div>
          <p v-if="deleting" class="muted small">{{ t.trashHint }}</p>
        </template>
        <template v-else>
          <Meter :dialog="dialog" :state="state" :pct="pct" :lang="lang" :t="t" :label="name">
            <div v-if="state === 'errors'" class="pb-failed"><span v-for="f in job.failed" :key="f" class="small"><span class="mono">{{ f }}</span> · {{ t.error }}</span></div>
          </Meter>
          <div v-if="finished" class="small muted">{{ resultLine }}</div>
        </template>
      </template>

      <!-- E: a quiet sheet, one sentence, one bar and a stop button beside it -->
      <template v-else-if="variant === 'E'">
        <div class="pe-line">
          <span class="pe-tile"><G :n="job.glyph" /></span>
          <span class="pe-say"><b>{{ state === 'confirm' ? t.ask[dialog] : name }}</b><span class="muted">{{ deleting ? t.sentence.delete : `${t.into} ${where.to}` }}</span></span>
        </div>
        <p v-if="state === 'confirm' && deleting" class="muted small">{{ t.trashHint }}</p>
        <template v-else-if="state !== 'confirm'">
          <Meter class="pe-meter" :dialog="dialog" :state="state" :pct="pct" :lang="lang" :t="t" :label="name">
            <template v-if="!finished" #lead><button class="pe-stop" :aria-label="t.stop" :title="t.stop"><G n="IconStop" /></button></template>
            <template #facts><div class="muted small glim-num">{{ finished ? resultLine : `${Math.round(pct)} % · ${facts}` }}</div></template>
          </Meter>
        </template>
      </template>

      <!-- G: the calm sheet to confirm, and a quiet bar that opens up into the graph across the whole window -->
      <template v-else-if="variant === 'G'">
        <template v-if="state === 'confirm'">
          <div class="pe-line">
            <span class="pe-tile"><G :n="job.glyph" /></span>
            <span class="pe-say"><b>{{ t.ask[dialog] }}</b><span class="muted">{{ deleting ? `${name} · ${t.trashHint}` : `${name} ${t.into} ${where.to}` }}</span></span>
          </div>
        </template>
        <template v-else>
          <div class="pd-line"><b>{{ name }}</b><span class="glim-num muted">{{ Math.round(pct) }} %</span></div>
          <div v-if="finished" class="small muted">{{ resultLine }}</div>
          <Meter stacked inline :dialog="dialog" :state="state" :pct="pct" :lang="lang" :t="t" :label="name">
            <div v-if="state === 'errors'" class="pb-failed"><span v-for="f in job.failed" :key="f" class="small"><span class="mono">{{ f }}</span> · {{ t.error }}</span></div>
            <Checkbox v-model="keep" class="pg-keep" :label="t.keep" />
          </Meter>
        </template>
      </template>

      <!-- F: a big share done, the route with flying files, and the graph with its numbers below when opened -->
      <template v-else>
        <template v-if="state === 'confirm'">
          <div class="pf-head"><span class="pf-big">{{ t.ask[dialog] }}</span></div>
          <Flow v-bind="flow" />
          <p v-if="deleting" class="muted small">{{ t.trashHint }}</p>
        </template>
        <template v-else>
          <div class="pf-head"><span class="pf-big glim-num">{{ Math.round(pct) }} % {{ t.doneOf }}</span><Badge :tone="tone" size="small">{{ t.badge[state as 'running'] }}</Badge></div>
          <div class="muted small">{{ finished ? resultLine : `${job.count} ${t.objects} · ${where.from} › ${where.to}` }}</div>
          <Meter stacked marker :dialog="dialog" :state="state" :pct="pct" :lang="lang" :t="t" :label="name">
            <template v-if="!finished" #lead><Flow v-bind="flow" /></template>
            <template v-if="!finished" #facts><div class="muted small glim-num">{{ facts }}</div></template>
          </Meter>
          <div v-if="state === 'errors'" class="pb-failed"><span v-for="f in job.failed" :key="f" class="small"><span class="mono">{{ f }}</span> · {{ t.error }}</span></div>
        </template>
      </template>
    </template>
  </Win>
</template>
