// Round 18: the title bar, the pins on the shortcuts and the house colour picker with name suggestions.
module.exports = async (page, shot) => {
  const wait = (ms = 350) => page.waitForTimeout(ms);
  await page.setViewportSize({ width: 1908, height: 1000 });
  await wait(600);
  console.log('head:', await page.evaluate(() => { const h = $('.head').getBoundingClientRect(); return `${Math.round(h.width)}x${Math.round(h.height)}, magnifier ${!!$('.head [data-run="palette"]')}`; }));
  await shot('r18-1-head');
  await page.click('.head [data-run="palette"]'); await wait();
  console.log('palette open:', await page.evaluate(() => !!$('.pal-scrim')));
  await page.keyboard.press('Escape'); await wait();

  const pins = () => page.evaluate(() => DATA.bookmarks.map(b => b.name).join(', '));
  console.log('shortcuts:', await pins(), 'pins:', await page.locator('#shortcuts .unpin').count());
  const before = await page.evaluate(() => Files.T().path);
  await page.click('#shortcuts .unpin >> nth=2'); await wait();
  console.log('after unpin:', await pins(), '| path unchanged:', before === await page.evaluate(() => Files.T().path));
  await page.click('.toast button:has-text("Rückgängig")'); await wait();
  console.log('after undo:', await pins());

  await page.click('.panel.active [data-labelsmenu]'); await page.click('#menu .hedit >> nth=0'); await wait();
  await page.click('[data-colpick="2"]'); await wait();
  console.log('picker:', await page.evaluate(() => `${!!$('.cpick .cp-sv')} field`));
  await shot('r18-2-picker');
  await page.fill('.cp-hex', '#2bb8e0'); await page.click('.cp-ok'); await wait();
  console.log('row 2 colour:', await page.evaluate(() => $('[data-colpick="2"]').style.getPropertyValue('--c')));
  await page.click('[data-colpick="new"]'); await wait();
  await page.fill('.cp-hex', '#c06cf0'); await wait();
  await page.click('.cp-ok'); await wait();
  console.log('suggested name:', await page.inputValue('#mk-new'));
  await page.click('[data-colpick="new"]'); await page.fill('.cp-hex', '#a87a55'); await page.click('.cp-ok'); await wait();
  console.log('suggestion follows the swatch:', await page.inputValue('#mk-new'));
  await page.click('[data-coladd]'); await wait();
  await shot('r18-3-colours');
  await page.click('.win-foot .btn.primary'); await wait();
  console.log('colours:', await page.evaluate(() => LABEL_COLORS.map(c => `${c[1]} ${c[2]}`).join(', ')));
  await page.evaluate(() => document.documentElement.dataset.theme = 'light'); await wait();
  await shot('r18-4-light');
};
