// Motion that plays over the page rather than inside one component: a layer of its own above the panels, with
// nothing in it that takes a click. Every piece reads its numbers from the level's dials in motion.css and takes
// itself away when its animation ends, so a quick level and a slow one leave the page the same.
import { glyph } from '@/core/glyphs';
import { dial } from '@/core/motion';
import { type Blast, type Burst, burst, type Picture, splinters } from './burst';

type Vars = Record<string, string | number>;
interface Point { x: number; y: number }

const centre = (el: Element): Point => {
  const r = el.getBoundingClientRect();
  return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
};

let layer: HTMLElement | null = null;
function host() {
  if (!layer?.isConnected) {
    layer = document.createElement('div');
    layer.className = 'fx-layer';
    layer.setAttribute('aria-hidden', 'true');
    document.body.append(layer);
  }
  return layer;
}

const svg = (name: string) => {
  const g = glyph(name);
  return `<svg viewBox="${g.viewBox}" fill="currentColor">${g.svg}</svg>`;
};

// A tumbleweed and a cannonball have no glyph in GlimStone, so they are drawn here in its manner: filled shapes on
// the 14 unit grid, inked by the colour they stand in. The tumbleweed is a ring around three crossed loops of twig.
const TWIG = 'M1.8 7a5.2 2.5 0 1 0 10.4 0a5.2 2.5 0 1 0-10.4 0ZM3 7a4 1.35 0 1 0 8 0a4 1.35 0 1 0-8 0Z';
const WEED = `<svg viewBox="0 0 14 14" fill="currentColor" fill-rule="evenodd"><path d="M0.5 7a6.5 6.5 0 1 0 13 0a6.5 6.5 0 1 0-13 0ZM1.7 7a5.3 5.3 0 1 0 10.6 0a5.3 5.3 0 1 0-10.6 0Z"/>${
  [0, 60, 120].map((a) => `<path d="${TWIG}" transform="rotate(${a} 7 7)"/>`).join('')}</svg>`;
const BALL = `<svg viewBox="0 0 14 14"><circle cx="7" cy="7" r="6" fill="currentColor"/><circle class="fx-shine" cx="4.9" cy="4.9" r="1.5"/></svg>`;

function piece(cls: string, vars: Vars, html: string | Node = '', into: Element = host()) {
  const el = document.createElement('div');
  el.className = cls;
  for (const [k, v] of Object.entries(vars)) el.style.setProperty(k, typeof v === 'number' ? `${v}px` : v);
  if (typeof html === 'string') el.innerHTML = html;
  else el.append(html);
  const gone = (e: AnimationEvent) => { if (e.target === el) el.remove(); };
  el.addEventListener('animationend', gone);
  el.addEventListener('animationcancel', gone);
  into.append(el);
  return el;
}

// Plays a class on an element of the page once, again from the start when it is already playing.
export function replay(el: Element, cls: string) {
  el.classList.remove(cls);
  void (el as HTMLElement).offsetWidth;
  el.classList.add(cls);
  const done = (e: Event) => {
    if (e.target !== el) return;
    el.classList.remove(cls);
    el.removeEventListener('animationend', done);
  };
  el.addEventListener('animationend', done);
}

// The lights of a copy run side by side across the way, a volley of them in the storm.
export function trace(from: Element, to: Element) {
  const a = centre(from), b = centre(to), dx = b.x - a.x, dy = b.y - a.y, length = Math.hypot(dx, dy) || 1;
  const count = Math.max(1, Math.round(dial('--motion-trace-count'))), spread = dial('--motion-trace-spread');
  return Array.from({ length: count }, (_, i) => {
    const off = (i - (count - 1) / 2) * spread;
    return piece('fx-trace', { left: b.x - (dy / length) * off, top: b.y + (dx / length) * off, '--fx-dx': dx, '--fx-dy': dy, '--fx-angle': `${Math.atan2(dy, dx)}rad`, '--fx-i': String(i) });
  });
}

const puff = (at: Point, delay: string) =>
  [-1, 0, 1].map((side, i) => piece('fx-puff', { left: at.x, top: at.y, '--fx-side': String(side), '--fx-i': String(i), '--fx-delay': delay }, svg('IconCloud')));

export function cannon(from: Element, to: Element) {
  const a = centre(from), b = centre(to);
  piece('fx-cannon', { left: b.x, top: b.y, '--fx-dx': b.x - a.x, '--fx-dy': b.y - a.y }, `<div class="fx-ball">${BALL}</div>`);
  puff(b, 'var(--motion-cannon-dur)');
}

// Rows on their way to the trash, as many as the level allows: a hundred falling entries would only hide what is
// left. With no trash in sight they fall toward the bottom edge.
export function drop(rows: { el: Element; glyph: string; name: string }[], bin?: Element | null) {
  const target = bin && centre(bin);
  return rows.slice(0, Math.round(dial('--motion-drop-count'))).map(({ el, glyph: g, name }, i) => {
    const r = el.getBoundingClientRect();
    const label = document.createElement('span');
    label.textContent = name;
    return piece('fx-drop', { left: r.left, top: r.top, '--fx-dx': target ? target.x - r.left : 0, '--fx-dy': target ? target.y - r.top : 160, '--fx-i': String(i) }, svg(g) + label.outerHTML);
  });
}

