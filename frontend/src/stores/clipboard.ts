// Krusader's own clipboard: names cut or copied from one folder, pasted into another as a copy or a move.
import { defineStore } from 'pinia';
import { ref } from 'vue';

export interface Clip { cut: boolean; from: string; items: string[] }

export const useClipboard = defineStore('clipboard', () => {
  const clip = ref<Clip | null>(null);
  const put = (cut: boolean, from: string, items: string[]) => { clip.value = { cut, from, items }; };
  const clear = () => { clip.value = null; };
  return { clip, put, clear };
});
