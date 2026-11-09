// The bubble beside anything with a data-tip, placed below it or above when the window ends.
import { dismissBubbleOn } from '@glimstone/core';

let bubble: (HTMLDivElement & { _for?: Element }) | null = null;
let tipOf: (el: HTMLElement) => string = (el) => el.dataset.tip || '';
export const setTipSource = (fn: (el: HTMLElement) => string) => { tipOf = fn; };

let undismiss: (() => void) | undefined;
export function hideTip() {
  bubble?.remove();
  bubble = null;
  undismiss?.();
  undismiss = undefined;
}

// While the header cannon fires, no bubble covers the shot or points at a control that lies in pieces.
let quiet = false;
export function quietTips(on: boolean) {
  quiet = on;
  if (on) hideTip();
}

function onOver(e: MouseEvent) {
  if (quiet || matchMedia('(hover: none)').matches) return;
  const tr = (e.target as Element).closest?.<HTMLElement>('[data-tip]');
  if (!tr) return hideTip();
  if (bubble && bubble._for === tr) return;
  hideTip();
  bubble = Object.assign(document.createElement('div'), { className: 'bubble', _for: tr });
  bubble.textContent = tipOf(tr);
  document.body.appendChild(bubble);
  const r = tr.getBoundingClientRect(), b = bubble.getBoundingClientRect();
  bubble.style.left = Math.max(8, Math.min(r.left + r.width / 2 - b.width / 2, innerWidth - b.width - 8)) + 'px';
  bubble.style.top = (r.bottom + b.height + 12 > innerHeight ? r.top - b.height - 8 : r.bottom + 8) + 'px';
  // A click or a key can change the view under a pointer that stays where it is, and then no element is entered
  // that would close the bubble, so those close it, as GlimStone's own bubble closes on a scroll.
  undismiss = dismissBubbleOn(hideTip);
}

function onOut(e: MouseEvent) {
  if (bubble?._for?.contains(e.target as Node) && !bubble._for.contains(e.relatedTarget as Node | null)) hideTip();
}

export function installTips() {
  document.addEventListener('mouseover', onOver);
  document.addEventListener('mouseout', onOut);
  for (const type of ['pointerdown', 'keydown', 'blur']) addEventListener(type, hideTip, true);
}
