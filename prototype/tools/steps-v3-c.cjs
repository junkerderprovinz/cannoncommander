// Proposals for the server side of jobs and lists: space check, parity wait, speed limits, job messages, live refresh and folder pictures.
module.exports = async (page, shot) => {
  const wait = (ms = 350) => page.waitForTimeout(ms);
  const closeAll = () => page.evaluate(() => { closeMenu(); while (winStack.length) CC.closeWin(); });
  await page.setViewportSize({ width: 1700, height: 1000 });
  await wait(600);

  // Space: the 64 GB image into filme, where Unraid would fill up Disk 1 with only 48 GB left.
  await page.evaluate(() => { Files.go('/mnt/user/backups', 0); Files.go('/mnt/user/filme', 1); const t = Files.T(0); Files.active = 0; t.cursor = Files.entries(0).findIndex(k => k.name.startsWith('vm-win11')); t.marked = new Set(); CC.render(); CC.run('copy'); });
  await wait();
  console.log('space warning:', await page.evaluate(() => $('.u-space p')?.textContent));
  console.log('button:', await page.evaluate(() => `${$('.u-space [data-space-disk]')?.innerText} | primary ${$('.xwin .win-foot .btn.primary').innerText}`));
  await shot('v3c-1-space');
  await page.click('.u-space [data-space-disk]'); await wait();
  console.log('after fix:', await page.evaluate(() => `${$('#dest').value} | warning ${!!$('.u-space')} | primary ${$('.xwin .win-foot .btn.primary').innerText}`));
  await page.fill('#dest', '/mnt/user/isos'); await page.dispatchEvent('#dest', 'input'); await wait();
  console.log('into isos:', await page.evaluate(() => !!$('.u-space')));
  await closeAll();

  // Speed limit for all jobs, then one job's own limit from the jobs window.
  await page.evaluate(() => CC.setPref('speedCap', 50));
  await page.evaluate(async () => { const kids = FS.kids(FS.nodeAt('/mnt/user/backups')); await Files.transfer('copy', [kids.find(k => k.name.startsWith('appdata-2026-10-31'))], '/mnt/user/dokumente', '/mnt/user/backups'); });
  await wait(1200);
  console.log('rate with 50 MB/s for all:', await page.evaluate(() => Math.round(Jobs.running()[0].rate / MB)));
  await page.evaluate(() => CC.run('jobs')); await wait();
  console.log('job line:', await page.evaluate(() => $('#jobs-body .job .sub').innerText));
  await page.click('#jobs-body [data-jact="limit"]'); await wait();
  console.log('limit menu:', await page.evaluate(() => $$('#menu button').map(b => b.innerText.trim()).join(' | ')));
  await shot('v3c-2-limit');
  await page.click('#menu button:has-text("25 MB/s")'); await wait(900);
  console.log('rate with 25 MB/s own:', await page.evaluate(() => Math.round(Jobs.running()[0].rate / MB)));
  await page.evaluate(() => { Jobs.running().forEach(Jobs.cancel); Jobs.list = []; Jobs.changed(); CC.setPref('speedCap', 0); });
  await closeAll();

  // Parity: a big copy waits, a small one asks first.
  await page.evaluate(() => CC.parity.start()); await wait();
  await page.evaluate(async () => { const kids = FS.kids(FS.nodeAt('/mnt/user/backups')); await Files.transfer('copy', [kids.find(k => k.name.startsWith('appdata-2026-10-24'))], '/mnt/user/dokumente', '/mnt/user/backups'); });
  await wait(900);
  console.log('big job:', await page.evaluate(() => `${Jobs.running()[0].state} held ${Jobs.held(Jobs.running()[0])} pct ${Math.round(Jobs.running()[0].pct)}`));
  await page.evaluate(() => { const kids = FS.kids(FS.nodeAt('/mnt/user/backups')); Files.transfer('copy', [kids.find(k => k.name.startsWith('flash'))], '/mnt/user/dokumente', '/mnt/user/backups'); });
  await wait();
  console.log('small asks:', await page.evaluate(() => $('.win .notch')?.textContent));
  await shot('v3c-3-parity-ask');
  await page.click('.win-foot button:has-text("Danach")'); await wait(600);
  await page.evaluate(() => CC.run('jobs')); await wait();
  console.log('jobs:', await page.evaluate(() => $$('#jobs-body .job .sub').map(s => s.innerText).join(' | ')));
  console.log('sidebar:', await page.evaluate(() => $('[data-parity]')?.innerText.replace(/\s+/g, ' ')));
  await shot('v3c-4-parity-jobs');
  await closeAll();
  await page.evaluate(() => CC.parity.stop(true)); await wait(900);
  console.log('after parity:', await page.evaluate(() => Jobs.running().map(j => j.state).join(', ')));
  await page.evaluate(() => { Jobs.running().forEach(Jobs.cancel); Jobs.list = []; Jobs.changed(); });

  // Messages: the browser's Notification is replaced by a recorder, the Unraid message lands in the activity log.
  await page.evaluate(() => {
    window.sent = [];
    window.Notification = class { constructor(t, o) { sent.push(`${t}: ${o.body}`); } static permission = 'granted'; static requestPermission() { return Promise.resolve('granted'); } };
    CC.prefs.notifyDone = true; CC.prefs.notifyUnraid = true; CC.prefs.notifyAfter = 0;
  });
  await page.evaluate(async () => { const kids = FS.kids(FS.nodeAt('/mnt/user/backups')); await Files.transfer('copy', [kids.find(k => k.name.startsWith('flash'))], '/mnt/user/fotos', '/mnt/user/backups'); });
  await page.waitForFunction(() => window.sent.length > 0, null, { timeout: 15000 }).catch(() => {});
  console.log('browser message:', await page.evaluate(() => window.sent.join(' | ')), '| unraid:', await page.evaluate(() => DATA.activity[0]?.who === 'Unraid'));
  await page.evaluate(() => CC.go('settings', 'general')); await wait(500);
  console.log('settings rows:', await page.evaluate(() => $$('.card').find(c => $('.notch', c).textContent === 'Aufgaben').innerText.split('\n').filter(l => l.trim()).slice(0, 9).join(' | ')));
  await page.click('#f-notify-test'); await wait();
  console.log('test message:', await page.evaluate(() => window.sent.at(-1)));
  await shot('v3c-5-settings');
  await page.evaluate(() => document.documentElement.dataset.theme = 'light'); await wait();
  await shot('v3c-6-settings-light');
  await page.evaluate(() => { document.documentElement.dataset.theme = 'dark'; CC.go('files'); });

  // Live refresh: the played download client writes into downloads, the cursor stays on its file.
  await page.evaluate(() => { Files.go('/mnt/user/downloads', 0); const t = Files.T(0); t.cursor = Files.entries(0).findIndex(k => k.name === 'setup-notes.md'); t.marked = new Set(); CC.render(); });
  await wait();
  await page.evaluate(() => CC.live.demo(true)); await wait(400);
  console.log('new row:', await page.evaluate(() => $('.panel[data-panel="0"] .row.fresh')?.innerText.split('\n')[0]), '| cursor on:', await page.evaluate(() => Files.entries(0)[Files.T(0).cursor].name));
  await shot('v3c-7-live');
  await page.evaluate(() => CC.run('jobs')); await wait(4500);
  console.log('while a window is open:', await page.evaluate(() => $$('.panel[data-panel="0"] .row').some(r => r.innerText.includes('900 MB'))));
  await closeAll(); await wait(1200);
  console.log('after closing:', await page.evaluate(() => $$('.panel[data-panel="0"] .row').map(r => r.innerText.split('\n')[0]).filter(n => /Cosmos/.test(n)).join(', ')));
  await page.evaluate(() => CC.live.demo(false));

  // Folder pictures in the card views.
  await page.evaluate(() => { Files.go('/mnt/user/filme', 0); CC.run('view-tiles'); });
  await wait(500);
  console.log('film folders with a picture:', await page.locator('.panel[data-panel="0"] .folderpic').count());
  await shot('v3c-8-posters');
  await page.evaluate(() => { Files.go('/mnt/user/musik', 0); CC.run('view-large'); }); await wait(500);
  console.log('music folder with a cover:', await page.locator('.panel[data-panel="0"] .folderpic').count());
  await shot('v3c-9-cover');
  await page.evaluate(() => { CC.run('view-details'); Files.go('/mnt/user/filme', 0); CC.run('view-details'); });
};
