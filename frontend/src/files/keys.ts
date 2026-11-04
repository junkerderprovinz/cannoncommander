// Keys of the file view itself, after windows, menus and the key map had their turn.
import { nextTick } from 'vue';
import { run } from '@/core/actions';
import { prefs } from '@/core/prefs';
import { usePanels } from '@/stores/panels';

export function filesKey(e: KeyboardEvent, combo: string) {
  const F = usePanels();
  const t = F.T(), list = F.entries();
  const move = (to: number) => {
    t.cursor = Math.max(0, Math.min(list.length - 1, to));
    nextTick(() => document.querySelector('.panel.active .row.cursor')?.scrollIntoView({ block: 'nearest' }));
  };
  const markStep = () => {
    const k = list[t.cursor];
    if (k && !k.up) t.marked.has(k.name) ? t.marked.delete(k.name) : t.marked.add(k.name);
    move(t.cursor + 1);
  };
  const keys: Record<string, () => unknown> = {
    Tab: () => { if (prefs.dual && !prefs.quickview) F.active = F.other(); },
    ArrowDown: () => move(t.cursor + 1), ArrowUp: () => move(t.cursor - 1), Home: () => move(0), End: () => move(list.length - 1),
    PageDown: () => move(t.cursor + 10), PageUp: () => move(t.cursor - 10),
    Enter: () => list[t.cursor] && F.open(F.active, list[t.cursor]!),
    Backspace: () => t.path !== '/' && F.open(F.active, { up: true }),
    Insert: markStep, ' ': () => run('quickLook'),
    Escape: () => {
      if (t.qf || t.filter) Object.assign(t, { qf: false, filter: '', cursor: 0 });
      else if (t.marked.size) t.marked = new Set();
    },
  };
  if (keys[combo]) {
    e.preventDefault();
    keys[combo]!();
  }
}
