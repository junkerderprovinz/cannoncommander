// Krusader's own clipboard: names cut or copied from one folder, pasted into another as a copy or a move. In the desktop
// app the computer's clipboard holds them instead, so Explorer and the file managers of Linux paste them too and the
// panels paste what those copied.
import { defineStore } from 'pinia';
import { ref } from 'vue';

// paths names every item in full when they come from several folders, as a search or the flat view shows them.
export interface Clip { cut: boolean; from: string; items: string[]; paths?: string[] }

export const useClipboard = defineStore('clipboard', () => {
  const clip = ref<Clip | null>(null);
  // The count of files on the computer's clipboard, or null where the app does not reach it, as in the container.
  const system = ref<number | null>(null);
  // What the computer's clipboard held when clip got names it could not take, such as files inside an archive. Once
  // it holds something else, that came later and wins.
  const under = ref('');
  const put = (cut: boolean, from: string, items: string[], paths?: string[]) => { clip.value = { cut, from, items, paths }; };
  const clear = () => { clip.value = null; };
  const ready = () => !!clip.value || !!system.value;
  return { clip, system, under, put, clear, ready };
});
