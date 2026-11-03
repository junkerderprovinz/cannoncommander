// The transfer preview: every mode mid-run, pause, cancel, a failure with retry, finished, and in light.
module.exports = async (page, shot) => {
  const wait = (ms) => page.waitForTimeout(ms);
  const state = async () => `pct ${await page.textContent('#pct')}, papers ${await page.locator('.paper').count()}, left ${await page.textContent('#left')}, files ${await page.textContent('#files')}, speed ${await page.textContent('#speed')}, buttons ${(await page.locator('#foot button').allTextContents()).join('/')}`;
  for (const m of ['copy', 'move', 'delete', 'pack']) {
    await page.click(`[data-mode="${m}"]`); await wait(2600);
    console.log(m, await state());
    await shot('tr-' + m);
  }
  await page.click('[data-mode="copy"]'); await wait(2000);
  await page.click('#pause'); const held = await page.textContent('#pct'); await wait(1500);
  console.log('paused:', held === await page.textContent('#pct'), await state());
  await shot('tr-paused');
  await page.click('#pause'); await wait(800);
  console.log('resumed:', await state());
  await page.click('#cancel'); await wait(400);
  console.log('cancelled:', await page.locator('.win.cancelled').count(), await page.textContent('#ticker'), await state());
  await page.click('[data-mode="fail"]'); await wait(26000);
  console.log('failed:', await page.locator('.win.failed').count(), await page.textContent('#ticker'), await state());
  await shot('tr-failed');
  await page.click('#retry'); await wait(500);
  console.log('retry restarts:', await page.locator('.win.failed').count() === 0, await state());
  await page.click('[data-mode="copy"]'); await wait(42000);
  console.log('finished:', await page.locator('.win.done').count(), await state());
  await shot('tr-done');
  await page.click('#theme'); await page.click('[data-mode="copy"]'); await wait(6000);
  await shot('tr-light');
  await page.click('[data-mode="fail"]'); await wait(26000);
  await shot('tr-light-failed');
};
