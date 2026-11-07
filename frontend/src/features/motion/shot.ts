// The long press on the header logo: the cannon trembles harder and harder, fires, and its ball crosses the head and
// breaks everything it passes. Then the head puts itself together again. Nothing of the app's state is touched, and
// whatever ends the shot early puts the head back at once, so it cannot be left broken.
import { dialMs, moving } from '@/core/motion';
import { quietTips } from '@/core/tips';
import { HOLD, holdPress } from './cues';
import { flyBall, muzzleSmoke, replay, shatter } from './fx';

export interface Times { tremble: number; shot: number; wait: number; rebuild: number; step: number }

export const timesOf = (): Times => ({
  tremble: dialMs('--motion-tremble-dur'),
  shot: dialMs('--motion-shot-dur'),
  wait: dialMs('--motion-rebuild-wait'),
  rebuild: dialMs('--motion-rebuild-dur'),
  step: dialMs('--motion-stagger-step'),
});

const shown = (el: Element) => el.getClientRects().length > 0;

// The menu bar breaks title by title, everything else as one piece.
const targets = (head: Element) => [...head.children]
  .flatMap((el) => (el.matches('.brand, .spacer, .fx-shards') ? [] : el.matches('.menubar') ? [...el.querySelectorAll('.glim-menubar-item')] : [el]))
  .filter(shown);

let stop: (() => void) | null = null;

export const stopShot = () => stop?.();

export function fire(head: HTMLElement, times = timesOf()): boolean {
  if (stop) return true;
  const logo = head.querySelector('.brand-logo'), cannon = logo?.querySelector('.brand-cannon'), ball = logo?.querySelector<SVGGraphicsElement>('.brand-ball > g');
  if (!moving() || !logo || !cannon || !ball) return false;

  const timers: ReturnType<typeof setTimeout>[] = [], pieces: Element[] = [], hit: Element[] = [];
  const later = (ms: number, fn: () => void) => timers.push(setTimeout(fn, ms));
  const layer = document.createElement('div');
  layer.className = 'fx-shards';
  layer.setAttribute('aria-hidden', 'true');
  layer.inert = true;
  head.append(layer);

  stop = () => {
    timers.forEach(clearTimeout);
    for (const el of [layer, ...pieces]) el.remove();
    for (const el of [...hit, cannon, logo, ...head.querySelectorAll('.fx-shattered, .fx-rebuild')]) el.classList.remove('fx-shattered', 'fx-rebuild', 'fx-tremble', 'fx-kick', 'fx-fired');
    quietTips(false);
    stop = null;
  };

  quietTips(true);
  cannon.classList.add('fx-tremble');
  later(times.tremble, () => {
    cannon.classList.remove('fx-tremble');
    replay(cannon, 'fx-kick');
    logo.classList.add('fx-fired');

    const dir = getComputedStyle(head).direction === 'rtl' ? -1 : 1;
    const b = ball.getBoundingClientRect(), h = head.getBoundingClientRect(), muzzle = cannon.firstElementChild!.getBoundingClientRect();
    const lead = dir > 0 ? b.right : b.left;
    // Far enough that the whole ball leaves the head before it fades.
    const end = dir > 0 ? h.right + b.width : h.left - b.width;
    pieces.push(flyBall(ball, end - (lead - b.left)), ...muzzleSmoke({ x: dir > 0 ? muzzle.right : muzzle.left, y: muzzle.top + muzzle.height / 2 }));

    for (const el of targets(head)) {
      const r = el.getBoundingClientRect();
      const reach = Math.max(0, ((dir > 0 ? r.left : r.right) - lead) * dir) / Math.abs(end - lead);
      later(times.shot * reach, () => {
        if (!el.isConnected) return;
        pieces.push(...shatter(el, layer, dir));
        el.classList.add('fx-shattered');
        hit.push(el);
      });
    }

    later(times.shot + times.wait, () => {
      hit.forEach((el, i) => later(i * times.step, () => {
        el.classList.remove('fx-shattered');
        replay(el, 'fx-rebuild');
      }));
      logo.classList.remove('fx-fired');
      replay(logo.querySelector('.brand-ball')!, 'fx-rebuild');
      later(times.rebuild + hit.length * times.step, () => stop?.());
    });
  });
  return true;
}

// Listens on the window, so the head's own button keeps its click for going to the files.
export function armLogo(on: Window = window, times?: Times) {
  const logoOf = (e: Event) => ((e.target as Element | null)?.closest?.('.head .brand') ?? null) as HTMLElement | null;
  const press = holdPress(HOLD, () => {
    const head = document.querySelector<HTMLElement>('.head');
    return !!head && fire(head, times);
  });
  const listeners: [string, (e: Event) => void][] = [
    ['pointerdown', (e) => {
      const p = e as PointerEvent, logo = logoOf(e);
      if (!logo || !p.isPrimary || p.button !== 0) return;
      // A finger keeps its pointer on the element it touched, which would hide it sliding off the logo.
      if (logo.hasPointerCapture?.(p.pointerId)) logo.releasePointerCapture(p.pointerId);
      press.down();
    }],
    ['pointerup', (e) => {
      press.up();
      // Released elsewhere, no click follows that could take the shot's place.
      if (!logoOf(e)) press.click();
    }],
    ['pointercancel', press.cancel],
    ['pointerleave', (e) => { if ((e.target as Element).matches?.('.head .brand')) press.cancel(); }],
    ['contextmenu', (e) => { if (logoOf(e) && press.held()) e.preventDefault(); }],
    ['click', (e) => {
      if (!logoOf(e) || press.click()) return;
      e.preventDefault();
      e.stopImmediatePropagation();
    }],
    ['resize', stopShot],
    ['pagehide', stopShot],
  ];
  for (const [type, fn] of listeners) on.addEventListener(type, fn, true);
  return () => {
    for (const [type, fn] of listeners) on.removeEventListener(type, fn, true);
    press.cancel();
  };
}
