// Round 12: the panel search walks into subfolders and understands *.mkv, square pause button.
module.exports = async (page, shot) => {
  const wait = (ms = 350) => page.waitForTimeout(ms);
  await wait(800);
  await page.evaluate(() => { Files.go('/mnt/user', 0); Files.active = 0; CC.render(); $('.panel.active .list').focus(); });
  await page.keyboard.press('Control+f'); await wait();
  await page.keyboard.type('*.mkv'); await wait(400);
  const hits = await page.evaluate(() => Files.entries().map(k => (k.at || '') + '/' + k.name));
  console.log('hits for *.mkv:', hits.length, hits.slice(0, 6).join(', '));
  await shot('r12-search-mkv');
  await page.click('.panel.active [data-qfdeep]'); await wait();
  console.log('only here:', await page.evaluate(() => Files.entries().length), 'input focused:', await page.evaluate(() => document.activeElement?.id));
  await page.click('.panel.active [data-qfdeep]'); await wait();
  console.log('copy blocked:', await page.evaluate(() => { const t = Files.T(); t.cursor = 0; return CC.blocked(CC.actions.copy); }));
  await page.keyboard.press('Enter'); await page.keyboard.press('Enter'); await wait(500);
  console.log('after Enter: path', await page.evaluate(() => Files.T().path), 'window:', await page.locator('.scrim .win .notch').first().textContent().catch(() => 'none'));
  await page.evaluate(() => { while (winStack.length) CC.closeWin(); });
  await page.evaluate(() => { Files.go('/mnt/user/downloads', 0); const t = Files.T(); t.cursor = Files.entries().findIndex(k => k.name === 'album.flac'); CC.render(); $('.panel.active .list').focus(); });
  await page.keyboard.press(' '); await wait(800);
  console.log('pause button size:', await page.evaluate(() => { const b = $('.ql .cv-playbtn'); const r = b.getBoundingClientRect(); return Math.round(r.width) + 'x' + Math.round(r.height); }));
  await shot('r12-pause');
  await page.keyboard.press('Escape');
};
