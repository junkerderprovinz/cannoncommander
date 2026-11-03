// Round 8: seven views, clearing the selection three ways, the panels and the sidebar on one line, Verpacken.
module.exports = async (page, shot) => {
  const wait = (ms = 350) => page.waitForTimeout(ms);
  await wait(800);
  const tops = await page.evaluate(() => [$('.side').getBoundingClientRect().top, $('.panel').getBoundingClientRect().top]);
  console.log('side and panel top:', tops.map(Math.round).join(' / '));

  for (const v of ['details', 'compact', 'list', 'content', 'tiles', 'large', 'gallery']) {
    await page.evaluate((id) => { Files.go(id === 'gallery' || id === 'large' ? '/mnt/user/fotos/2026' : '/mnt/user/downloads', 0); Files.active = 0; CC.run('view-' + id); const t = Files.T(); t.cursor = 1; t.marked = new Set([Files.entries()[2].name]); CC.render(); }, v);
    await wait(); await shot('r8-view-' + v);
    const before = await page.evaluate(() => Files.T().cursor);
    await page.keyboard.press('ArrowDown'); await page.keyboard.press('ArrowRight'); await wait(150);
    console.log(v, 'cursor', before, '->', await page.evaluate(() => Files.T().cursor));
  }
  await page.evaluate(() => { CC.run('view-details'); Files.go('/mnt/user/downloads', 0); });

  await page.click('.panel.active [data-viewmenu]'); await wait();
  console.log('view menu:', (await page.locator('#menu button').allTextContents()).map(s => s.trim()).join(' | '));
  await shot('r8-viewmenu');
  await page.keyboard.press('Escape');

  await page.evaluate(() => { const t = Files.T(); t.marked = new Set(Files.entries().slice(0, 4).map(k => k.name)); CC.render(); });
  await page.click('.panel.active .lclear'); await wait();
  console.log('after footer x:', await page.evaluate(() => Files.T().marked.size));
  await page.evaluate(() => { const t = Files.T(); t.marked = new Set(Files.entries().slice(0, 4).map(k => k.name)); CC.render(); });
  const box = await page.locator('.panel.active .list').boundingBox();
  await page.mouse.click(box.x + 200, box.y + box.height - 30); await wait();
  console.log('after empty click:', await page.evaluate(() => Files.T().marked.size));

  console.log('pack label:', await page.evaluate(() => CC.actions.pack.label));
  await page.keyboard.press('Control+Shift+V'); await wait();
  console.log('after Ctrl+Shift+V:', await page.evaluate(() => Files.T().view));
};
