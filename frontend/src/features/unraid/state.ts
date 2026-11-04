// What the interface knows about the array: disks with their sleep, shares with their storage, the parity run and
// where the files of a folder lie. The server reports changes to sleep and parity as events, so the state is read
// once and then follows them.
import { defineStore } from 'pinia';
import { computed, reactive, ref } from 'vue';
import { on } from '@/api/sse';
import { type SpinEvent, unraid, type UnraidParity, type UnraidShare, type UnraidState, type UnraidWhere } from '@/api/unraid';
import { toast } from '@/core/toast';
import { split } from '@/files/paths';
import { t } from '@/i18n';
import { diskName } from '@/features/jobs/label';
import { useListing } from '@/stores/listing';

// Where a path sits on the array: the user shares, or a disk or pool seen directly.
export interface Place { vol: string; share: string; rel: string }

export const names = (devs: string[]) => devs.map(diskName).join(devs.length === 2 ? t('unraid.and') : ', ');

export const useUnraid = defineStore('unraid', () => {
  const st = ref<UnraidState | null>(null);
  const whereOf = reactive(new Map<string, UnraidWhere | null | undefined>());

  async function load() {
    try {
      st.value = await unraid.state();
    } catch {
      st.value = null;
    }
  }

  const available = computed(() => !!st.value?.available);
  const disks = computed(() => (st.value?.disks ?? []).filter((d) => d.kind === 'data' && d.mounted));
  const pools = computed(() => (st.value?.disks ?? []).filter((d) => d.kind === 'pool' && !d.pool).map((d) => d.name));
  const devices = computed(() => [...pools.value, ...disks.value.map((d) => d.name)]);
  const asleep = (dev: string) => !!disks.value.find((d) => d.name === dev)?.spundown;
  const share = (name: string): UnraidShare | undefined => st.value?.shares.find((s) => s.name === name);

  function place(path: string): Place | null {
    if (!available.value || !path || path.includes('::')) return null;
    const p = split(path);
    if (p[0] !== 'mnt' || !p[1] || !(p[1] === 'user' || devices.value.includes(p[1]))) return null;
    return { vol: p[1], share: p[2] || '', rel: p.slice(2).join('/') };
  }

  // The disks a listed entry lies on, from the listing's disk column; a path on a disk share is on that disk.
  function devicesOf(folder: string, name: string): string[] | undefined {
    const w = place(folder);
    if (w && w.vol !== 'user') return [w.vol];
    const v = useListing().metaOf(folder, name)?.disk;
    if (v === undefined) return undefined;
    return typeof v === 'string' && v ? v.split(', ') : [];
  }

  // Asked once per folder and again after it changed; a folder far down the shares can take a moment.
  function where(path: string): UnraidWhere | null | undefined {
    if (!whereOf.has(path)) {
      whereOf.set(path, undefined);
      unraid.where(path).then((got) => whereOf.set(path, got), () => whereOf.set(path, null));
    }
    return whereOf.get(path);
  }
  on('fs.changed', (e) => {
    const paths = (e.data as { paths?: string[] } | undefined)?.paths ?? [];
    for (const key of [...whereOf.keys()]) if (paths.some((p) => key === p || key.startsWith(p + '/') || p.startsWith(key + '/'))) whereOf.delete(key);
  });

  function setAsleep(devs: string[], sleep: boolean) {
    for (const d of disks.value) if (devs.includes(d.name)) d.spundown = sleep;
  }
  on('unraid.spin', (e) => {
    const ev = e.data as SpinEvent | undefined;
    if (!ev) return;
    for (const d of disks.value) d.spundown = ev.asleep.includes(d.name);
  });

  // Reading needs the disk; a sleeping one takes a few seconds to spin up, which is worth a word beforehand.
  function wake(devs: string[]) {
    const sleepy = [...new Set(devs)].filter(asleep);
    if (!sleepy.length) return;
    setAsleep(sleepy, false);
    toast(t(sleepy.length === 1 ? 'unraid.wakesOne' : 'unraid.wakesMany', { disks: names(sleepy) }), 'warn');
  }

  on('unraid.parity', (e) => {
    const p = e.data as UnraidParity | undefined;
    if (!p || !st.value) return;
    const was = st.value.parity.running;
    st.value.parity = p;
    if (was && !p.running && p.lastExit === 0) {
      toast(t('unraid.parityDone'));
    }
  });

  return { st, load, available, disks, pools, devices, asleep, share, place, devicesOf, where, wake, setAsleep };
});
