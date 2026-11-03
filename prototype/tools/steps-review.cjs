// A walk through the main screens of the bundled prototype, one screenshot each.
module.exports = async (page, shot) => {
  const esc = () => page.keyboard.press('Escape');
  const wait = (ms = 450) => page.waitForTimeout(ms);
  const run = async (id, name) => { await page.evaluate((a) => CC.run(a), id); await wait(); await shot('rv-' + name); };

  await shot('rv-01-main');
  await page.click('[data-menu="4"]'); await wait(250); await shot('rv-02-menu-werkzeuge'); await esc();

  await page.evaluate(() => Files.go('/mnt/user/fotos/2026'));
  await page.keyboard.press('ArrowDown'); await page.keyboard.press('ArrowDown');
  await run('view', '03-view-image'); await esc();

  await page.evaluate(() => Files.go('/mnt/user/dokumente'));
  await page.evaluate(() => { const i = Files.entries().findIndex(k => k.name === 'Einkaufsliste.md'); Files.T().cursor = i; CC.render(); });
  await run('edit', '04-editor'); await esc(); await wait(200); await esc();

  await run('search', '05-search'); await esc();
  await page.evaluate(() => { Files.go('/mnt/user/isos', 0); Files.go('/mnt/user/backups', 1); });
  await run('compareDirs', '06-compare'); await esc();
  await page.evaluate(() => Files.go('/mnt/user', 0));
  await run('diskUsage', '07-diskusage'); await esc();

  await page.evaluate(() => Files.go('/mnt/user/fotos/2025'));
  await page.keyboard.press('Control+a');
  await run('multiRename', '08-rename'); await esc();
  await run('props', '09-props'); await esc();

  await page.evaluate(() => CC.go('settings', 'look')); await wait(); await shot('rv-10-settings-look');
  await page.evaluate(() => CC.go('settings', 'users')); await wait(); await shot('rv-11-settings-users');
  await page.evaluate(() => CC.go('settings', 'info')); await wait(); await shot('rv-12-settings-info');
  await run('openTrash', '13-trash');
  await page.evaluate(() => CC.go('files'));
  await run('upload', '14-upload'); await esc();

  await page.evaluate(() => { CC.signedIn = false; CC.render(); }); await wait(); await shot('rv-15-login');
  await page.evaluate(() => { CC.signedIn = true; CC.render(); });
  await page.setViewportSize({ width: 390, height: 844 }); await wait(); await shot('rv-16-phone');
};
