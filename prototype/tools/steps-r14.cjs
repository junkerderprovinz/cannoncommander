// Round 14: disk usage in the manner of TreeSize, and the search field's menu with saved searches and labels.
module.exports = async (page, shot) => {
  const wait = (ms = 350) => page.waitForTimeout(ms);
  const closeAll = () => page.evaluate(() => { closeMenu(); while (winStack.length) CC.closeWin(); });
  const rows = () => page.evaluate(() => $$('#du-body .du-row:not(.du-head)').slice(0, 6).map(r => r.innerText.replace(/\s+/g, ' ').trim()).join(' | '));
  await wait(600);

  console.log('sidebar cards:', await page.evaluate(() => $$('#side .side-card .notch').map(n => n.textContent).join(' | ')));
  await shot('r14-0-sidebar');

  // Tree: sorted by size, expand with the toggle and with the right arrow, sort by name, pick two and delete them.
  await page.evaluate(() => { CC.setPref('duView', 'tree'); CC.run('diskUsage', '/mnt/user'); }); await wait(500);
  console.log('tree:', await rows());
  console.log('foot:', await page.evaluate(() => $('.bk-du .win-foot .start').textContent), 'delete disabled:', await page.evaluate(() => $('.bk-du .win-foot [data-f="2"]').disabled));
  await page.click('#du-body .du-row:not(.du-head) [data-du-tog]'); await wait();
  console.log('expanded:', await rows());
  await page.keyboard.press('ArrowDown'); await page.keyboard.press('ArrowRight'); await wait();
  console.log('arrow right, open rows:', await page.evaluate(() => $$('#du-body [aria-expanded="true"]').length));
  await shot('r14-1-tree');
  await page.click('[data-du-sort="name"]'); await wait();
  console.log('by name:', await rows());
  await page.click('[data-du-sort="size"]'); await wait();
  const backups = await page.evaluate(() => $$('#du-body .du-row').findIndex(r => r.innerText.startsWith('backups')));
  await page.evaluate((i) => CC.run('diskUsage', '/mnt/user/backups'), backups); await wait(400);
  await closeAll();
  await page.evaluate(() => CC.run('diskUsage', '/mnt/user/backups')); await wait(400);
  console.log('backups:', await rows());
  await page.click('#du-body .du-row:not(.du-head) >> nth=0');
  await page.click('#du-body .du-row:not(.du-head) >> nth=1', { modifiers: ['Control'] }); await wait();
  console.log('picked:', await page.evaluate(() => $('.bk-du .win-foot .start').textContent));
  await page.click('#du-body .du-row.sel >> nth=0', { button: 'right' }); await wait();
  console.log('menu:', await page.evaluate(() => $$('#menu button').map(b => b.innerText.trim()).join(' | ')));
  await page.keyboard.press('Escape'); await page.evaluate(() => closeMenu());
  await page.focus('#du-body'); await page.keyboard.press('Delete'); await wait();
  console.log('delete window:', await page.evaluate(() => $('.xwin .xdel b')?.textContent));
  await shot('r14-2-delete');
  await page.click('.xwin .win-foot .btn.primary'); await wait(500);
  console.log('after delete:', await rows(), '| trash:', await page.evaluate(() => DATA.trash.slice(0, 2).map(t => t.name).join(', ')));
  await page.click('.toast button:has-text("Rückgängig")').catch(() => console.log('no undo button')); await wait(400);
  console.log('after undo:', await rows());

  await page.click('#du-view [data-v="types"]'); await wait();
  console.log('types:', await rows());
  await shot('r14-3-types');
  await page.click('#du-body [data-du-kind] >> nth=0'); await wait();
  console.log('largest of one kind:', await page.evaluate(() => $('.du-kind')?.innerText.trim()), await rows());
  await page.click('[data-du-allkinds]'); await wait();
  console.log('largest:', await rows());
  await shot('r14-4-largest');
  await page.click('#du-view [data-v="map"]'); await wait(500);
  console.log('map blocks:', await page.evaluate(() => $$('#du-body .bk-blk').length));
  await shot('r14-5-map');
  await page.evaluate(() => CC.setPref('duView', 'tree'));
  await closeAll();

  await page.click('.panel.active [data-run="quickFilter"]'); await wait();
  console.log('search menu:', await page.evaluate(() => $('.qdrop')?.innerText.replace(/\s+/g, ' ')));
  await shot('r14-6-searchmenu');
  await page.keyboard.type('*.mkv'); await wait();
  console.log('menu while typing:', await page.evaluate(() => !!$('.qdrop')), 'save button:', await page.evaluate(() => !!$('[data-qfsave]')));
  await page.click('[data-qfsave]'); await wait();
  console.log('save window:', await page.evaluate(() => $('.win [aria-label], .win .notch')?.textContent), await page.evaluate(() => $('#sm-name')?.value));
  await page.click('.win-foot .btn.primary'); await wait();
  await page.fill('.qfilter input', ''); await page.evaluate(() => { const i = $('.qfilter input'); i.dispatchEvent(new Event('input', { bubbles: true })); }); await wait();
  await page.click('.qfilter input'); await wait();
  console.log('saved rows:', await page.evaluate(() => $$('.qdrop .qrow').map(r => r.innerText.replace(/\s+/g, ' ')).join(' | ')));
  await page.click('.qdrop [data-labelsearch="#steuer"]'); await wait();
  console.log('label search:', await page.evaluate(() => `${Files.T().path} ${Files.T().filter} ${Files.entries().length} hits`));
  await page.click('[data-qfclose]'); await page.click('.panel.active [data-run="quickFilter"]'); await wait();
  await page.click('.qdrop .qrow >> nth=0'); await wait();
  console.log('saved search open:', await page.evaluate(() => $('.smart-bar')?.innerText.replace(/\s+/g, ' ')));
};
