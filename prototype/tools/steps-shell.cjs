module.exports = async (page, shot) => {
  await page.evaluate(() => CC.setPref('theme', 'light'));
  await page.waitForTimeout(200);
  await shot('shell-light');
  await page.evaluate(() => CC.setPref('theme', 'dark'));
  await page.setViewportSize({ width: 390, height: 844 });
  await page.waitForTimeout(300);
  await shot('shell-phone');
  await page.setViewportSize({ width: 1100, height: 800 });
  await page.waitForTimeout(300);
  await shot('shell-1100');
};
