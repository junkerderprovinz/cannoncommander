// Planning a synchronisation hands it over to ArrowLoop instead of a planner of our own.
module.exports = async (page, shot) => {
  const wait = (ms = 350) => page.waitForTimeout(ms);
  await wait(600);
  await page.evaluate(() => { CC.prefs.dual = true; Files.go('/mnt/user/dokumente', 0); Files.go('/mnt/user/backups', 1); CC.run('sync'); }); await wait(500);
  await page.click('.win-foot .btn:has-text("In ArrowLoop planen")'); await wait();
  console.log('hand-over window:', await page.evaluate(() => `${$$('.win .notch').map(n => n.textContent).join(' > ')} | ${$('#al-name').value}`));
  await shot('r20-al');
  await page.click('.scrim:last-of-type .win-foot .btn.primary'); await wait();
  console.log('toast:', await page.evaluate(() => $('.toast')?.innerText.replace(/\s+/g, ' ')), '| sync still open:', await page.evaluate(() => winStack.length));
};
