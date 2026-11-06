// The made up languages of the development server. en-XA puts accents on every letter and draws the words out by about
// two fifths, between brackets, so a cut end shows. ar-XB turns the text round to test right to left.
const ACCENTED: Record<string, string> = {
  a: 'á', b: 'ƀ', c: 'ç', d: 'ð', e: 'é', f: 'ƒ', g: 'ĝ', h: 'ĥ', i: 'î', j: 'ĵ', k: 'ķ', l: 'ļ', m: 'ɱ',
  n: 'ñ', o: 'ö', p: 'þ', q: 'ǫ', r: 'ŕ', s: 'š', t: 'ţ', u: 'û', v: 'ṽ', w: 'ŵ', x: 'ẋ', y: 'ý', z: 'ž',
  A: 'Å', B: 'Ɓ', C: 'Ç', D: 'Ð', E: 'É', F: 'Ƒ', G: 'Ĝ', H: 'Ĥ', I: 'Î', J: 'Ĵ', K: 'Ķ', L: 'Ļ', M: 'Ṁ',
  N: 'Ñ', O: 'Ö', P: 'Þ', Q: 'Ǫ', R: 'Ŕ', S: 'Š', T: 'Ţ', U: 'Û', V: 'Ṽ', W: 'Ŵ', X: 'Ẋ', Y: 'Ý', Z: 'Ž',
};

function stretch(part: string): string {
  const letters = [...part].filter((ch) => ACCENTED[ch]).length;
  const extra = Math.round(letters * 0.4);
  let seen = 0;
  return [...part].map((ch) => {
    const accented = ACCENTED[ch];
    if (!accented) return ch;
    // The extra letters spread evenly over the text, so every word grows and not only the last one.
    const twice = Math.floor(((seen + 1) * extra) / letters) > Math.floor((seen * extra) / letters);
    seen++;
    return twice ? accented + accented.toLowerCase() : accented;
  }).join('');
}

export function pseudo(text: string, code: string): string {
  if (code === 'ar-XB') return '‮' + text + '‬';
  return '[' + text.split(/(\{\w+\})/).map((part, i) => (i % 2 ? part : stretch(part))).join('') + ']';
}
