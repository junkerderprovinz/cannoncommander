// The speed each job had along its way, for the graph of a job window. One sample per half percent is enough for a
// line across a window, and two hundred of them cover a whole job.
import { reactive } from 'vue';

export interface Sample { pct: number; rate: number; at: number }

const STEP = 0.5;
const MAX = 200;
const traces = reactive(new Map<string, Sample[]>());

export function record(id: string, pct: number, rate: number, at: number) {
  if (pct <= 0 || rate <= 0) return;
  const list = traces.get(id) ?? [];
  const last = list.at(-1);
  if (last && pct < last.pct) return;
  if (last && pct - last.pct < STEP) list[list.length - 1] = { pct: last.pct, rate, at };
  else list.push({ pct, rate, at });
  if (list.length > MAX) list.splice(0, list.length - MAX);
  traces.set(id, list);
}

export const samplesOf = (id: string): Sample[] => traces.get(id) ?? [];
export const forget = (id: string) => { traces.delete(id); };
