// Auswahlmodus: a long press on a row starts it, every row gets a box to tick, and a bar at the bottom carries the
// actions. Going to another folder ends it, as a phone's gallery does.
import './touch.css';
import { getActivePinia } from 'pinia';
import { createApp, watch } from 'vue';
import { action } from '@/core/actions';
import { isMenuOpen } from '@/core/menu';
import { go as goView, view } from '@/core/views';
import { winStack } from '@/core/windows';
import { touchSel } from '@/files/touch';
import { tl } from '@/i18n';
import { usePanels } from '@/stores/panels';
import { start, stop, toggle } from './state';
import TouchBar from './TouchBar.vue';

const HOLD = 500;

export default function setup() {
  const F = usePanels();
  let press: { x: number; y: number; timer: ReturnType<typeof setTimeout> } | null = null, swallow = false;
  const rowAt = (e: Event) => (view.id === 'files' ? (e.target as Element | null)?.closest?.<HTMLElement>('.panel[data-panel] .list [data-idx]') : null);
  const where = (row: HTMLElement) => [Number(row.closest<HTMLElement>('.panel')!.dataset.panel), Number(row.dataset.idx)] as const;

  document.addEventListener('pointerdown', (e) => {
    swallow = false;
    const row = e.pointerType === 'touch' ? rowAt(e) : null;
    if (!row) return;
    const [panel, idx] = where(row);
    if (press) clearTimeout(press.timer);
    press = { x: e.clientX, y: e.clientY, timer: setTimeout(() => {
      press = null;
      swallow = true;
      if (touchSel.on) toggle(panel, idx);
      else start(panel, idx);
    }, HOLD) };
  });
  const drop = () => { if (press) { clearTimeout(press.timer); press = null; } };
  document.addEventListener('pointermove', (e) => { if (press && Math.hypot(e.clientX - press.x, e.clientY - press.y) > 10) drop(); });
  document.addEventListener('pointerup', drop);
  document.addEventListener('pointercancel', drop);
  // The browser answers a long press with its own context menu, a drag or a click; the selection has taken it over.
  document.addEventListener('contextmenu', (e) => { if (press || swallow) { e.preventDefault(); e.stopImmediatePropagation(); } }, true);
  document.addEventListener('dragstart', (e) => { if (press || touchSel.on) { e.preventDefault(); e.stopImmediatePropagation(); } }, true);
  document.addEventListener('click', (e) => {
    if (swallow) { swallow = false; if (rowAt(e)) { e.preventDefault(); e.stopImmediatePropagation(); return; } }
    const row = touchSel.on ? rowAt(e) : null;
    if (!row) return;
    e.preventDefault();
    e.stopImmediatePropagation();
    toggle(...where(row));
  }, true);
  document.addEventListener('keydown', (e) => {
    if (touchSel.on && e.key === 'Escape' && !winStack.length && !isMenuOpen()) { e.preventDefault(); e.stopImmediatePropagation(); stop(); }
  }, true);

  watch(() => F.T().path, (path) => { if (touchSel.on && path !== touchSel.path) touchSel.on = false; });
  watch(() => touchSel.on, (on) => document.getElementById('app')?.classList.toggle('touch-sel', on));

  action('touchSelect', { label: tl('touch.title'), g: 'IconSelectAll', checked: () => touchSel.on, run: () => {
    if (touchSel.on) return stop();
    if (view.id !== 'files') goView('files');
    start(F.active, F.T().cursor);
  } });

  const host = document.body.appendChild(document.createElement('div'));
  const pinia = getActivePinia();
  const app = createApp(TouchBar);
  if (pinia) app.use(pinia);
  app.mount(host);
}
