// Round 2: columns, path line, glyph page and the change dots on each view.
module.exports = async (page, shot) => {
  const wait = (ms = 450) => page.waitForTimeout(ms);
  const dots = () => page.evaluate(() => [...document.querySelectorAll('.pwdot:not(.static)')].filter(d => d.getBoundingClientRect().width > 0).length);
  await wait(800);
  console.log('dots on files:', await dots());
  await page.click('.panel.active .lhead', { button: 'right' }); await wait(250); await shot('r2-colmenu');
  await page.keyboard.press('Escape');
  await page.evaluate(() => CC.setPref('columns', ['name', 'type', 'size', 'modified', 'owner', 'rights', 'dimensions']));
  await wait(); await shot('r2-columns');
  const h = await page.locator('.panel.active .lh[data-colid="size"]').boundingBox();
  await page.mouse.move(h.x + 10, h.y + 8); await page.mouse.down();
  await page.mouse.move(h.x - 140, h.y + 8, { steps: 8 }); await page.mouse.up(); await wait();
  console.log('order after drag:', await page.evaluate(() => CC.prefs.columns.join(',')));
  const bar = await page.locator('.panel.active .pathbar').boundingBox();
  await page.mouse.click(bar.x + bar.width - 20, bar.y + bar.height / 2); await wait(250); await shot('r2-pathedit');
  console.log('path editing:', await page.locator('.panel.active input.pathedit').count());
  await page.keyboard.press('Escape');
  await page.evaluate(() => CC.go('settings', 'look')); await wait(700);
  console.log('dots on settings:', await dots());
  await shot('r2-settings');
  await page.evaluate(() => CC.go('glyphs')); await wait(700);
  console.log('dots on glyphs:', await dots());
  await shot('r2-glyphs');
};
