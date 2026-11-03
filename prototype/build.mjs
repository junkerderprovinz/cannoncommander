// node build.mjs [round] [out]: bundles src/ into one HTML file (dist/cannoncommander.html by default) with tokens, glyphs and the Pinnwand.
import { readFileSync, readdirSync, writeFileSync, existsSync, mkdirSync } from 'fs';
import { join } from 'path';

const here = import.meta.dirname;
const round = Number(process.argv[2] || 1);
const read = (...p) => readFileSync(join(here, ...p), 'utf8');

const jsDir = join(here, 'src', 'js');
const featureDir = join(jsDir, 'features');
const core = readdirSync(jsDir).filter(f => f.endsWith('.js')).sort();
const features = existsSync(featureDir) ? readdirSync(featureDir).filter(f => f.endsWith('.js')).sort() : [];
const boot = core.filter(f => f.startsWith('99-'));
const ordered = [...core.filter(f => !f.startsWith('99-')).map(f => join('src', 'js', f)), ...features.map(f => join('src', 'js', 'features', f)), ...boot.map(f => join('src', 'js', f))];
const js = ordered.map(f => `// ${f.replaceAll('\\', '/')}\n${read(f)}`).join('\n');

const glyphs = Object.fromEntries([...JSON.parse(read('vendor', 'glyphs.json')), ...JSON.parse(read('src', 'glyphs-extra.json'))].map(gl => [gl.name, { viewBox: gl.viewBox, svg: gl.svg.replace(/\s+/g, ' ') }]));
const formats = JSON.parse(read('src', 'formats.json'));
const candidates = ['IconFile', 'IconFileText', 'IconFileImage', 'IconFileVideo', 'IconFileAudio', 'IconFileSubtitles', 'IconCode', 'IconConfig', 'IconLog',
  'IconDatabase', 'IconArchive', 'IconKey', 'IconMail', 'IconExternalLink', 'IconLink', 'IconManual', 'IconApp', 'IconVM', 'IconContainers',
  'IconShieldOn', 'IconCompressMax', 'IconDownload', 'IconDashboard', 'IconBrowser', 'IconFlash', 'IconStar',
  ...JSON.parse(read('src', 'glyphs-extra.json')).map(gl => gl.name)];
const used = new Set([...(js.match(/Icon[A-Z][A-Za-z]+/g) || []), ...formats.map(f => f.glyph), ...candidates]);
const missing = [...used].filter(n => !glyphs[n]);
if (missing.length) throw new Error('unknown glyphs: ' + missing.join(', '));

