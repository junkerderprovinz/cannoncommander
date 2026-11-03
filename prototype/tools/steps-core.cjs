module.exports = async (page, shot) => {
  await shot('core-1');
  await page.click('[data-menu="0"]'); await page.waitForTimeout(200); await shot('core-menu-datei');
  await page.keyboard.press('Escape');
  await page.click('[data-menu="2"]'); await page.waitForTimeout(200); await shot('core-menu-ansicht');
  await page.keyboard.press('Escape');
  await page.keyboard.press('ArrowDown'); await page.keyboard.press('Insert'); await page.keyboard.press('Insert');
  await page.keyboard.press('F5'); await page.waitForTimeout(300); await shot('core-copy');
  await page.click('.win-foot .btn.primary'); await page.waitForTimeout(3500); await shot('core-after');
  await page.click('[data-go-view="settings"]'); await page.waitForTimeout(300); await shot('core-settings');
};
