// Round 9: the sidebar as two cards, real previews on the cards, quick look on the space bar.
module.exports = async (page, shot) => {
  const wait = (ms = 350) => page.waitForTimeout(ms);
  await wait(800);
  console.log('sidebar cards:', await page.locator('.side .side-card').count());
  await shot('r9-main');

  for (const [dir, view] of [['/mnt/user/downloads', 'tiles'], ['/mnt/user/dokumente', 'tiles'], ['/mnt/user/fotos/2026', 'gallery'], ['/mnt/user/dokumente', 'content'], ['/mnt/user/musik/Kevin MacLeod', 'large']]) {
    const ok = await page.evaluate(([d, v]) => { if (!FS.nodeAt(d)) return false; Files.go(d, 0); Files.active = 0; CC.run('view-' + v); return true; }, [dir, view]);
    if (!ok) { console.log('missing folder', dir); continue; }
    await wait(); await shot('r9-' + view + '-' + dir.split('/').pop().replace(/\s+/g, '-'));
  }
  await page.evaluate(() => { CC.run('view-details'); Files.go('/mnt/user/fotos/2026', 0); const t = Files.T(); t.cursor = 2; CC.render(); $('.panel.active .list').focus(); });
  await page.keyboard.press(' '); await wait(500);
  console.log('quick look open:', await page.locator('.ql').count(), await page.locator('.ql-name').textContent());
  await shot('r9-quicklook');
  await page.keyboard.press('ArrowRight'); await wait(300);
  console.log('after arrow:', await page.locator('.ql-name').textContent(), 'cursor', await page.evaluate(() => Files.T().cursor));
  await page.keyboard.press(' '); await wait(200);
  console.log('closed with space:', await page.locator('.ql').count(), 'marked:', await page.evaluate(() => Files.T().marked.size));
  await page.evaluate(() => { Files.go('/mnt/user/downloads', 0); const t = Files.T(); t.cursor = Files.entries().findIndex(k => k.name === 'setup-notes.md'); CC.render(); $('.panel.active .list').focus(); });
  await page.keyboard.press(' '); await wait(500);
  await shot('r9-quicklook-text');
  await page.keyboard.press('Escape'); await wait(200);
  await page.keyboard.press('Insert'); await wait(200);
  console.log('insert marks:', await page.evaluate(() => Files.T().marked.size));
};
