// Round 19 head and drives: logo alone, magnifier for the command search, job count only while busy, a clear way out of the settings, and fill meters for volumes.
module.exports = async (page, shot) => {
  const wait = (ms = 350) => page.waitForTimeout(ms);
  await page.setViewportSize({ width: 1908, height: 1000 });
  await wait(600);
  console.log('brand text:', JSON.stringify(await page.evaluate(() => $('.brand').innerText.trim())), '| count visible:', await page.isVisible('#job-count'));
  await page.click('.head [data-run="palette"]'); await wait();
  console.log('palette from magnifier:', await page.evaluate(() => !!$('.pal-scrim')));
  await page.keyboard.press('Escape'); await wait();
  await page.evaluate(() => Files.go('/mnt', 0)); await wait();
  console.log('tree meters:', await page.locator('#tree .vmeter').count(), '| list meters:', await page.locator('.panel.active .vcell').count());
  console.log('list row:', await page.evaluate(() => $('.panel.active .vcell')?.closest('.row').innerText.replace(/\s+/g, ' ')));
  await shot('r19-1-drives');
  await page.click('.head [data-go-view="settings"]'); await wait();
  console.log('settings head:', await page.evaluate(() => `${!!$('.hback')} back, gear pressed ${$('.head [aria-pressed="true"]')?.dataset.goView}`));
  await shot('r19-2-settings');
  await page.keyboard.press('Escape'); await wait();
  console.log('after Esc:', await page.evaluate(() => CC.view));
  await page.click('.head [data-go-view="settings"]'); await wait();
  await page.click('.hback'); await wait();
  console.log('after back button:', await page.evaluate(() => CC.view));
  await page.evaluate(async () => { const kids = FS.kids(FS.nodeAt('/mnt/user/backups')); await Files.transfer('copy', [kids.find(k => k.name.startsWith('vm-win11'))], '/mnt/user/dokumente', '/mnt/user/backups'); }); await wait(900);
  console.log('count while busy:', await page.isVisible('#job-count'), await page.textContent('#job-count'));
};
