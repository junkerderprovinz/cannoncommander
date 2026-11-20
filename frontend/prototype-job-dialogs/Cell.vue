<script setup lang="ts">
// Prototype: one window in one state, drawn the way the chosen variant would draw it.
import { Badge, Checkbox, Pill, ProgressBar, Toggle } from '@glimstone/vue';
import { computed, ref } from 'vue';
import { G } from '@/core/glyphs';
import { type Dialog, JOBS, type Lang, type State, texts } from './data';
import Win, { type FootButton } from './Win.vue';

const props = defineProps<{ variant: string; dialog: Dialog; state: State; lang: Lang }>();
const t = computed(() => texts(props.lang));
const job = computed(() => JOBS[props.dialog]);
const name = computed(() => (props.dialog === 'delete' ? (props.lang === 'de' ? '3 Objekte' : '3 items') : job.value.name));
const keep = ref(false);
const all = ref(false);
const open = ref(props.state === 'errors');
const deleting = computed(() => props.dialog === 'delete');
const pct = computed(() => ({ confirm: 0, running: job.value.run.pct, paused: job.value.run.pct, conflict: job.value.run.pct, done: 100, errors: 100, cancelled: Math.round(job.value.run.pct / 2) })[props.state]);
const tone = computed(() => ({ running: 'active', paused: 'warn', conflict: 'warn', done: 'ok', errors: 'fail', cancelled: 'neutral', confirm: 'neutral' })[props.state] as 'active');
const facts = computed(() => {
  const r = job.value.run;
  if (deleting.value) return `${r.done} · ${t.value.files}`;
  return [`${r.done} / ${job.value.total}`, props.state === 'running' ? r.speed : '', props.state === 'running' ? `${t.value.left} ${r.left}` : ''].filter(Boolean).join(' · ');
});
const resultLine = computed(() => ({ done: t.value.doneLine, errors: t.value.errorsLine, cancelled: t.value.cancelledLine } as Record<string, Record<Dialog, string>>)[props.state]?.[props.dialog] ?? '');
const crumbs = (path: string) => path.split('/').filter(Boolean).slice(2);
const finished = computed(() => ['done', 'errors', 'cancelled'].includes(props.state));

