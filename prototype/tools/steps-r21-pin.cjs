// Round 21: a folder's menu pins it to the shortcuts, and once pinned the same menu unpins it again.
module.exports = async (page, shot) => {
  const wait = (ms = 300) => page.waitForTimeout(ms);
  const pins = () => page.evaluate(() => DATA.bookmarks.map(b => b.path).join(', '));
  const items = async () => (await page.locator('#menu [role="menuitem"]').allTextContents()).map(s => s.trim()).join(' | ');
  const row = '#tree [data-go="/mnt/user/isos"]';
  await wait(800);
  await page.evaluate(() => Files.go('/mnt/user', 0));
  await wait();
  console.log('pinned at the start:', await pins());

  await page.click(row, { button: 'right' }); await wait(250);
  console.log('menu of a folder:', await items());
  await page.click('#menu >> text=Als Verknüpfung anlegen'); await wait(250);
  console.log('pinned after adding:', await pins());

  await page.click(row, { button: 'right' }); await wait(250);
  console.log('menu of the pinned folder:', await items());
  await page.click('#menu >> text=Vom Schnellzugriff lösen'); await wait(250);
  console.log('pinned after unpinning:', await pins());

  await page.click(row, { button: 'right' }); await wait(250);
  console.log('menu of the folder again:', await items());
  await page.keyboard.press('Escape');
};
