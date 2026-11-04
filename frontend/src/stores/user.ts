// Who is signed in and their shortcuts.
import { defineStore } from 'pinia';
import { ref } from 'vue';
import { api } from '@/api/client';
import type { Bookmark, Me } from '@/api/types';

export const useUser = defineStore('user', () => {
  const me = ref<Me>({ name: '', full: '', role: '' });
  const bookmarks = ref<Bookmark[]>([]);

  // Parts the server does not offer yet stay empty, and the shell shows what it has.
  async function load() {
    const [m, b] = await Promise.allSettled([api.me(), api.bookmarks()]);
    if (m.status === 'fulfilled') me.value = m.value;
    if (b.status === 'fulfilled') bookmarks.value = b.value;
  }
  const admin = () => me.value.role.toLowerCase() === 'admin';

  return { me, bookmarks, load, admin };
});
