// The play state of a video or audio element, as the prototype's player draws it: time, length, playing.
import { onBeforeUnmount, reactive, type Ref, watch } from 'vue';
import { toast } from '@/core/toast';
import { t } from '@/i18n';

export function usePlayer(media: Ref<HTMLMediaElement | null>) {
  const st = reactive({ t: 0, dur: 0, playing: false, failed: false });
  let told = false;

  const sync = () => {
    const m = media.value;
    if (!m) return;
    st.t = m.currentTime;
    if (Number.isFinite(m.duration)) st.dur = m.duration;
  };
  const on: Record<string, () => void> = {
    timeupdate: sync,
    loadedmetadata: sync,
    durationchange: sync,
    play: () => { st.playing = true; },
    pause: () => { st.playing = false; },
    ended: () => { st.playing = false; },
    error: () => { st.failed = true; st.playing = false; },
  };
  watch(media, (m, old) => {
    for (const [ev, fn] of Object.entries(on)) {
      old?.removeEventListener(ev, fn);
      m?.addEventListener(ev, fn);
    }
  }, { immediate: true });

  // A format the browser cannot play says so once, when someone presses play, not while the poster shows.
  const cannot = () => {
    if (told) return;
    told = true;
    toast(t('viewer.media.cantPlay'), 'warn');
  };

  function toggle() {
    const m = media.value;
    if (!m) return;
    if (st.failed) return cannot();
    if (m.paused) {
      if (m.ended) m.currentTime = 0;
      m.play().catch((e: DOMException) => { if (e.name !== 'AbortError') { st.failed = true; cannot(); } });
    } else m.pause();
  }
  function seek(to: number) {
    const m = media.value;
    if (!m || !st.dur) return;
    m.currentTime = Math.max(0, Math.min(st.dur, to));
    sync();
  }
  // Pointer on the scrub bar: a press jumps there and dragging follows.
  function scrub(e: PointerEvent) {
    const bar = e.currentTarget as HTMLElement;
    const at = (ev: PointerEvent) => {
      const r = bar.getBoundingClientRect();
      seek(((ev.clientX - r.left) / r.width) * st.dur);
    };
    at(e);
    bar.setPointerCapture(e.pointerId);
    bar.onpointermove = at;
    bar.onpointerup = () => { bar.onpointermove = null; };
  }

  const keys = (e: KeyboardEvent, combo: string) => {
    if ((combo === ' ' && !(e.target as Element).closest?.('button, input, textarea')) || combo === 'k') return toggle(), true;
    if (combo === 'ArrowRight') return seek(st.t + 10), true;
    if (combo === 'ArrowLeft') return seek(st.t - 10), true;
    return undefined;
  };

  onBeforeUnmount(() => media.value?.pause());
  return { st, toggle, seek, scrub, keys };
}
