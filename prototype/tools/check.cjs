// node tools/check.cjs [steps.cjs] [page.html]: opens the built page (dist/cannoncommander.html by default), runs the steps,
// saves screenshots to shots/ and reports page errors.
const path = require('path');
const fs = require('fs');
const { chromium } = require('playwright');

const root = path.join(__dirname, '..');
const shots = path.join(root, 'shots');
fs.mkdirSync(shots, { recursive: true });
const stepsFile = process.argv[2];

(async () => {
  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: { width: 1600, height: 950 } });
  const errors = [];
  page.on('pageerror', e => errors.push(String(e).slice(0, 300)));
  page.on('console', m => { if (m.type() === 'error') errors.push(m.text().slice(0, 300)); });
  const html = process.argv[3] ? path.resolve(process.argv[3]) : path.join(root, 'dist', 'cannoncommander.html');
  await page.goto('file:///' + html.split(path.sep).join('/'));
  await page.waitForTimeout(800);
  const shot = async (name) => page.screenshot({ path: path.join(shots, name + '.png') });
  if (stepsFile) await require(path.resolve(stepsFile))(page, shot);
  else await shot('start');
  console.log(errors.length ? 'ERRORS:\n' + errors.join('\n') : 'no page errors');
  await browser.close();
})();
