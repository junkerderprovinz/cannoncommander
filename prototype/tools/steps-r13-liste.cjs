// Round 13, the list: flat view, grouping, view per folder, colours and tags, column view, dropping on folders and tabs.
module.exports = async (page, shot) => {
  const wait = (ms = 350) => page.waitForTimeout(ms);
  await wait(800);
  await page.evaluate(() => { Files.go('/mnt/user/fotos', 0); Files.active = 0; CC.run('flat'); });
  await wait();
  console.log('flat entries:', await page.evaluate(() => Files.entries().length), 'first:', await page.evaluate(() => { const k = Files.entries()[0]; return k.at + '/' + k.name; }));
  console.log('copy allowed on flat:', await page.evaluate(() => { Files.T().cursor = 0; return CC.blocked(CC.actions.copy) || 'ja'; }));
  await shot('r13-flat');
  await page.evaluate(() => CC.run('flat'));

  await page.evaluate(() => { Files.go('/mnt/user/downloads', 0); CC.run('group-kind'); });
  await wait(); await shot('r13-group-kind');
  console.log('group heads:', (await page.locator('.panel.active .lgroup').allTextContents()).join(' | '));
  await page.evaluate(() => CC.run('group-date')); await wait();
  console.log('date heads:', (await page.locator('.panel.active .lgroup').allTextContents()).join(' | '));
  await page.evaluate(() => CC.run('group-none'));

  await page.evaluate(() => { Files.go('/mnt/user/fotos/2026', 0); CC.run('view-gallery'); Files.go('/mnt/user/downloads', 0); });
  console.log('downloads view:', await page.evaluate(() => viewOf(Files.T())), 'back in fotos:', await page.evaluate(() => { Files.go('/mnt/user/fotos/2026', 0); return viewOf(Files.T()); }));
  await page.evaluate(() => { CC.run('view-details'); Files.go('/mnt/user/dokumente', 0); });
  await wait(); await shot('r13-labels');
  console.log('label marks:', await page.locator('.panel.active .cdot').count(), 'tags:', (await page.locator('.panel.active .pill.tag').allTextContents()).join(' '));
  await page.click('.panel.active [data-run="quickFilter"]'); await wait();
  console.log('search menu labels:', (await page.locator('.qdrop [data-labelsearch]').allTextContents()).map(s => s.trim()).join(' | '));
  await page.click('.qdrop [data-labelsearch="#steuer"]'); await wait();
  console.log('#steuer hits:', await page.evaluate(() => Files.entries().map(k => k.name).join(', ')));
  await shot('r13-tagsearch');
  await page.evaluate(() => { Files.go('/mnt/user/dokumente', 0); const t = Files.T(); t.cursor = Files.entries().findIndex(k => k.name === 'notizen.txt'); CC.render(); });
  await page.click('.panel.active .row.cursor', { button: 'right' }); await wait();
  await page.hover('#menu >> text=Farbe'); await wait();
  await shot('r13-colormenu');
  await page.click('#submenu >> text=Blau'); await wait();
  console.log('notizen color:', await page.evaluate(() => DATA.labels['/mnt/user/dokumente/notizen.txt']?.color));

  await page.evaluate(() => { Files.go('/mnt/user/fotos/2026', 0); CC.run('view-columns'); const t = Files.T(); t.cursor = 0; CC.render(); });
  await wait(); await shot('r13-columns');
  console.log('columns:', await page.locator('.panel.active .mcol').count());
  await page.evaluate(() => $('.panel.active .list').focus());
  await page.keyboard.press('ArrowLeft'); await wait();
  console.log('after left:', await page.evaluate(() => Files.T().path));
  await shot('r13-columns-up');
  await page.evaluate(() => CC.run('view-details'));

  await page.evaluate(() => { Files.go('/mnt/user/downloads', 0); Files.go('/mnt/user/backups', 1); CC.render(); });
  const src = page.locator('.panel[data-panel="0"] .row', { hasText: 'setup-notes.md' });
  const dst = page.locator('.panel[data-panel="0"] .row.dir', { hasText: 'unfertig' });
  await src.dragTo(dst); await wait(2500);
  console.log('dropped into unfertig:', await page.evaluate(() => FS.kids(FS.nodeAt('/mnt/user/downloads/unfertig')).some(k => k.name === 'setup-notes.md')));
};
