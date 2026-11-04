// The steps that can be taken back: the server's file operations, and what a toast offered to undo here, such as a
// shortcut taken off the sidebar. Strg+Z takes the newest of both.
import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import { ops, type UndoStep } from '@/api/ops';
import { on } from '@/api/sse';
import { now } from '@/core/format';
import { useJobs } from './jobs';

export interface LocalStep { id: string; label: string; g: string; at: number; run: () => void }
export type Step = { at: number } & ({ server: UndoStep } | { local: LocalStep });

const KEEP = 30;
let seq = 0;

export const useUndo = defineStore('undo', () => {
  const steps = ref<UndoStep[]>([]);
  const locals = ref<LocalStep[]>([]);

  const list = computed<Step[]>(() => [
    ...steps.value.map((s) => ({ at: Date.parse(s.at) || 0, server: s })),
    ...locals.value.map((s) => ({ at: s.at, local: s })),
  ].sort((a, b) => b.at - a.at).slice(0, KEEP));

  async function load() {
    steps.value = await ops.undoSteps();
  }

  function push(label: string, run: () => void, g = 'IconUndo') {
    const step: LocalStep = { id: `local-${++seq}`, label, g, at: now(), run };
    locals.value = [step, ...locals.value].slice(0, KEEP);
    return step;
  }

  // A step goes once: from the list first, so a second press of the same toast button finds nothing to do.
  async function take(step: Step) {
    if ('local' in step) {
      if (!locals.value.includes(step.local)) return false;
      locals.value = locals.value.filter((s) => s !== step.local);
      step.local.run();
      return true;
    }
    steps.value = steps.value.filter((s) => s.id !== step.server.id);
    const got = await ops.undo(step.server.id).catch((e) => { load().catch(() => {}); throw e; });
    if (got.job) useJobs().upsert(got.job);
    return true;
  }
  const byId = (id: string) => list.value.find((s) => ('server' in s ? s.server.id : s.local.id) === id);

  on('undo.changed', () => { load().catch(() => {}); });
  on('resync', () => { load().catch(() => {}); });

  return { steps, locals, list, load, push, take, byId };
});
