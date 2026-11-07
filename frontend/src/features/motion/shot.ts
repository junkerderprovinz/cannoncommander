// The long press on the header logo: the cannon trembles harder and harder while its ball rolls back into the barrel,
// then it fires with a flash and a cloud of smoke, and the ball crosses the head and bursts everything it strikes.
// Then the head puts itself together again. Nothing of the app's state is touched, and whatever ends the shot early
// puts the head back at once, so it cannot be left broken.
import { dialMs, moving } from '@/core/motion';
import { quietTips } from '@/core/tips';
import { HOLD, holdPress } from './cues';
import { flyBall, gunSmoke, muzzleFlash, replay, shatter } from './fx';

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
  const logo = head.querySelector('.brand-logo'), cannon = logo?.querySelector('.brand-cannon'), rest = logo?.querySelector<SVGSVGElement>('.brand-ball');
  const ball = rest?.querySelector<SVGGraphicsElement>(':scope > g'), barrel = cannon?.firstElementChild;
  if (!moving() || !logo || !cannon || !rest || !ball || !barrel) return false;

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
    for (const el of [...hit, cannon, rest, logo, ...head.querySelectorAll('.fx-shattered, .fx-rebuild')]) el.classList.remove('fx-shattered', 'fx-rebuild', 'fx-tremble', 'fx-loading', 'fx-kick', 'fx-fired');
    rest.style.removeProperty('--fx-muzzle');
    rest.style.removeProperty('--fx-reach');
    quietTips(false);
    stop = null;
  };

  // A mirrored head mirrors the logo too, so the muzzle and the way out are on the left.
  const dir = getComputedStyle(head).direction === 'rtl' ? -1 : 1;
  const b = ball.getBoundingClientRect(), muzzle = barrel.getBoundingClientRect(), box = rest.getBoundingClientRect();
  const mouth = dir > 0 ? muzzle.right : muzzle.left;
  // The ball rolls in until it is gone, behind a cut at the muzzle; both are measured in the logo's own direction.
  rest.style.setProperty('--fx-muzzle', `${(((dir > 0 ? mouth - box.left : box.right - mouth) / box.width) * 100).toFixed(2)}%`);
  rest.style.setProperty('--fx-reach', Math.max(100, ((dir > 0 ? b.right - mouth : mouth - b.left) / b.width) * 100).toFixed(1));

  quietTips(true);
  cannon.classList.add('fx-tremble');
  rest.classList.add('fx-tremble', 'fx-loading');
  later(times.tremble, () => {
    cannon.classList.remove('fx-tremble');
    rest.classList.remove('fx-tremble', 'fx-loading');
    replay(cannon, 'fx-kick');
    logo.classList.add('fx-fired');

    const h = head.getBoundingClientRect(), y = b.top + b.height / 2;
    // The ball leaves from the muzzle, its middle where the flash is brightest.
    const from = mouth - b.width / 2, lead = dir > 0 ? from + b.width : from;
    // Far enough that the whole ball leaves the head before it fades.
    const end = dir > 0 ? h.right + b.width : h.left - b.width;
    pieces.push(flyBall(ball, from, end - (lead - from)), ...gunSmoke({ x: mouth, y }, b.height * 1.3, dir), muzzleFlash({ x: mouth, y }, b.height, dir));

    for (const el of targets(head)) {
      const r = el.getBoundingClientRect();
      const reach = Math.max(0, ((dir > 0 ? r.left : r.right) - lead) * dir) / Math.abs(end - lead);
      later(times.shot * reach, () => {
        if (!el.isConnected) return;
        pieces.push(...shatter(el, layer, dir, y));
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
      replay(rest, 'fx-rebuild');
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
