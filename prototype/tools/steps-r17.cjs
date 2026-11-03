// Round 17: one window each for colours and tags, colours can be added and deleted, and the side buttons step through windows.
module.exports = async (page, shot) => {
  const wait = (ms = 350) => page.waitForTimeout(ms);
  await wait(600);
  await page.click('.panel.active [data-labelsmenu]'); await wait();
  console.log('pencils:', await page.evaluate(() => $$('#menu .hedit').map(b => b.dataset.tip).join(' | ')));
  await page.click('#menu .hedit >> nth=0'); await wait();
  console.log('colours window:', await page.evaluate(() => `${$('.win .notch').textContent}, ${$$('[data-colname]').length} rows`));
  await page.fill('[data-colname="0"]', 'Dringend');
  await page.click('[data-coldel="5"]'); await wait();
  await page.fill('#mk-new', 'Türkis'); await page.keyboard.press('Enter'); await wait();
  await shot('r17-1-colours');
  await page.click('.win-foot .btn.primary'); await wait();
  console.log('colours after:', await page.evaluate(() => LABEL_COLORS.map(c => c[1]).join(', ')), '| purple label gone:', await page.evaluate(() => !DATA.labels['/mnt/user/fotos/2026/IMG_0203.jpg']?.color));
  await page.click('.panel.active [data-labelsmenu]'); await wait();
  await page.click('#menu .hedit >> nth=1'); await wait();
  console.log('tags window:', await page.evaluate(() => `${$('.win .notch').textContent}, ${$$('[data-tagname]').length} rows`));
  await page.fill('[data-tagname="0"]', 'erledigt'); await page.click('.win-foot .btn.primary'); await wait();
  console.log('tags after:', await page.evaluate(() => Labels.tags().join(', ')));

  // Side buttons: the browser history steps through the folders of the disk usage window.
  await page.evaluate(() => CC.run('diskUsage', '/mnt/user')); await wait(500);
  await page.click('#du-view [data-v="tree"]'); await wait();
  await page.dblclick('#du-body .du-row.dir >> nth=0'); await wait();
  const here = () => page.evaluate(() => $('#du-crumbs').innerText.replace(/\s+/g, ' '));
  console.log('in:', await here());
  await page.goBack(); await wait();
  console.log('after back:', await here());
  await page.goForward(); await wait();
  console.log('after forward:', await here());
  await page.keyboard.press('Escape'); await page.evaluate(() => { while (winStack.length) CC.closeWin(); }); await wait(500);
  console.log('history back at panel level:', await page.evaluate(() => Files.T().path));
  await shot('r17-2-after');
};
