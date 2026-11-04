// Who is signed in, their shortcuts and what waits in the bin.
import { defineStore } from 'pinia';
import { ref } from 'vue';
import { api } from '@/api/client';
import type { Bookmark, Me, TrashItem } from '@/api/types';

export const useUser = defineStore('user', () => {
  const me = ref<Me>({ name: '', full: '', role: '' });
  const bookmarks = ref<Bookmark[]>([]);
  const trash = ref<TrashItem[]>([]);

  // Parts the server does not offer yet stay empty, and the shell shows what it has.
  async function load() {
    const [m, b, tr] = await Promise.allSettled([api.me(), api.bookmarks(), api.trash()]);
    if (m.status === 'fulfilled') me.value = m.value;
    if (b.status === 'fulfilled') bookmarks.value = b.value;
    if (tr.status === 'fulfilled') trash.value = tr.value;
  }

  return { me, bookmarks, trash, load };
});
