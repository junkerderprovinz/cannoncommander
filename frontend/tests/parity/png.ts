// Just enough PNG for screenshots: 8-bit RGB or RGBA without interlacing, which is what Chromium writes.
import { deflateSync, inflateSync } from 'node:zlib';

export interface Image {
  width: number;
  height: number;
  data: Uint8Array;
}

const SIGNATURE = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);

export function decode(buf: Buffer): Image {
  if (!buf.subarray(0, 8).equals(SIGNATURE)) throw new Error('not a PNG');
  let width = 0, height = 0, channels = 0;
  const idat: Buffer[] = [];
  for (let at = 8; at < buf.length;) {
    const len = buf.readUInt32BE(at);
    const type = buf.toString('latin1', at + 4, at + 8);
    const body = buf.subarray(at + 8, at + 8 + len);
    if (type === 'IHDR') {
      width = body.readUInt32BE(0);
      height = body.readUInt32BE(4);
      const [depth, colour, , , interlace] = body.subarray(8, 13);
      channels = colour === 6 ? 4 : colour === 2 ? 3 : 0;
      if (depth !== 8 || !channels || interlace) throw new Error(`unsupported PNG: depth ${depth}, colour ${colour}, interlace ${interlace}`);
    } else if (type === 'IDAT') idat.push(body);
    else if (type === 'IEND') break;
    at += 12 + len;
  }
  const raw = inflateSync(Buffer.concat(idat));
  const stride = width * channels;
  const rows = new Uint8Array(stride * height);
  for (let y = 0; y < height; y++) {
    const filter = raw[y * (stride + 1)];
    const line = raw.subarray(y * (stride + 1) + 1, (y + 1) * (stride + 1));
    const out = rows.subarray(y * stride, (y + 1) * stride);
    const up = y ? rows.subarray((y - 1) * stride, y * stride) : new Uint8Array(stride);
    for (let x = 0; x < stride; x++) {
      const a = x >= channels ? out[x - channels] : 0, b = up[x], c = x >= channels ? up[x - channels] : 0;
      let p = line[x];
      if (filter === 1) p += a;
      else if (filter === 2) p += b;
      else if (filter === 3) p += (a + b) >> 1;
      else if (filter === 4) {
        const pa = Math.abs(b - c), pb = Math.abs(a - c), pc = Math.abs(a + b - 2 * c);
        p += pa <= pb && pa <= pc ? a : pb <= pc ? b : c;
      }
      out[x] = p;
    }
  }
  if (channels === 4) return { width, height, data: rows };
  const data = new Uint8Array(width * height * 4);
  for (let i = 0, j = 0; i < rows.length; i += 3, j += 4) {
    data[j] = rows[i];
    data[j + 1] = rows[i + 1];
    data[j + 2] = rows[i + 2];
    data[j + 3] = 255;
  }
  return { width, height, data };
}

const CRC = Array.from({ length: 256 }, (_, n) => {
  let c = n;
  for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
  return c >>> 0;
});

function chunk(type: string, body: Buffer): Buffer {
  const head = Buffer.alloc(8);
  head.writeUInt32BE(body.length, 0);
  head.write(type, 4, 'latin1');
  let c = 0xffffffff;
  for (const b of Buffer.concat([head.subarray(4), body])) c = CRC[(c ^ b) & 0xff] ^ (c >>> 8);
  const crc = Buffer.alloc(4);
  crc.writeUInt32BE((c ^ 0xffffffff) >>> 0, 0);
  return Buffer.concat([head, body, crc]);
}

export function encode({ width, height, data }: Image): Buffer {
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(width, 0);
  ihdr.writeUInt32BE(height, 4);
  ihdr.set([8, 6, 0, 0, 0], 8);
  const raw = Buffer.alloc((width * 4 + 1) * height);
  for (let y = 0; y < height; y++) raw.set(data.subarray(y * width * 4, (y + 1) * width * 4), y * (width * 4 + 1) + 1);
  return Buffer.concat([SIGNATURE, chunk('IHDR', ihdr), chunk('IDAT', deflateSync(raw)), chunk('IEND', Buffer.alloc(0))]);
}

// pixelmatch's way of comparing: the colour difference in YIQ space, weighted the way the eye weighs brightness
// against hue, and pixels that only differ by anti-aliasing at an edge do not count. threshold 0.1 is its default.
const blend = (c: number, a: number) => 255 + ((c - 255) * a) / 255;

