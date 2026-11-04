// Pictures, films and music made by ffmpeg from the metadata the prototype shows, so thumbnails, players and
// the media columns have something real to read. Without ffmpeg every call returns null.
import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

let available;
export function hasFfmpeg() {
  if (available === undefined) {
    try {
      execFileSync('ffmpeg', ['-hide_banner', '-version'], { stdio: 'ignore' });
      available = true;
    } catch {
      available = false;
    }
  }
  return available;
}

// The container has two CPUs and 2 GB, while ffmpeg sizes its threads by the cores it sees on the host.
function run(args, ext) {
  const dir = mkdtempSync(join(tmpdir(), 'cc-media-'));
  const out = join(dir, 'out.' + ext);
  try {
    execFileSync('ffmpeg', ['-hide_banner', '-loglevel', 'error', '-y', ...args, '-threads', '2', out], { stdio: ['ignore', 'ignore', 'inherit'] });
    return readFileSync(out);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

const seedOf = (s) => {
  let h = 2166136261;
  for (const c of s) h = Math.imul(h ^ c.codePointAt(0), 16777619);
  return h >>> 0;
};

export function dimsOf(text, fallback = [640, 360]) {
  const m = /(\d+)\s*[×x]\s*(\d+)/.exec(text || '');
  return m ? [+m[1], +m[2]] : fallback;
}

export function secondsOf(text) {
  const parts = (text || '').split(':').map(Number);
  if (!parts.length || parts.some(Number.isNaN)) return 0;
  return parts.reduce((a, p) => a * 60 + p, 0);
}

const metadata = (cols) => Object.entries({
  title: cols.title, artist: cols.artist, album: cols.album, album_artist: cols.albumartist,
  track: cols.track, date: cols.year, genre: cols.genre, composer: cols.composer,
}).filter(([, v]) => v).flatMap(([k, v]) => ['-metadata', `${k}=${v}`]);

// A JPEG at the listed dimensions, as small as it gets; quality drops until it fits under the listed size.
export function image(name, [w, h], maxBytes) {
  if (!hasFfmpeg()) return null;
  const seed = seedOf(name);
  for (const [q, scale] of [[4, 1], [12, 1], [24, 1], [31, 2], [31, 4]]) {
    const buf = run(['-f', 'lavfi', '-i', `gradients=s=${Math.round(w / scale)}x${Math.round(h / scale)}:seed=${seed % 100000}:n=4:type=spiral`,
      '-frames:v', '1', '-vf', `hue=h=${seed % 360}`, '-q:v', String(q)], 'jpg');
    if (!maxBytes || buf.length <= maxBytes) return buf;
  }
  return null;
}

// A film with the listed dimensions and length and one frame every four seconds, so even a long one encodes quickly.
export function video(name, [w, h], seconds, cols) {
  if (!hasFfmpeg() || !seconds) return null;
  const seed = seedOf(name);
  return run(['-f', 'lavfi', '-i', `testsrc2=s=${w}x${h}:r=1/4`, '-f', 'lavfi', '-i', `sine=f=${220 + (seed % 440)}:r=8000`,
    '-t', String(seconds), '-vf', `hue=h=${seed % 360}`, '-c:v', 'libx264', '-preset', 'ultrafast', '-x264-params', 'threads=2:lookahead-threads=1', '-tune', 'stillimage', '-g', '150',
    '-c:a', 'libopus', '-b:a', '6k', '-ac', '1', ...metadata(cols), '-shortest'], 'mkv');
}

export function audio(name, ext, seconds, cols) {
  if (!hasFfmpeg() || !seconds) return null;
  const codec = { mp3: ['-c:a', 'libmp3lame', '-b:a', '32k'], flac: ['-c:a', 'flac', '-sample_fmt', 's16'], ogg: ['-c:a', 'libopus', '-b:a', '16k'], wav: ['-c:a', 'pcm_s16le'] }[ext];
  if (!codec) return null;
  return run(['-f', 'lavfi', '-i', `sine=f=${220 + (seedOf(name) % 440)}:r=8000`, '-t', String(seconds), '-ac', '1', ...codec, ...metadata(cols)], ext);
}

// Bytes after the last MP3 frame make players guess the length from the bitrate, so an MP3 grows inside its ID3v2
// tag instead: the tag's padding is zeros by definition and the frames move to the end of the file.
export function id3Padded(buf, size) {
  if (buf.toString('latin1', 0, 3) !== 'ID3' || buf[5] & 0x10) return null;
  const tagEnd = 10 + ((buf[6] << 21) | (buf[7] << 14) | (buf[8] << 7) | buf[9]);
  const frames = buf.subarray(tagEnd);
  const body = size - frames.length - 10;
  if (body < tagEnd - 10 || body >= 1 << 28) return null;
  const head = Buffer.from(buf.subarray(0, tagEnd));
  head.set([(body >> 21) & 0x7f, (body >> 14) & 0x7f, (body >> 7) & 0x7f, body & 0x7f], 6);
  return { length: size, chunks: [{ offset: 0, buf: head }, { offset: size - frames.length, buf: frames }] };
}

// Matroska allows a Void element anywhere, so a film grows to its listed size and players skip the rest.
export function matroskaVoid(gap) {
  if (gap < 9) return null;
  const h = Buffer.alloc(9);
  h[0] = 0xec;
  h[1] = 0x01;
  h.writeUIntBE(gap - 9, 3, 6);
  return h;
}
