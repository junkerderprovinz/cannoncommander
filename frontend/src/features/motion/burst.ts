// What an element of the head turns into when the ball strikes it, made the way BombVault's logo bursts: the element
// is painted once into a picture of its own, the picture is cut into small splinters, and every splinter flies on one
// canvas over the whole window, thrown out from the strike and carried on by the ball, turning, falling and fading,
// with a spray of sparks where it was hit.
import { dial, dialMs } from '@/core/motion';

interface Point { x: number; y: number }

export interface Blast {
  /** How long a splinter flies, in seconds. */
  life: number;
  /** In pixels over that time: how far a splinter is thrown out from the strike, how far the ball carries it on and how far it falls. */
  spread: number;
  carry: number;
  fall: number;
  /** How far a splinter turns over its flight, in radians. */
  spin: number;
  /** How many splinters one element breaks into. */
  count: number;
  /** The pause between the splinters nearest the strike leaving and the farthest, in seconds. */
  lag: number;
}

export const blastOf = (): Blast => ({
  life: dialMs('--motion-shard-dur') / 1000,
  spread: dial('--motion-shard-spread'),
  carry: dial('--motion-shard-carry'),
  fall: dial('--motion-shard-fall'),
  spin: (dial('--motion-shard-spin') * Math.PI) / 180,
  count: dial('--motion-shard-count'),
  lag: (dialMs('--motion-stagger-step') * 3) / 1000,
});

export interface Splinter {
  /** The corners, in pixels of the element's box. */
  poly: [number, number][];
  /** Its middle, where it turns about. */
  x: number;
  y: number;
  /** In pixels a second. */
  vx: number;
  vy: number;
  /** In radians a second. */
  spin: number;
  /** How much larger it grows as it leaves. */
  grow: number;
  /** Seconds it stays in place before it leaves. */
  delay: number;
}

// Cuts a w by h box into about `count` triangles: a grid whose inner points are pushed a little out of line, and every
// cell split corner to corner one way or the other. The pieces fill the box exactly, so in the moment it breaks it
// still looks whole. Each one is thrown out from the strike, the nearest hardest and first, and the ball carries all
// of them on the way it flies.
export function splinters(w: number, h: number, at: Point, dir: number, b: Blast, rand = Math.random): Splinter[] {
  const cells = Math.max(1, Math.round(b.count / 2)), side = Math.max(2, Math.sqrt((w * h) / cells));
  const cols = Math.max(1, Math.round(w / side)), rows = Math.max(1, Math.round(h / side)), cw = w / cols, ch = h / rows;
  const nudge = (i: number, n: number) => (i > 0 && i < n ? i + (rand() - 0.5) * 0.44 : i);
  const grid = Array.from({ length: rows + 1 }, (_, j) => Array.from({ length: cols + 1 }, (_, i): [number, number] => [nudge(i, cols) * cw, nudge(j, rows) * ch]));
  const tris: [number, number][][] = [];
  for (let j = 0; j < rows; j++) {
    for (let i = 0; i < cols; i++) {
      const tl = grid[j]![i]!, tr = grid[j]![i + 1]!, br = grid[j + 1]![i + 1]!, bl = grid[j + 1]![i]!;
      tris.push(...(rand() < 0.5 ? [[tl, tr, br], [tl, br, bl]] : [[tl, tr, bl], [tr, br, bl]]));
    }
  }
  const far = Math.hypot(Math.max(at.x, w - at.x), Math.max(at.y, h - at.y)) || 1, speed = b.spread / b.life;
  return tris.map((poly) => {
    const x = (poly[0]![0] + poly[1]![0] + poly[2]![0]) / 3, y = (poly[0]![1] + poly[1]![1] + poly[2]![1]) / 3;
    const near = Math.min(1, Math.hypot(x - at.x, y - at.y) / far), a = Math.atan2(y - at.y, x - at.x) + (rand() - 0.5) * 0.9;
    const v = speed * (0.45 + rand() * 0.9) * (1.25 - near * 0.5);
    return {
      poly, x, y,
      vx: Math.cos(a) * v + (dir * b.carry * (0.35 + rand() * 0.8)) / b.life,
      vy: Math.sin(a) * v - speed * (0.15 + rand() * 0.45),
      spin: ((rand() * 2 - 1) * b.spin) / b.life,
      grow: 0.85 + rand() * 0.55,
      delay: near * b.lag,
    };
  });
}

export interface Picture { canvas: HTMLCanvasElement; scale: number }