function colourDelta(p: Uint8Array, q: Uint8Array, k: number, m: number, yOnly = false): number {
  const r1 = blend(p[k]!, p[k + 3]!), g1 = blend(p[k + 1]!, p[k + 3]!), b1 = blend(p[k + 2]!, p[k + 3]!);
  const r2 = blend(q[m]!, q[m + 3]!), g2 = blend(q[m + 1]!, q[m + 3]!), b2 = blend(q[m + 2]!, q[m + 3]!);
  const y1 = r1 * 0.29889531 + g1 * 0.58662247 + b1 * 0.11448223;
  const y2 = r2 * 0.29889531 + g2 * 0.58662247 + b2 * 0.11448223;
  const y = y1 - y2;
  if (yOnly) return y;
  const i = (r1 - r2) * 0.59597799 - (g1 - g2) * 0.2741761 - (b1 - b2) * 0.32180189;
  const qq = (r1 - r2) * 0.21147017 - (g1 - g2) * 0.52261711 + (b1 - b2) * 0.31114694;
  const delta = 0.5053 * y * y + 0.299 * i * i + 0.1957 * qq * qq;
  return y1 > y2 ? -delta : delta;
}

function around(x: number, y: number, w: number, h: number) {
  return { x0: Math.max(x - 1, 0), y0: Math.max(y - 1, 0), x2: Math.min(x + 1, w - 1), y2: Math.min(y + 1, h - 1) };
}

function manySiblings(img: Uint8Array, x1: number, y1: number, w: number, h: number): boolean {
  const { x0, y0, x2, y2 } = around(x1, y1, w, h);
  const pos = (y1 * w + x1) * 4;
  let zeroes = x1 === x0 || x1 === x2 || y1 === y0 || y1 === y2 ? 1 : 0;
  for (let x = x0; x <= x2; x++) {
    for (let y = y0; y <= y2; y++) {
      if (x === x1 && y === y1) continue;
      const o = (y * w + x) * 4;
      if (img[pos] === img[o] && img[pos + 1] === img[o + 1] && img[pos + 2] === img[o + 2] && img[pos + 3] === img[o + 3]) zeroes++;
      if (zeroes > 2) return true;
    }
  }
  return false;
}

function antialiased(img: Uint8Array, x1: number, y1: number, w: number, h: number, other: Uint8Array): boolean {
  const { x0, y0, x2, y2 } = around(x1, y1, w, h);
  const pos = (y1 * w + x1) * 4;
  let zeroes = x1 === x0 || x1 === x2 || y1 === y0 || y1 === y2 ? 1 : 0;
  let min = 0, max = 0, minX = 0, minY = 0, maxX = 0, maxY = 0;
  for (let x = x0; x <= x2; x++) {
    for (let y = y0; y <= y2; y++) {
      if (x === x1 && y === y1) continue;
      const delta = colourDelta(img, img, pos, (y * w + x) * 4, true);
      if (delta === 0) {
        if (++zeroes > 2) return false;
      } else if (delta < min) {
        min = delta; minX = x; minY = y;
      } else if (delta > max) {
        max = delta; maxX = x; maxY = y;
      }
    }
  }
  if (min === 0 || max === 0) return false;
  return (manySiblings(img, minX, minY, w, h) && manySiblings(other, minX, minY, w, h))
    || (manySiblings(img, maxX, maxY, w, h) && manySiblings(other, maxX, maxY, w, h));
}

export function diff(a: Image, b: Image, threshold = 0.1): { pixels: number; total: number; image: Image } {
  const { width, height } = a;
  const out = new Uint8Array(width * height * 4);
  if (b.width !== width || b.height !== height) {
    out.fill(255);
    return { pixels: width * height, total: width * height, image: { width, height, data: out } };
  }
  const max = 35215 * threshold * threshold;
  let pixels = 0;
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      const o = (y * width + x) * 4;
      const delta = colourDelta(a.data, b.data, o, o);
      if (Math.abs(delta) > max) {
        const edge = antialiased(a.data, x, y, width, height, b.data) || antialiased(b.data, x, y, width, height, a.data);
        if (edge) out.set([255, 255, 0, 255], o);
        else {
          pixels++;
          out.set([255, 0, 0, 255], o);
        }
      } else {
        const grey = 255 + (0.299 * a.data[o]! + 0.587 * a.data[o + 1]! + 0.114 * a.data[o + 2]! - 255) * 0.1;
        out.set([grey, grey, grey, 255], o);
      }
    }
  }
  return { pixels, total: width * height, image: { width, height, data: out } };
}