// The app is one page with several views, so the Pinnwand's change dots need two things the template
// assumes away: a dot whose target is not on screen stays hidden, and "Hingehen" switches the view.
const patch = (src, from, to) => {
  if (!src.includes(from)) throw new Error('Pinnwand template changed, anchor missing: ' + from.slice(0, 60));
  return src.replace(from, to);
};
// The review board lives outside this repository; without it the pages build plain.
const pinnwandPath = process.env.PINNWAND || join(here, '..', '..', '_templates', 'pinnwand', 'pinnwand.js');
let pinnwand = '';
if (existsSync(pinnwandPath)) {
  pinnwand = readFileSync(pinnwandPath, 'utf8')
    .replace("'kl-redesign-notes'", "'cannoncommander-notes'")
    .replace(/^ {2}const PAGES = \{.*$/m, "  const PAGES = { '': 'CannonCommander', 'cannoncommander.html': 'CannonCommander' };")
    .replaceAll('kl-redesign-notizen', 'cannoncommander-notizen')
    .replaceAll('Notizen zum KnightLoader-Redesign', 'Notizen zu CannonCommander');
  pinnwand = patch(pinnwand, "|| (c.find && findText(c.find)) || document.querySelector('main') || document.body;", '|| (c.find && findText(c.find)) || null;');
  pinnwand = patch(pinnwand, 'if (seen.has(i) || c.route !== routeNow()) return;', 'if (seen.has(i) || c.route !== routeNow() || !changeTarget(c)) return;');
  pinnwand = patch(pinnwand, 'function jumpChange(i) {\n    const c = CHANGES[i];', "function jumpChange(i) {\n    const c = CHANGES[i];\n    if (c.go && window.PW_GO) { window.PW_GO(c.go); setTimeout(() => { changeTarget(c)?.scrollIntoView({ block: 'center', behavior: 'smooth' }); setTimeout(drawPins, 450); }, 350); return; }");
}
const board = (setup, code) => pinnwand ? `<script>${setup}</script>\n<script>\n${code}\n</script>` : '';
const changes = existsSync(join(here, 'src', 'changes.json')) ? JSON.parse(read('src', 'changes.json')) : [];
if (pinnwand && round > 1 && !changes.length) throw new Error('round ' + round + ' needs src/changes.json');

const html = read('src', 'index.html')
  .replace('/*TOKENS*/', () => read('vendor', 'tokens.css'))
  .replace('/*APPCSS*/', () => read('src', 'css', 'app.css'))
  .replace('/*GLYPHS*/', () => JSON.stringify(Object.fromEntries([...used].map(n => [n, glyphs[n]]))))
  .replace('/*FORMATS*/', () => JSON.stringify(formats))
  .replace('/*APP*/', () => js)
  .replace('<!--PINNWAND-->', () => board(`window.PW_ROUND = ${round}; window.PW_CHANGES = ${JSON.stringify(changes.map(c => ({ route: '/cannoncommander.html', ...c })))};`, pinnwand));
mkdirSync(join(here, 'dist'), { recursive: true });
const out = process.argv[3] || join('dist', 'cannoncommander.html');
writeFileSync(join(here, out), html);

// The format list as a page of its own: every format group, the glyph it gets and a choice of others.
const glyphPage = read('src', 'glyph-page.html')
  .replace('/*TOKENS*/', () => read('vendor', 'tokens.css'))
  .replace('/*GLYPHS*/', () => JSON.stringify(Object.fromEntries(candidates.concat(formats.map(f => f.glyph)).map(n => [n, glyphs[n]]))))
  .replace('/*FORMATS*/', () => JSON.stringify(formats))
  .replace('/*CANDIDATES*/', () => JSON.stringify(candidates))
  .replace('<!--PINNWAND-->', () => board(`window.PW_ROUND = ${round};`, pinnwand.replaceAll('cannoncommander-notizen', 'cannoncommander-glyphen-notizen').replace("'cannoncommander-notes'", "'cannoncommander-glyphen-notes'").replace("'cannoncommander.html': 'CannonCommander'", "'cannoncommander-glyphen.html': 'Glyphen'")));
writeFileSync(join(here, 'dist', 'cannoncommander-glyphen.html'), glyphPage);
console.log(`${out}:${(html.length / 1024).toFixed(0)} KB, ${ordered.length} scripts, ${used.size} glyphs, round ${round}`);

// Design previews: single pages with the same tokens, their own glyphs and their own notes.
const previewDir = join(here, 'previews');
for (const file of existsSync(previewDir) ? readdirSync(previewDir).filter(f => f.endsWith('.html')) : []) {
  const name = file.replace(/\.html$/, ''), src = read('previews', file);
  const names = [...new Set(src.match(/Icon[A-Z][A-Za-z]+/g) || [])];
  const unknown = names.filter(n => !glyphs[n]);
  if (unknown.length) throw new Error(`${file}: unknown glyphs ${unknown.join(', ')}`);
  const notes = pinnwand.replaceAll('cannoncommander-notizen', `cannoncommander-${name}-notizen`).replace("'cannoncommander-notes'", `'cannoncommander-${name}-notes'`).replace("'cannoncommander.html': 'CannonCommander'", `'cannoncommander-${name}.html': 'CannonCommander ${name}'`);
  const page = src
    .replace('/*TOKENS*/', () => read('vendor', 'tokens.css'))
    .replace('/*GLYPHS*/', () => JSON.stringify(Object.fromEntries(names.map(n => [n, glyphs[n]]))))
    .replace('<!--PINNWAND-->', () => board(`window.PW_ROUND = ${round};`, notes));
  writeFileSync(join(here, 'dist', `cannoncommander-${name}.html`), page);
  console.log(`dist/cannoncommander-${name}.html: ${(page.length / 1024).toFixed(0)} KB`);
}
