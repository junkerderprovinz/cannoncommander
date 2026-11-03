// Round 6: browser history drives back and forward, the full F-key bar, tabs without roles.
module.exports = async (page, shot) => {
  const wait = (ms = 400) => page.waitForTimeout(ms);
  await wait(800);
  console.log('role badges:', await page.locator('.ptab .role').count());

  // Real clicks give the history entries a user gesture, as on jdp's machine.
  await page.click('#tree [data-go="/mnt/user/fotos"]'); await wait(250);
  await page.click('#tree [data-go="/mnt/user/musik"]'); await wait(250);
  console.log('now:', await page.evaluate(() => Files.T(0).path));
  await page.goBack(); await wait(300);
  console.log('after browser back:', await page.evaluate(() => Files.T(0).path));
  await page.goBack(); await wait(300);
  console.log('after second back:', await page.evaluate(() => Files.T(0).path));
  await page.goForward(); await wait(300);
  console.log('after forward:', await page.evaluate(() => Files.T(0).path));

  await page.evaluate(() => CC.setPref('fkeys', true)); await wait(300);
  console.log('F keys:', (await page.locator('#fkeys .kbd').allTextContents()).join(' '));
  await shot('r6-fkeys');
  await page.keyboard.press('F10'); await wait(250);
  console.log('F10 opens menu:', await page.locator('#menu').count());
  await page.keyboard.press('Escape');
  await page.evaluate(() => CC.setPref('fkeys', false));
};
