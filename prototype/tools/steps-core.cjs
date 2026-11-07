module.exports = async (page, shot) => {
  await shot('core-1');
  await page.click('[data-menu="0"]'); await page.waitForTimeout(200); await shot('core-menu-datei');
  await page.keyboard.press('Escape');
  await page.click('[data-menu="2"]'); await page.waitForTimeout(200); await shot('core-menu-ansicht');
  await page.keyboard.press('Escape');
  await page.keyboard.press('ArrowDown'); await page.keyboard.press('Insert'); await page.keyboard.press('Insert');
  await page.keyboard.press('F5'); await page.waitForTimeout(300); await shot('core-copy');
  await page.click('.win-foot .btn.primary'); await page.waitForTimeout(300);
  // The selection holds hard-linked files, so the copy asks once more before it gives them their own data.
  await page.click('.win-foot .btn:has-text("Trotzdem kopieren")');
  await page.waitForTimeout(3500); await shot('core-after');
  // The app keeps the settings in the avatar's menu, the prototype has a gear for them in the head bar.
  if (await page.$('[data-go-view="settings"]')) await page.click('[data-go-view="settings"]');
  else { await page.click('.head [data-account]'); await page.waitForTimeout(200); await page.click('.glim-menu-item:has-text("Einstellungen")'); }
  await page.waitForTimeout(300); await shot('core-settings');
};
