// The redesigned copy, move and delete windows.
module.exports = async (page, shot) => {
  const wait = (ms = 400) => page.waitForTimeout(ms);
  await wait(800);
  await page.evaluate(() => { Files.go('/mnt/user/downloads', 0); Files.go('/mnt/user/backups', 1); Files.active = 0; const t = Files.T(); t.marked = new Set(['album.flac', 'setup-notes.md', 'Rechnung-Oktober.pdf', 'debian-13.1.0-amd64-netinst.iso', 'Fotos-Urlaub-2026.zip']); CC.render(); CC.run('copy'); });
  await wait(); await shot('r13-copy');
  await page.click('.xmore summary'); await wait(200);
  await shot('r13-copy-more');
  await page.fill('#dest', '/mnt/user/dokumente'); await page.dispatchEvent('#dest', 'input'); await wait(200);
  console.log('dest name:', await page.textContent('#xdest'));
  await page.evaluate(() => { while (winStack.length) CC.closeWin(); CC.run('move'); });
  await wait(); await shot('r13-move');
  await page.evaluate(() => { while (winStack.length) CC.closeWin(); CC.run('delete'); });
  await wait(); await shot('r13-delete');
  await page.click('#totrash'); await wait(300);
  console.log('forever label:', await page.textContent('.win-foot .btn.primary'), '|', await page.textContent('#xwhere'));
  await shot('r13-delete-forever');
  await page.evaluate(() => { while (winStack.length) CC.closeWin(); const t = Files.T(); t.marked = new Set(); t.cursor = 2; CC.render(); CC.run('delete'); });
  await wait(); await shot('r13-delete-one');
  await page.evaluate(() => { while (winStack.length) CC.closeWin(); });
};
