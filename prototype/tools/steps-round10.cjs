// Round 10: section badges on the sidebar cards, a quiet quick look with details on I.
module.exports = async (page, shot) => {
  const wait = (ms = 350) => page.waitForTimeout(ms);
  await wait(800);
  console.log('sidebar badges:', (await page.locator('.side .notch').allTextContents()).join(' | '));
  await shot('r10-main');
  await page.evaluate(() => { Files.go('/mnt/user/downloads', 0); Files.active = 0; const t = Files.T(); t.cursor = Files.entries().findIndex(k => k.name === 'album.flac'); CC.render(); $('.panel.active .list').focus(); });
  await page.keyboard.press(' '); await wait(500);
  console.log('buttons in quick look head:', await page.locator('.ql > .btn, .ql .notch .btn').count(), 'details visible:', await page.locator('.ql .cv-details:visible').count());
  await shot('r10-ql');
  await page.keyboard.press('i'); await wait(300);
  console.log('after I, details visible:', await page.locator('.ql .cv-details:visible').count());
  await shot('r10-ql-info');
  await page.keyboard.press('ArrowRight'); await wait(300);
  await shot('r10-ql-video');
  await page.keyboard.press('i'); await page.keyboard.press('Escape'); await wait(200);
  console.log('closed:', await page.locator('.ql').count());
};
