// A tour through every view and the main windows, dark and light, desktop and phone width, for a design review.
module.exports = async (page, shot) => {
  const wait = (ms = 350) => page.waitForTimeout(ms);
  const close = () => page.evaluate(() => { while (winStack.length) CC.closeWin(); closeMenu(); });
  const hidePw = () => page.evaluate(() => document.querySelectorAll('[class*="pw-"],[id*="pw-"],#pinnwand').forEach(e => { e.style.display = 'none'; }));
  await wait(800);
  await hidePw();

  await page.evaluate(() => { Files.go('/mnt/user/fotos/2026', 0); CC.setPref('quickview', true); const t = Files.T(); t.cursor = 2; CC.render(); });
  await wait(); await shot('tour-01-quickview');
  await page.evaluate(() => { CC.setPref('quickview', false); CC.setPref('dual', false); CC.render(); });
  await wait(); await shot('tour-02-single');
  await page.evaluate(() => { CC.setPref('dual', true); Files.go('/mnt/user/downloads', 0); const t = Files.T(); t.marked = new Set(['album.flac', 'setup-notes.md']); CC.render(); });
  await wait(); await shot('tour-03-marked');

  for (const [id, name] of [['view', 'viewer'], ['props', 'props'], ['search', 'search'], ['openTrash', 'trash'], ['openActivity', 'activity'], ['shortcuts', 'keys'], ['diskUsage', 'diskusage'], ['multiRename', 'rename'], ['compareDirs', 'compare']]) {
    await close();
    await page.evaluate((a) => { Files.go('/mnt/user/downloads', 0); const t = Files.T(); t.cursor = Files.entries().findIndex(k => k.name === 'setup-notes.md'); t.marked = new Set(); CC.render(); try { CC.run(a); } catch (e) { console.error(a, e.message); } }, id);
    await wait(500); await shot('tour-10-' + name);
  }
  await close();
  await page.evaluate(() => { Jobs && CC.run('copy'); });
  await wait(400); await shot('tour-20-copy');
  await close();

  const tabs = await page.evaluate(() => { CC.go('settings'); return CC.orderedTabs ? CC.orderedTabs().map(t => t.id || t[0]) : []; });
  console.log('settings tabs:', tabs.join(', '));
  for (const t of tabs) {
    await page.evaluate((x) => CC.go('settings', x), t);
    await wait(); await shot('tour-30-settings-' + t);
  }
  await page.evaluate(() => CC.go('files'));

  await page.evaluate(() => { CC.setPref('theme', 'light'); CC.applyPrefs(); CC.render(); });
  await wait(); await shot('tour-40-light');
  await page.evaluate(() => CC.run('search'));
  await wait(); await shot('tour-41-light-search');
  await close();
  await page.evaluate(() => { CC.setPref('theme', 'dark'); CC.applyPrefs(); CC.render(); });

  await page.setViewportSize({ width: 390, height: 844 });
  await wait(600); await shot('tour-50-phone');
  await page.evaluate(() => CC.go('settings'));
  await wait(); await shot('tour-51-phone-settings');
  await page.setViewportSize({ width: 1600, height: 950 });
};
