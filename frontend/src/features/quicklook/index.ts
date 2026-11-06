// Quick look on the space bar. The arrows walk through the folder while it is open, I adds the file's details
// beside the picture, Enter opens the file and Space or Escape close it again.
import './quicklook.css';
import { getActivePinia } from 'pinia';
import { createApp, nextTick, watch } from 'vue';
import { action } from '@/core/actions';
import { prefs, setPref } from '@/core/prefs';
import { tl } from '@/i18n';
import { usePanels } from '@/stores/panels';
import QuickLook from './QuickLook.vue';
import { look } from './state';

const focusList = () => document.querySelector<HTMLElement>('.panel.active .list')?.focus();

function step(d: number) {
  const F = usePanels(), tab = F.T(), n = F.entries().length;
  tab.cursor = Math.max(0, Math.min(n - 1, tab.cursor + d));
  nextTick(() => document.querySelector('.panel.active .row.cursor')?.scrollIntoView({ block: 'nearest' }));
}

// While the preview is up it owns the keyboard; everything else would act on the list behind it.
function onKey(e: KeyboardEvent) {
  if (!look.open) return;
  const F = usePanels();
  const steps: Record<string, number> = { ArrowLeft: -1, ArrowUp: -1, ArrowRight: 1, ArrowDown: 1 };
  const act = e.key === ' ' || e.key === 'Escape' ? () => { look.open = false; }
    : steps[e.key] ? () => step(steps[e.key]!)
      : e.key === 'Enter' ? () => { const k = F.cursorEntry(); look.open = false; if (k) F.open(F.active, k); }
        : e.key.toLowerCase() === 'i' ? () => setPref('quickInfo', !prefs.quickInfo) : null;
  if (!act) return;
  e.preventDefault();
  e.stopImmediatePropagation();
  act();
}

export default function setup() {
  action('quickLook', { label: tl('quicklook.title'), g: 'IconEye', key: ' ', when: () => !!usePanels().cursorEntry(), run: () => {
    if (usePanels().cursorEntry()) look.open = true;
  } });
  addEventListener('keydown', onKey, true);
  watch(() => look.open, (open) => { if (!open) focusList(); });
  // Its own small app on the body, sharing the store, so the shell's frame needs no slot for it.
  const host = document.body.appendChild(document.createElement('div'));
  const pinia = getActivePinia();
  const app = createApp(QuickLook);
  if (pinia) app.use(pinia);
  app.mount(host);
}
