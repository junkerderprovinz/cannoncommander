import { touchSel } from '@/files/touch';
import { usePanels } from '@/stores/panels';

export function start(panel: number, idx: number) {
  const F = usePanels();
  F.active = panel;
  const tab = F.T(), k = F.entries()[idx];
  Object.assign(touchSel, { on: true, path: tab.path });
  if (k && !k.up) tab.marked.add(k.name);
  if (k) tab.cursor = idx;
  navigator.vibrate?.(12);
}

export function stop() {
  touchSel.on = false;
  usePanels().T().marked = new Set();
}

export function toggle(panel: number, idx: number) {
  const F = usePanels();
  F.active = panel;
  const tab = F.T(), k = F.entries()[idx];
  if (!k || k.up) return;
  if (tab.marked.has(k.name)) tab.marked.delete(k.name);
  else tab.marked.add(k.name);
  tab.cursor = idx;
}
