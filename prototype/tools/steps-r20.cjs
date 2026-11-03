// Round 20: no meters in the tree, the drives view as the default where volumes live, per folder views, and the column view carrying on.
module.exports = async (page, shot) => {
  const wait = (ms = 350) => page.waitForTimeout(ms);
  await page.setViewportSize({ width: 1908, height: 1000 });
  await wait(600);
  console.log('tree meters:', await page.locator('#tree .vmeter').count());
  await page.evaluate(() => { CC.setPref('folderViews', {}); Files.go('/mnt', 0); }); await wait();
  console.log('/mnt view:', await page.evaluate(() => viewOf(Files.T())), 'cards:', await page.locator('.panel.active .dcard').count());
  console.log('first card:', await page.evaluate(() => $('.panel.active .dcard')?.innerText.replace(/\s+/g, ' ')));
  await shot('r20-1-drives');
  await page.evaluate(() => Files.go('/', 0)); await wait();
  console.log('/ view:', await page.evaluate(() => viewOf(Files.T())));
  await page.evaluate(() => { Files.go('/mnt/user', 0); CC.run('view-columns'); }); await wait();
  await page.evaluate(() => Files.go('/mnt/user/filme', 0)); await wait();
  console.log('child of columns:', await page.evaluate(() => viewOf(Files.T())));
  const deeper = await page.evaluate(() => { const d = FS.kids(FS.nodeAt('/mnt/user/filme')).find(k => k.dir); return d && '/mnt/user/filme/' + d.name; });
  if (deeper) { await page.evaluate((p) => Files.go(p, 0), deeper); await wait(); console.log('grandchild of columns:', await page.evaluate(() => viewOf(Files.T()))); }
  await shot('r20-2-columns');
  await page.evaluate(() => { Files.go('/mnt/user/fotos', 0); CC.run('view-gallery'); Files.go('/mnt/user/musik', 0); }); await wait();
  console.log('musik keeps own default:', await page.evaluate(() => viewOf(Files.T())));
  await page.evaluate(() => Files.go('/mnt/user/fotos', 0)); await wait();
  console.log('fotos remembers:', await page.evaluate(() => viewOf(Files.T())));
  await page.evaluate(() => { Files.go('/mnt', 0); CC.run('view-details'); }); await wait();
  console.log('/mnt can be changed:', await page.evaluate(() => viewOf(Files.T())), 'list meters:', await page.locator('.panel.active .vcell').count());
  await page.evaluate(() => { CC.run('view-drives'); document.documentElement.dataset.theme = 'light'; }); await wait();
  await shot('r20-3-drives-light');
};