const radius = (v: string, r: DOMRect) => (v.endsWith('%') ? (parseFloat(v) / 100) * Math.min(r.width, r.height) : parseFloat(v) || 0);

function box(g: CanvasRenderingContext2D, r: DOMRect, s: CSSStyleDeclaration) {
  const radii = [s.borderTopLeftRadius, s.borderTopRightRadius, s.borderBottomRightRadius, s.borderBottomLeftRadius]
    .map((v) => Math.min(radius(v, r), r.width / 2, r.height / 2));
  g.fillStyle = s.backgroundColor;
  g.beginPath();
  g.roundRect(r.left, r.top, r.width, r.height, radii);
  g.fill();
  const line = parseFloat(s.borderTopWidth) || 0;
  if (!line || s.borderTopStyle === 'none') return;
  g.strokeStyle = s.borderTopColor;
  g.lineWidth = line;
  g.beginPath();
  g.roundRect(r.left + line / 2, r.top + line / 2, r.width - line, r.height - line, radii.map((x) => Math.max(0, x - line / 2)));
  g.stroke();
}

function words(g: CanvasRenderingContext2D, node: Text, s: CSSStyleDeclaration) {
  const text = node.data.replace(/\s+/g, ' ').trim(), range = document.createRange();
  range.selectNodeContents(node);
  const r = range.getBoundingClientRect();
  g.font = `${s.fontStyle} ${s.fontWeight} ${s.fontSize} ${s.fontFamily}`;
  g.fillStyle = s.color;
  g.textBaseline = 'alphabetic';
  g.textAlign = 'left';
  const m = g.measureText(text);
  g.fillText(text, r.left, r.top + (r.height - m.fontBoundingBoxAscent - m.fontBoundingBoxDescent) / 2 + m.fontBoundingBoxAscent);
}

