import { win } from '@/core/windows';
import FolderPicker from './FolderPicker.vue';

// Resolves to the chosen path, or null when the window closes without one.
export function pickFolder({ title, start, exts = null }: { title: string; start: string; exts?: string[] | null }) {
  return new Promise<string | null>((resolve) => {
    let done = false;
    win({ title, cls: 'a-picker', body: FolderPicker, props: { start, exts, done: (p: string) => { done = true; resolve(p); } }, onClose: () => { if (!done) resolve(null); } });
  });
}
