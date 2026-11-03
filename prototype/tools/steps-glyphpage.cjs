// The format glyph page: pick another glyph for one group and check the saved answer text.
module.exports = async (page, shot) => {
  await page.waitForTimeout(600);
  await shot('gp-1');
  await page.click('.fmt[data-id="pdf"] .pick button[data-g="IconManual"]');
  await page.waitForTimeout(200);
  console.log('answer:', await page.evaluate(() => answers()));
};
