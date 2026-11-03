// Round 5: cascading column menu, the column window with checkboxes, no parent row, drawn glyphs.
module.exports = async (page, shot) => {
  const wait = (ms = 400) => page.waitForTimeout(ms);
  await wait(800);
  console.log('parent rows:', await page.locator('.row.up').count());
  await page.evaluate(() => { Files.go('/mnt/user/dokumente', 0); Files.active = 0; CC.render(); });
  await wait(300);
  await shot('r5-main');

  await page.click('.panel.active .lhead', { button: 'right' }); await wait(250);
  console.log('header menu:', (await page.locator('#menu > button').allTextContents()).join(' | '));
  await page.hover('#menu [data-sub]:nth-of-type(7)'); await wait(250);
  console.log('fotos submenu items:', await page.locator('#submenu [role="menuitemcheckbox"]').count());
  await shot('r5-cascade');
  await page.click('#submenu [role="menuitemcheckbox"] >> nth=2'); await wait(300);
  console.log('after tick, submenu open:', await page.locator('#submenu').count(), 'columns:', await page.evaluate(() => Columns.shown().join(',')));
  await page.keyboard.press('Escape'); await wait(200);
  console.log('menus after escape:', await page.locator('#menu, #submenu').count());
  await page.evaluate(() => Columns.set(DEFAULT_COLUMNS));

  await page.evaluate(() => CC.run('columns')); await wait(300);
  await shot('r5-colwin');
  await page.click('.col-check[data-col="iso"]'); await wait(100);
  console.log('iso checked:', await page.getAttribute('.col-check[data-col="iso"]', 'aria-checked'));
  await page.fill('#col-q', 'codec'); await wait(200);
  console.log('visible for codec:', await page.locator('.col-check:visible').count());
  await shot('r5-colsearch');
  await page.evaluate(() => { while (winStack.length) CC.closeWin(); });

  await page.evaluate(() => { Files.go('/mnt/user/downloads', 0); CC.render(); });
  await wait(300);
  await shot('r5-glyphs-in-list');
};
