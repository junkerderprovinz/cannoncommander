// Round 15: search field in the path row with examples, labels button, tag editor, treemap first, wide F-keys, settings in one column.
module.exports = async (page, shot) => {
  const wait = (ms = 350) => page.waitForTimeout(ms);
  const closeAll = () => page.evaluate(() => { closeMenu(); while (winStack.length) CC.closeWin(); });
  await wait(600);
  await page.evaluate(() => CC.setPref('fkeys', true)); await wait();

  await page.click('.panel.active [data-run="quickFilter"]'); await wait();
  const box = await page.evaluate(() => { const q = $('.panel.active .qfilter'), c = $('.panel.active .crumbs'); return q && c && `${q.parentElement.className} inside crumbs: ${c.contains(q)}`; });
  console.log('field:', box);
  console.log('menu:', await page.evaluate(() => $('.qdrop')?.innerText.replace(/\s+/g, ' ')));
  await shot('r15-1-search');
  await page.click('.qdrop [data-example="*.mkv"]'); await wait();
  console.log('example:', await page.evaluate(() => `${Files.T().filter} ${Files.entries().length} hits, focused: ${document.activeElement?.id}`));
  await page.click('[data-qfclose]'); await wait();

  await page.click('.panel.active [data-labelsmenu]'); await wait();
  console.log('labels menu:', await page.evaluate(() => $$('#menu > *').map(b => b.innerText.replace(/\s+/g, ' ').trim()).join(' | ')));
  await shot('r15-2-labels');
  await page.click('#menu button:has-text("Steuer")'); await wait();
  console.log('label search:', await page.evaluate(() => `${Files.T().path} ${Files.T().filter} ${Files.entries().map(k => k.name).join(', ')}`));
  await page.click('[data-qfclose]'); await wait();

  await page.evaluate(() => { Files.go('/mnt/user/dokumente', 0); const t = Files.T(); t.cursor = Files.entries().findIndex(k => k.name === 'notizen.txt'); CC.render(); });
  await page.click('.panel.active .row.cursor', { button: 'right' }); await wait();
  await page.click('#menu button:has-text("Schlagworte")'); await wait();
  await page.fill('#newtag', 'neu'); await page.keyboard.press('Enter'); await wait();
  console.log('tag editor:', await page.evaluate(() => `${$('.win .well').firstElementChild.className} first, tags: ${$$('#tags .tagchip').map(c => c.textContent.trim() + (c.classList.contains('on') ? '(on)' : '')).join(' ')}`));
  await shot('r15-3-tags');
  await page.click('#tags [data-untag="neu"]'); await wait();
  console.log('after x:', await page.evaluate(() => $$('#tags .tagchip.on').map(c => c.textContent.trim()).join(' ')));
  await closeAll();

  await page.evaluate(() => CC.run('diskUsage', '/mnt/user')); await wait(500);
  console.log('disk usage opens on:', await page.evaluate(() => $('#du-view [aria-pressed="true"]').dataset.v), 'blocks:', await page.evaluate(() => $$('#du-body .bk-blk').length));
  await closeAll();

  console.log('fkeys:', await page.evaluate(() => { const bar = $('#fkeys').getBoundingClientRect(), bs = $$('#fkeys .btn'), last = bs.at(-1).getBoundingClientRect(); return `${bs.length} buttons, bar ${Math.round(bar.width)}px, right gap ${Math.round(bar.right - last.right)}px`; }));
  await shot('r15-4-fkeys');

  await page.setViewportSize({ width: 1908, height: 1000 });
  await page.evaluate(() => CC.go('settings')); await wait(500);
  console.log('settings columns:', await page.evaluate(() => { const xs = [...new Set($$('.settings-col > .card').map(c => Math.round(c.getBoundingClientRect().left)))]; return xs.length; }));
  await shot('r15-5-settings');
};
