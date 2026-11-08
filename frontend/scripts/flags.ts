// node scripts/flags.ts writes src/i18n/flags.css anew: one rule per flag the language list shows, the drawing inlined.
// flag-icons has some 270 flags, and its own stylesheet would ship all of them for the 42 the list needs. Run it again
// after a language joins languages.ts.
import { readFileSync, writeFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { LANGUAGES } from '../src/i18n/languages.ts';

const root = resolve(import.meta.dirname, '..');
const drawings = join(root, 'node_modules', 'flag-icons', 'flags', '4x3');
const codes = [...new Set(LANGUAGES.map((l) => l.flag))];

const rules = codes.map((code) => {
  let svg: Buffer;
  try {
    svg = readFileSync(join(drawings, code + '.svg'));
  } catch {
    console.error(`no flag ${code}.svg, is flag-icons installed?`);
    process.exit(1);
  }
  return `.fi-${code} { background-image: url("data:image/svg+xml;base64,${svg.toString('base64')}"); }`;
});

const head = `/* Written by scripts/flags.ts from flag-icons (MIT), only the flags of the language list. */
.fi { display: inline-block; flex: none; width: 1.25em; height: 1em; background: center / contain no-repeat; }
`;
writeFileSync(join(root, 'src', 'i18n', 'flags.css'), head + rules.join('\n') + '\n');
console.log(`wrote ${codes.length} flags to src/i18n/flags.css`);
