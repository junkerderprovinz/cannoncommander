// Round 16: the six colours, the pencil in the labels menu with the Markierungen window, and the path bar in disk usage.
module.exports = async (page, shot) => {
  const wait = (ms = 350) => page.waitForTimeout(ms);
  await wait(600);
  await page.click('.panel.active [data-labelsmenu]'); await wait();
  console.log('menu:', await page.evaluate(() => $$('#menu > *').map(b => b.innerText.replace(/\s+/g, ' ').trim()).join(' | ')), 'pencils:', await page.locator('#menu .hedit').count());
  await shot('r16-1-menu');
  await page.click('#menu .hedit >> nth=0'); await wait();
  console.log('window:', await page.evaluate(() => $('.win .notch')?.textContent), 'colour rows:', await page.locator('[data-colname]').count(), 'tag rows:', await page.locator('[data-tagname]').count());
  await page.fill('[data-colname="red"]', 'Dringend');
  await page.fill('[data-tagname="0"]', 'erledigt');
  await page.click('[data-tagdel="1"]'); await wait();
  await page.fill('#mk-new', 'Urlaub'); await page.keyboard.press('Enter'); await wait();
  await shot('r16-2-manage');
  console.log('rows before apply:', await page.evaluate(() => $$('[data-tagname]').map(i => i.value).join(', ')));
  await page.click('.win-foot .btn.primary'); await wait();
  console.log('after apply:', await page.evaluate(() => `${Labels.color('red')[1]} | tags ${Labels.tags().join(', ')} | labels ${JSON.stringify(DATA.labels['/mnt/user/downloads/ubuntu-24.04.3-desktop-amd64.iso'])}`));
  await page.click('.panel.active [data-labelsmenu]'); await wait();
  await page.click('#menu button:has-text("Dringend")'); await wait();
  console.log('search by renamed colour:', await page.evaluate(() => `${Files.T().filter} ${Files.entries().map(k => k.name).join(', ')}`));
  await page.evaluate(() => { closeMenu(); while (winStack.length) CC.closeWin(); CC.run('diskUsage', '/mnt/user/backups'); }); await wait(500);
  console.log('du path:', await page.evaluate(() => $('#du-crumbs').innerText.replace(/\s+/g, ' ')), 'class:', await page.evaluate(() => $('#du-crumbs').className));
  await page.click('#du-crumbs [data-du-cd="/mnt/user"]'); await wait();
  console.log('du after click:', await page.evaluate(() => $('#du-crumbs').innerText.replace(/\s+/g, ' ')));
  await shot('r16-3-du');
};