// A glyph goes in as an image of itself, which loads a moment later; the picture is painted well before the ball
// arrives, and a splinter shows whatever the picture holds when it is drawn.
function glyph(g: CanvasRenderingContext2D, svg: SVGSVGElement, s: CSSStyleDeclaration, alpha: number) {
  const r = svg.getBoundingClientRect(), copy = svg.cloneNode(true) as SVGSVGElement;
  copy.setAttribute('width', String(r.width));
  copy.setAttribute('height', String(r.height));
  copy.style.cssText = `color: ${s.color}; fill: ${s.fill}; stroke: ${s.stroke}; stroke-width: ${s.strokeWidth}`;
  const img = new Image();
  img.onload = () => {
    g.globalAlpha = alpha;
    g.drawImage(img, r.left, r.top, r.width, r.height);
  };
  img.src = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(new XMLSerializer().serializeToString(copy))}`;
}

function draw(g: CanvasRenderingContext2D, el: Element, alpha: number) {
  if (!el.getClientRects().length) return;
  const s = getComputedStyle(el), seen = s.visibility === 'visible';
  alpha *= parseFloat(s.opacity);
  if (!alpha) return;
  g.globalAlpha = alpha;
  if (seen) box(g, el.getBoundingClientRect(), s);
  if (el instanceof SVGSVGElement) {
    if (seen) glyph(g, el, s, alpha);
    return;
  }
  for (const node of el.childNodes) {
    if (node instanceof Element) draw(g, node, alpha);
    else if (seen && node instanceof Text && node.data.trim()) {
      g.globalAlpha = alpha;
      words(g, node, s);
    }
  }
}

const clear = (colour: string) => colour === 'transparent' || /[,/]\s*0\)$/.test(colour);

// Paints an element as it shows, on the ground of the bar it sits on: a menu title has no ground of its own, and
// without one most of its splinters would be empty. Shadows, pictures and pseudo elements stay out; a splinter is too
// small to miss them.
export function paint(el: Element): Picture | null {
  const r = el.getBoundingClientRect(), scale = Math.min(2, window.devicePixelRatio || 1);
  const canvas = document.createElement('canvas');
  canvas.width = Math.max(1, Math.ceil(r.width * scale));
  canvas.height = Math.max(1, Math.ceil(r.height * scale));
  const g = canvas.getContext('2d');
  if (!g) return null;
  g.scale(scale, scale);
  g.translate(-r.left, -r.top);
  let under = el.parentElement;
  while (under && clear(getComputedStyle(under).backgroundColor)) under = under.parentElement;
  if (under) {
    g.fillStyle = getComputedStyle(under).backgroundColor;
    g.fillRect(r.left, r.top, r.width, r.height);
  }
  draw(g, el, 1);
  return { canvas, scale };
}

interface Flight { fill: CanvasPattern; scale: number; left: number; top: number; pieces: Splinter[]; sparks: Spark[]; at: Point; start: number; b: Blast }
interface Spark { vx: number; vy: number; life: number; width: number }

// Sparks fly faster than the splinters and burn out long before them.
const sparksOf = (n: number, dir: number, b: Blast, rand: () => number): Spark[] => Array.from({ length: n }, () => {
  const a = rand() * Math.PI * 2, v = (b.spread / b.life) * (1 + rand() * 2.2);
  return { vx: Math.cos(a) * v + (dir * b.carry) / b.life, vy: Math.sin(a) * v, life: b.life * (0.15 + rand() * 0.35), width: 1.2 + rand() * 1.6 };
});

// One canvas over the whole window that every struck element throws its splinters onto. It draws only while
// something flies on it, and goes with the layer it lies in.
export function burst(into: Element) {
  const cv = document.createElement('canvas'), dpr = Math.min(2, window.devicePixelRatio || 1);
  cv.className = 'fx-burst';
  cv.width = Math.round(window.innerWidth * dpr);
  cv.height = Math.round(window.innerHeight * dpr);
  into.append(cv);
  const g = cv.getContext('2d');
  const flights: Flight[] = [];
  let raf = 0;

  const frame = (now: number) => {
    raf = 0;
    if (!g || !cv.isConnected) return;
    g.setTransform(1, 0, 0, 1, 0, 0);
    g.clearRect(0, 0, cv.width, cv.height);
    let flying = false;
    for (const f of flights) {
      const t0 = (now - f.start) / 1000, { life, fall } = f.b, drop = (2 * fall) / (life * life);
      if (t0 > life + f.b.lag) continue;
      flying = true;
      g.globalCompositeOperation = 'lighter';
      g.setTransform(dpr, 0, 0, dpr, 0, 0);
      for (const p of f.sparks) {
        const k = t0 / p.life;
        if (k >= 1) continue;
        const slow = 1 - k * 0.45, x = f.at.x + p.vx * t0 * slow, y = f.at.y + p.vy * t0 * slow + drop * 0.35 * t0 * t0;
        g.globalAlpha = 1 - k;
        g.strokeStyle = `rgb(255 ${Math.round(230 - 120 * k)} ${Math.round(140 - 130 * k)})`;
        g.lineWidth = p.width;
        g.beginPath();
        g.moveTo(x, y);
        g.lineTo(x - p.vx * 0.03, y - p.vy * 0.03);
        g.stroke();
      }
      g.globalCompositeOperation = 'source-over';
      g.fillStyle = f.fill;
      // The pattern lies in the picture's own pixels, so the corners are drawn there and the turn shrinks them back.
      const ps = f.scale;
      for (const p of f.pieces) {
        const t = Math.max(0, t0 - p.delay), k = t / life;
        if (k >= 1) continue;
        const grow = (1 + (p.grow - 1) * Math.min(1, t * 3)) * dpr, turn = p.spin * t;
        const c = Math.cos(turn) * grow, s = Math.sin(turn) * grow;
        const x = (f.left + p.x + p.vx * t) * dpr, y = (f.top + p.y + p.vy * t + (drop * t * t) / 2) * dpr;
        g.globalAlpha = k < 0.6 ? 1 : (1 - k) / 0.4;
        g.setTransform(c / ps, s / ps, -s / ps, c / ps, x - c * p.x + s * p.y, y - s * p.x - c * p.y);
        g.beginPath();
        for (const [px, py] of p.poly) g.lineTo(px * ps, py * ps);
        g.closePath();
        g.fill();
      }
    }
    g.globalAlpha = 1;
    if (flying) raf = requestAnimationFrame(frame);
  };

  return {
    canvas: cv,
    /** Breaks the painted element whose box starts at left, top, at the point `at` of the page. */
    hit(pic: Picture, left: number, top: number, at: Point, pieces: Splinter[], dir: number, b: Blast, rand = Math.random) {
      const fill = g?.createPattern(pic.canvas, 'no-repeat');
      if (!g || !fill) return;
      flights.push({ fill, scale: pic.scale, left, top, pieces, at, b, start: performance.now(), sparks: sparksOf(Math.round(pieces.length / 5), dir, b, rand) });
      if (!raf) raf = requestAnimationFrame(frame);
    },
    stop() {
      cancelAnimationFrame(raf);
      cv.remove();
    },
  };
}

export type Burst = ReturnType<typeof burst>;
