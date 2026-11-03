// Round 7, the file list: receding inactive panel, calm marking, resizable columns, tiles, path menu, tab tools, phone, status bar.
module.exports = async (page, shot) => {
  const wait = (ms = 350) => page.waitForTimeout(ms);
  await wait(800);
  await page.evaluate(() => { Files.go('/mnt/user/downloads', 0); Files.active = 0; const t = Files.T(); t.marked = new Set(['album.flac', 'setup-notes.md']); CC.render(); });
  await wait(); await shot('r7-marked');

  const grip = page.locator('.panel.active .lh-grip[data-grip="size"]');
  const box = await grip.boundingBox();
  await page.mouse.move(box.x + 3, box.y + 5); await page.mouse.down();
  await page.mouse.move(box.x + 83, box.y + 5, { steps: 5 }); await page.mouse.up();
  await wait();
  console.log('size width after drag:', await page.evaluate(() => CC.prefs.colWidths.size), 'sort still name:', await page.evaluate(() => Files.T().sort));
  await page.dblclick('.panel.active .lh-grip[data-grip="modified"]'); await wait();
  console.log('modified width after double click:', await page.evaluate(() => CC.prefs.colWidths.modified));
  await shot('r7-resized');
  await page.evaluate(() => Columns.reset());

  await page.evaluate(() => { Files.go('/mnt/user/fotos/2026', 0); CC.run('view-tiles'); const t = Files.T(); t.marked = new Set(['IMG_0203.jpg']); CC.render(); });
  await wait(); await shot('r7-tiles');
  await page.evaluate(() => { Files.go('/mnt/user/downloads', 0); CC.render(); });
  await wait(); await shot('r7-tiles-mixed');
  await page.evaluate(() => CC.run('view-details'));

  await page.click('.panel.active .psep[data-psub="/mnt/user"]'); await wait();
  console.log('path menu:', (await page.locator('#menu button').allTextContents()).map(s => s.trim()).join(' | '));
  await shot('r7-pathmenu');
  await page.click('#menu >> text=musik'); await wait();
  console.log('after path menu:', await page.evaluate(() => Files.T().path));

  await page.click('.panel.active [data-tab="0"]', { button: 'right' }); await wait();
  console.log('tab menu:', (await page.locator('#menu button').allTextContents()).map(s => s.trim()).join(' | '));
  await page.click('#menu >> text=Tab sperren'); await wait();
  await page.keyboard.press('Escape');
  const before = await page.evaluate(() => Files.P().tabs.length);
  await page.evaluate(() => Files.go('/mnt/user/fotos'));
  console.log('locked tab opened new tab:', await page.evaluate(() => Files.P().tabs.length) - before, 'locked stays in:', await page.evaluate(() => Files.P().tabs[0].path));
  await shot('r7-locked');
  await page.click('.panel.active [data-tab="1"]', { button: 'middle' }); await wait();
  console.log('after middle click:', await page.evaluate(() => Files.P().tabs.length));
  await page.evaluate(() => { Files.P().tabs[0].locked = false; CC.render(); });

  await page.click('.panel.active .lfree'); await wait(500);
  console.log('disk usage title:', await page.locator('.scrim .win .notch').first().textContent().catch(() => 'none'));
  await shot('r7-du');
  await page.evaluate(() => { while (winStack.length) CC.closeWin(); });

  await page.setViewportSize({ width: 390, height: 844 });
  await page.evaluate(() => { Files.go('/mnt/user/fotos/2026', 0); Files.active = 0; CC.render(); });
  await wait(500); await shot('r7-phone');
  await page.click('.panel.active .pmore'); await wait();
  await shot('r7-phone-path');
  await page.click('.panel.active .pdots i:not(.on)'); await wait();
  console.log('phone active after dot:', await page.evaluate(() => Files.active));
  await page.setViewportSize({ width: 1600, height: 950 });
  await page.evaluate(() => { Files.active = 1; CC.render(); });
  await wait(); await shot('r7-active-right');
};
