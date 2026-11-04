// The bubble beside anything with a data-tip, placed below it or above when the window ends.
let bubble: (HTMLDivElement & { _for?: Element }) | null = null;
let tipOf: (el: HTMLElement) => string = (el) => el.dataset.tip || '';
export const setTipSource = (fn: (el: HTMLElement) => string) => { tipOf = fn; };

export function hideTip() {
  bubble?.remove();
  bubble = null;
}

function onOver(e: MouseEvent) {
  if (matchMedia('(hover: none)').matches) return;
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
}

export const installTips = () => document.addEventListener('mouseover', onOver);
