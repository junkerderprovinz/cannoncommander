// The hex view's rows and a first guess at what a file is from its opening bytes.
export function hexRows(bytes: Uint8Array) {
  const rows: { o: string; h: string; a: string }[] = [];
  for (let o = 0; o < bytes.length; o += 16) {
    const b = [...bytes.subarray(o, o + 16)];
    rows.push({
      o: o.toString(16).padStart(8, '0'),
      h: b.map((x) => x.toString(16).padStart(2, '0')).join(' '),
      a: b.map((x) => (x >= 32 && x < 127 ? String.fromCharCode(x) : '.')).join(''),
    });
  }
  return rows;
}

const MAGIC: [string, number[]][] = [
  ['sqlite', [...'SQLite format 3'].map((c) => c.charCodeAt(0))],
  ['program', [0x7f, 0x45, 0x4c, 0x46]],
  ['program', [0x4d, 0x5a]],
];

export function kindOfBytes(bytes: Uint8Array): 'sqlite' | 'program' | 'binary' {
  for (const [kind, sig] of MAGIC) if (sig.every((x, i) => bytes[i] === x)) return kind as 'sqlite' | 'program';
  return 'binary';
}