const foot = computed<FootButton[]>(() => {
  const x = t.value, A = props.variant === 'A';
  switch (props.state) {
    case 'confirm':
      return [{ label: x.cancel, g: 'IconClose' }, { label: deleting.value ? x.toTrash : x.dialog[props.dialog], g: job.value.glyph, primary: true }];
    case 'running':
      return A ? [{ label: x.background, g: 'IconWindowMinimise' }, { label: x.cancel, g: 'IconClose' }]
        : [{ label: x.background, g: 'IconWindowMinimise' }, { label: x.pause, g: 'IconPause' }, { label: x.cancel, g: 'IconClose' }];
    case 'paused':
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
    <template v-if="variant === 'B'" #start><Checkbox v-model="keep" :label="t.keep" /></template>
    <template #default>
    <!-- A: the windows as they are on the release branch -->
    <template v-if="variant === 'A'">
      <template v-if="state === 'confirm' && !deleting">
        <div class="pa-rows">
          <div class="pa-row"><span>{{ t.to }}</span><span class="pa-field mono">{{ job.to }}</span></div>
          <div class="pa-row"><span>{{ t.way }}</span><span class="pa-seg"><b class="on">{{ t.ways[dialog === 'move' ? 'move' : 'copy'] }}</b><b>{{ t.ways[dialog === 'move' ? 'copy' : 'move'] }}</b><b>{{ t.ways.link }}</b></span></div>
          <div class="pa-row"><span>{{ t.clash }}</span><span class="pa-seg"><b v-for="(r, i) in t.rules" :key="r" :class="{ on: i === 0 }">{{ r }}</b></span></div>
          <div class="pa-row muted"><span>{{ t.more }}</span><G n="IconExpand" /></div>
        </div>
        <div class="pa-chips"><Pill>{{ name }}</Pill><Pill class="glim-num">{{ job.total }}</Pill></div>
      </template>
      <template v-else-if="state === 'confirm'">
        <div class="pa-list"><div v-for="n in job.items" :key="n" class="pa-item"><G n="IconFile" />{{ n }}</div></div>
        <div class="pa-row"><span>{{ t.trashToggle }}</span><Toggle :model-value="true" :label="t.trashToggle" /></div>
        <p class="muted small">{{ t.trashHint }}</p>
      </template>
      <template v-else-if="state === 'conflict'">
        <div class="cf-name"><b>{{ job.run.current }}</b><Badge tone="fail" size="small">{{ t.taken }}</Badge></div>
        <div class="cf-side"><span class="muted">{{ t.newSide }}</span><Pill class="glim-num">2,1 MB</Pill><Pill class="glim-num">12.08.2026, 14:30</Pill></div>
        <div class="cf-side"><span class="muted">{{ t.oldSide }}</span><Pill class="glim-num">2,0 MB</Pill><Pill class="glim-num">12.08.2026, 14:30</Pill></div>
        <Checkbox v-model="all" :label="t.forAll" />
      </template>
      <template v-else>
        <div class="jf-head"><b>{{ name }}</b><Pill class="glim-num">{{ pct }} %</Pill></div>
        <ProgressBar :value="pct / 100" :label="name" :paused="state === 'paused'" :failed="state === 'errors'" />
        <div class="muted small glim-num">{{ finished ? resultLine : facts }}</div>
        <div v-if="state === 'running'" class="mono small">{{ job.run.current }}</div>
        <Checkbox v-model="keep" :label="t.keep" />
      </template>
    </template>

    <!-- B: one column, the route as one sentence, the checkbox in the foot -->
    <template v-else-if="variant === 'B'">
      <div class="pb-sentence"><G :n="job.glyph" /><span><b>{{ name }}</b> <span class="muted">{{ deleting ? t.sentence.delete : t.sentence[dialog] }}</span> <b v-if="!deleting">{{ crumbs(job.to).join(' › ') }}</b></span></div>
      <template v-if="state === 'confirm'">
        <button class="pb-options"><span>{{ t.options }}</span><span class="muted">{{ deleting ? t.trashHint : t.optionsLine }}</span><G n="IconExpand" /></button>
      </template>
      <template v-else-if="state === 'conflict'">
        <div class="pb-conflict"><b class="mono">{{ job.run.current }}</b><Badge tone="fail" size="small">{{ t.taken }}</Badge></div>
        <div class="pb-compare"><span>{{ t.newSide }} · 2,1 MB · 12.08.2026, 14:30</span><span>{{ t.oldSide }} · 2,0 MB · 12.08.2026, 14:30</span></div>
        <Checkbox v-model="all" :label="t.forAll" />
      </template>
      <template v-else>
        <div class="pb-big glim-num"><span>{{ pct }} %</span><Badge :tone="tone" size="small">{{ t.badge[state] }}</Badge></div>
        <ProgressBar :value="pct / 100" :label="name" :paused="state === 'paused'" :failed="state === 'errors'" />
        <div class="muted small glim-num">{{ finished ? resultLine : facts }}</div>
        <div v-if="state === 'errors'" class="pb-failed"><span v-for="f in job.failed" :key="f" class="mono small">{{ f }} · {{ t.error }}</span></div>
      </template>
    </template>

    <!-- C: two zones, where it goes on top and how far it is below -->
    <template v-else-if="variant === 'C'">
      <div class="pc-route">
        <div class="pc-end"><span class="muted small">{{ t.from }}</span><span class="mono">{{ job.from }}</span></div>
        <template v-if="!deleting"><G n="IconMoveDown" class="pc-arrow" /><div class="pc-end"><span class="muted small">{{ t.to }}</span><span class="mono">{{ job.to }}</span></div></template>
        <div class="pc-what"><G :n="job.glyph" /><b>{{ name }}</b><span class="muted glim-num">{{ job.total }}</span></div>
      </div>
      <template v-if="state === 'confirm'">
        <div class="pc-opts">
          <div class="pa-row"><span>{{ deleting ? t.trashToggle : t.clash }}</span><span v-if="!deleting" class="pa-seg"><b class="on">{{ t.rules[0] }}</b><b>{{ t.rules[1] }}</b><b>{{ t.rules[2] }}</b><b>{{ t.rules[4] }}</b></span><Toggle v-else :model-value="true" :label="t.trashToggle" /></div>
        </div>
      </template>
      <template v-else-if="state === 'conflict'">
        <div class="pc-progress">
          <div class="cf-name"><b class="mono">{{ job.run.current }}</b><Badge tone="fail" size="small">{{ t.taken }}</Badge></div>
          <div class="pc-stats"><div><span class="muted small">{{ t.newSide }}</span><b class="glim-num">2,1 MB</b></div><div><span class="muted small">{{ t.oldSide }}</span><b class="glim-num">2,0 MB</b></div><div><span class="muted small">{{ t.newSide }}</span><b class="glim-num">14:30</b></div><div><span class="muted small">{{ t.oldSide }}</span><b class="glim-num">14:30</b></div></div>
          <Checkbox v-model="all" :label="t.forAll" />
        </div>
      </template>
      <template v-else>
        <div class="pc-progress">
          <div class="jf-head"><Badge :tone="tone" size="small">{{ t.badge[state] }}</Badge><span class="muted small">{{ finished ? resultLine : '' }}</span></div>
          <ProgressBar :value="pct / 100" :label="name" :paused="state === 'paused'" :failed="state === 'errors'" />
          <div class="pc-stats">
            <div><span class="muted small">{{ t.progress }}</span><b class="glim-num">{{ pct }} %</b></div>
            <div><span class="muted small">{{ deleting ? t.files : t.size }}</span><b class="glim-num">{{ deleting ? job.run.files : `${job.run.done} / ${job.total}` }}</b></div>
            <div><span class="muted small">{{ t.speed }}</span><b class="glim-num">{{ state === 'running' && job.run.speed ? job.run.speed : '…' }}</b></div>
            <div><span class="muted small">{{ t.left }}</span><b class="glim-num">{{ state === 'running' && job.run.left ? job.run.left : '…' }}</b></div>
          </div>
          <div v-if="state === 'running'" class="pc-now"><span class="muted small">{{ t.now }}</span><span class="mono small">{{ job.run.current }}</span></div>
          <div v-if="state === 'errors'" class="pb-failed"><span v-for="f in job.failed" :key="f" class="mono small">{{ f }} · {{ t.error }}</span></div>
        </div>
      </template>
      <Checkbox v-model="keep" :label="t.keep" />
    </template>

    <!-- D: one line and a bar, everything else behind Details -->
    <template v-else>
      <div class="pd-line"><b>{{ state === 'confirm' ? t.ask[dialog] : name }}</b><span v-if="state !== 'confirm'" class="glim-num muted">{{ pct }} %</span></div>
      <template v-if="state === 'confirm'">
        <div class="pd-chip"><G :n="job.glyph" /><span>{{ name }}</span><template v-if="!deleting"><G n="IconForward" class="pd-sep" /><span class="mono">{{ crumbs(job.to).join(' › ') }}</span></template></div>
      </template>
      <template v-else-if="state === 'conflict'">
        <div class="cf-name"><span class="mono">{{ job.run.current }}</span><Badge tone="fail" size="small">{{ t.taken }}</Badge></div>
      </template>
      <ProgressBar v-if="state !== 'confirm' && state !== 'conflict'" :value="pct / 100" :label="name" :paused="state === 'paused'" :failed="state === 'errors'" />
      <div v-if="finished" class="small muted">{{ resultLine }}</div>
      <button class="pd-more" :aria-expanded="open" @click="open = !open"><G n="IconExpand" :class="{ turned: open }" />{{ state === 'confirm' ? t.options : t.details }}</button>
      <div v-if="open" class="pd-details">
        <template v-if="state === 'confirm'"><span class="muted small">{{ deleting ? t.trashHint : t.optionsLine }}</span></template>
        <template v-else-if="state === 'conflict'"><span class="small">{{ t.newSide }} · 2,1 MB · 14:30</span><span class="small">{{ t.oldSide }} · 2,0 MB · 14:30</span><Checkbox v-model="all" :label="t.forAll" /></template>
        <template v-else>
          <span class="small glim-num">{{ facts }}</span>
          <span class="mono small">{{ job.from }} → {{ job.to || '…' }}</span>
          <span v-for="f in state === 'errors' ? job.failed : []" :key="f" class="mono small">{{ f }} · {{ t.error }}</span>
          <Checkbox v-model="keep" :label="t.keep" />
        </template>
      </div>
    </template>
    </template>
  </Win>
</template>
