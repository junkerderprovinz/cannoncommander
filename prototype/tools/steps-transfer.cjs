// The transfer preview: every mode mid-run, finished, and in light.
module.exports = async (page, shot) => {
  const wait = (ms) => page.waitForTimeout(ms);
  for (const m of ['copy', 'move', 'delete', 'pack']) {
    await page.click(`[data-mode="${m}"]`); await wait(1700);
    console.log(m, 'pct', await page.textContent('#pct'), 'papers', await page.locator('.paper').count(), 'eta', await page.textContent('#eta'));
    await shot('tr-' + m);
  }
  await page.click('[data-mode="copy"]'); await wait(40000);
  console.log('finished:', await page.locator('.win.done').count(), await page.textContent('#eta'));
  await shot('tr-done');
  await page.click('#theme'); await page.click('[data-mode="delete"]'); await wait(1500);
  await shot('tr-light');
};
