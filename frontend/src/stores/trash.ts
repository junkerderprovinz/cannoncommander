// What waits in the trash. trash.changed only says that something changed, so the list is read again.
import { defineStore } from 'pinia';
import { ref } from 'vue';
import { ops, type TrashItem } from '@/api/ops';
import { on } from '@/api/sse';
import { useJobs } from './jobs';

export const useTrash = defineStore('trash', () => {
  const items = ref<TrashItem[]>([]);
  const days = ref(30);
  const bytes = ref(0);
  const system = ref(false);

  async function load() {
    const got = await ops.trash();
    items.value = got.items;
    days.value = got.days;
    bytes.value = got.bytes;
    system.value = !!got.system;
  }
  // Restoring and deleting for good run as jobs, so they report like every other job.
  const restore = async (list: TrashItem[]) => useJobs().adopt(await ops.restore(list.map((e) => e.id)));
  const purge = async (list: TrashItem[]) => useJobs().adopt(await ops.emptyTrash(list.map((e) => e.id)));
  const empty = async () => useJobs().adopt(await ops.emptyTrash());

  on('trash.changed', () => { load().catch(() => {}); });
  on('resync', () => { load().catch(() => {}); });

  return { items, days, bytes, system, load, restore, purge, empty };
});