export function smoke(from: Element) {
  const r = from.getBoundingClientRect();
  [0, 1, 2].forEach((i) => piece('fx-smoke', { left: r.left + r.width / 2, top: r.top, '--fx-i': String(i) }, svg('IconCloud')));
}

export function salute(bar: Element) {
  const r = bar.getBoundingClientRect(), n = Math.round(dial('--motion-salute-count'));
  for (let i = 0; i < n; i++) piece('fx-salute', { left: r.left + (r.width * (i + 0.5)) / n, top: r.top + r.height / 2, '--fx-i': String(i) }, svg('IconStar'));
}

// Sparks thrown out from the middle of an element, evenly round with a little scatter, so no two fly the same way.
export function sparks(from: Element, rand = Math.random) {
  const at = centre(from), n = Math.round(dial('--motion-spark-count'));
  return Array.from({ length: n }, (_, i) => {
    const angle = ((i + rand() * 0.6) / n) * 2 * Math.PI, reach = 0.6 + rand() * 0.4;
    return piece('fx-spark', { left: at.x, top: at.y, '--fx-sx': (Math.cos(angle) * reach).toFixed(3), '--fx-sy': (Math.sin(angle) * reach).toFixed(3) }, svg('IconStar'));
  });
}

export function tumble(box: Element) {
  const r = box.getBoundingClientRect();
  return piece('fx-tumble', { left: r.left, top: r.top, width: r.width, height: r.height }, `<div class="fx-tumble-run"><div class="fx-weed">${WEED}</div></div>`);
}

// The burst in front of the muzzle as the header's cannon fires, pointing the way the ball goes.
export const muzzleFlash = (at: Point, size: number, dir: number) =>
  piece('fx-flash', { left: at.x, top: at.y, '--fx-size': size, '--fx-turn': dir > 0 ? '0deg' : '180deg' });

// The cloud the shot leaves: soft puffs that start at the muzzle, swell, drift on after the ball and rise.
export function gunSmoke(at: Point, size: number, dir: number, rand = Math.random) {
  return [0, 1, 2, 3, 4].map((i) => piece('fx-gunsmoke', {
    left: at.x + dir * size * (0.2 + i * 0.3), top: at.y + (rand() - 0.5) * size * 0.4, '--fx-size': size * (0.8 + rand() * 0.5),
    '--fx-gx': String(dir * (0.4 + i * 0.25 + rand() * 0.4)), '--fx-gy': String(0.3 + rand() * 0.7), '--fx-i': String(i),
  }));
}

// The logo's own ball, its shapes copied as they are and cut to their own box, flying from the screen position `from`
// to `to` with a trail of fire behind it. Flying left it is mirrored, so it rolls the other way.
export function flyBall(ball: SVGGraphicsElement, from: number, to: number) {
  const r = ball.getBoundingClientRect(), box = ball.getBBox(), dir = Math.sign(to - from) || 1;
  const drawing = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  drawing.setAttribute('viewBox', `${box.x} ${box.y} ${box.width} ${box.height}`);
  drawing.append(ball.cloneNode(true));
  const shot = piece('fx-shot', { left: from, top: r.top, width: r.width, height: r.height, '--fx-dx': to - from, '--fx-flip': String(dir) }, drawing);
  const trail = document.createElement('div');
  trail.className = 'fx-trail';
  shot.prepend(trail);
  return shot;
}

// The splinters fly over the whole page, above everything on it, on a canvas in a layer of their own.
export function shardLayer() {
  const layer = document.createElement('div');
  layer.className = 'fx-shards';
  layer.setAttribute('aria-hidden', 'true');
  document.body.append(layer);
  return { layer, flight: burst(layer) };
}

// Breaks an element of the head where the ball strikes it, at the height `y` on the side it comes from: a short flash
// and a puff of dust there, and the picture of the element bursts into splinters that the blast throws out and the
// ball carries on. The element itself stays where it is; the caller hides it while its splinters are about.
export function shatter(el: Element, pic: Picture | null, flight: Burst, dir: number, y: number, b: Blast, rand = Math.random) {
  const r = el.getBoundingClientRect(), into = flight.canvas.parentElement!;
  const strike = { x: dir > 0 ? r.left : r.right, y: Math.min(r.bottom, Math.max(r.top, y)) };
  const size = Math.max(14, Math.min(r.height, 40) * 0.7);
  if (pic) flight.hit(pic, r.left, r.top, strike, splinters(r.width, r.height, { x: strike.x - r.left, y: strike.y - r.top }, dir, b, rand), dir, b, rand);
  const flash = piece('fx-impact', { left: strike.x, top: strike.y, '--fx-size': size }, '', into);
  const dust = [0, 1, 2, 3].map((i) => piece('fx-dust', {
    left: strike.x, top: strike.y, '--fx-size': size * (0.6 + rand() * 0.4), '--fx-i': String(i),
    '--fx-gx': String(dir * (0.3 + rand() * 0.8)), '--fx-gy': String(rand() * 1.2 - 0.8),
  }, '', into));
  return [flash, ...dust];
}
