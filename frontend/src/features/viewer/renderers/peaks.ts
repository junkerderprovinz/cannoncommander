// The waveform of a song. Songs small enough to decode in the browser get their real loudness; larger ones and
// those the browser cannot decode get a steady made-up shape from their name, so the bar still works for seeking.
import { rawUrl } from '../content';
import { rng } from '../kinds';

const MAX_BYTES = 40 * 1024 * 1024;
const cache = new Map<string, Promise<number[] | null>>();

export function madeUpBars(name: string, count: number): number[] {
  const r = rng(name);
  return Array.from({ length: count }, (_, k) => 0.14 + Math.abs(Math.sin(k * 0.11 + r() * 0.6)) * 0.5 + Math.abs(Math.sin(k * 0.53)) * 0.16 + r() * 0.2);
}

export function bars(values: Float32Array, count: number): number[] {
  const step = Math.max(1, Math.floor(values.length / count));
  const out: number[] = [];
  for (let k = 0; k < count; k++) {
    let peak = 0;
    for (let i = k * step, end = Math.min(values.length, i + step); i < end; i += 16) peak = Math.max(peak, Math.abs(values[i]!));
    out.push(peak);
  }
  const top = Math.max(...out, 1e-6);
  return out.map((v) => 0.08 + (v / top) * 0.9);
}

export function realBars(path: string, size: number, count: number): Promise<number[] | null> {
  if (size > MAX_BYTES || typeof AudioContext === 'undefined') return Promise.resolve(null);
  const key = `${path}\n${count}`;
  if (!cache.has(key)) {
    cache.set(key, (async () => {
      const res = await fetch(rawUrl(path));
      if (!res.ok) return null;
      const ctx = new AudioContext();
      try {
        const audio = await ctx.decodeAudioData(await res.arrayBuffer());
        return bars(audio.getChannelData(0), count);
      } finally {
        ctx.close().catch(() => {});
      }
    })().catch(() => null));
  }
  return cache.get(key)!;
}
