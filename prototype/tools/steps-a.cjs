// Walks through every settings tab and the admin windows, in the dark and the light theme.
module.exports = async (page, shot) => {
  const wait = (ms = 250) => page.waitForTimeout(ms);
  const tab = async (id) => { await page.evaluate((t) => CC.go('settings', t), id); await wait(); };
  const full = async (name) => {
    const h = await page.evaluate(() => document.getElementById('page')?.scrollHeight || 950);
    await page.setViewportSize({ width: 1600, height: Math.min(Math.max(950, h + 200), 3000) });
    await wait(150);
    await shot(name);
    await page.setViewportSize({ width: 1600, height: 950 });
  };
  const closeWin = async () => { await page.keyboard.press('Escape'); await wait(150); };
  const tabs = ['general', 'look', 'files', 'sources', 'remotes', 'users', 'rules', 'shares', 'security', 'integrations', 'app', 'info'];

  for (const theme of ['dark', 'light']) {
    await page.evaluate((t) => CC.setPref('theme', t), theme);
    for (const t of tabs) { await tab(t); await full(`a-${theme}-${t}`); }
  }
  await page.evaluate(() => CC.setPref('theme', 'dark'));

  // Live prefs: accent, shape, density, motion.
  await tab('look');
  await page.click('#a-shape [data-v="square"]'); await wait();
  await page.click('[data-preset="1"]'); await wait(300);
  await shot('a-look-picker');
  await page.mouse.click(200, 900); await wait();
  console.log('accent/shape:', await page.evaluate(() => [CC.prefs.accent, document.documentElement.dataset.shape, getComputedStyle(document.documentElement).getPropertyValue('--accent')]));
  for (let i = 0; i < 5; i++) { await page.click('#a-shape [data-v="square"]'); await wait(80); }
  console.log('leaf:', await page.evaluate(() => document.documentElement.dataset.shape));
  await page.click('#a-shape [data-v="round"]');
  await page.click('#a-accent-reset'); await wait();

  // Sources: edit window with validation, folder picker, reindex.
  await tab('sources');
  await page.click('[data-add]'); await wait();
  await page.click('.win-foot .btn.primary'); await wait(200);
  await page.fill('#a-sname', 'Disk 1');
  await page.click('[data-pickfor="a-spath"]'); await wait();
  await page.click('[data-pick="/mnt/disk1"]'); await wait();
  await shot('a-folder-picker');
  await page.click('.a-picker .win-foot .btn.primary'); await wait();
  await shot('a-source-new');
  await page.click('.win-foot .btn.primary'); await wait(400);
  await shot('a-source-added');
  console.log('sources:', await page.evaluate(() => DATA.sources.map(s => s.name + ':' + s.path).join(', ')));

  // Remotes: picker, form, test, save.
  await tab('remotes');
  await page.click('[data-add]'); await wait();
  await shot('a-remote-pick');
  await page.fill('#a-rq', 'drop'); await wait();
  await page.fill('#a-rq', ''); await page.keyboard.type('s'); await wait();
  await page.click('[data-type="sftp"]'); await wait();
  await page.fill('[data-rf="host"]', 'backup.lan');
  await page.fill('[data-rf="user"]', 'jdp');
  await page.fill('[data-rf="pass"]', 'geheim');
  await page.fill('#a-rname', 'backup-server');
  await page.click('#a-rtest'); await wait(900);
  await shot('a-remote-form');
  await page.click('.win-foot .btn.primary'); await wait(300);
  await shot('a-remote-added');
  await page.click('[data-add]'); await wait();
  await page.click('[data-type="gdrive"]'); await wait();
  await shot('a-remote-oauth');
  await closeWin();

  // Users and groups.
  await tab('users');
  await page.click('[data-uadd]'); await wait();
  await page.fill('#a-uname', 'max');
  await page.fill('#a-ufull', 'Max Mustermann');
  await page.fill('#a-upw', 'kurz');
  await page.click('.win-foot .btn.primary'); await wait(200);
  await page.fill('#a-upw', 'ein-langes-passwort');
  await page.fill('#a-upw2', 'ein-langes-passwort');
  await page.click('[data-ug="familie"]');
  await shot('a-user-new');
  await page.click('.win-foot .btn.primary'); await wait(300);
  await page.click('[data-gadd]'); await wait();
  await shot('a-group-new');
  await closeWin();
  await page.hover('.a-row[data-uopen="1"]'); await wait(150);
  await shot('a-users-hover');

  // Rules: new rule and the probe.
  await tab('rules');
  await page.click('[data-add]'); await wait();
  await page.click('#a-rkind [data-v="group"]'); await wait();
  await page.fill('#a-rpath', '/mnt/user/fotos/2025');
  await shot('a-rule-new');
  await page.click('.win-foot .btn.primary'); await wait(300);
  await page.fill('#a-ppath', '/mnt/user/fotos/2025'); await wait();
  await shot('a-rules-after');

  // Shares.
  await tab('shares');
  await page.click('.a-row[data-open="0"]'); await wait();
  await shot('a-share-win');
  await closeWin();
  await page.hover('.a-row[data-open="1"]'); await page.click('[data-revoke="1"]'); await wait();
  await shot('a-share-revoked');

  // Security: password, 2FA set-up, token.
  await tab('security');
  await page.click('[data-pw]'); await wait();
  await page.fill('#a-pw0', 'altes-passwort');
  await page.fill('#a-pw1', 'ein-neues-langes-passwort');
  await page.fill('#a-pw2', 'ein-neues-langes-passwort');
  await page.click('.win-foot .btn.primary'); await wait(300);
  console.log('sessions after pw:', await page.evaluate(() => DATA.sessions.map(s => s.device).join(' | ')));
  await page.click('[data-2off]'); await wait();
  await page.fill('#a-code', '123456');
  await page.click('.win-foot .btn.primary'); await wait(300);
  await page.click('[data-2on]'); await wait();
  await shot('a-totp-1');
  await page.fill('#a-totp', '482913');
  await page.click('.win-foot .btn.primary'); await wait(300);
  await page.mouse.click(10, 10); await page.keyboard.press('Escape'); await wait(200);
  await shot('a-totp-2');
  await page.click('.win-foot .btn.primary'); await wait(300);
  await page.click('[data-tadd]'); await wait();
  await page.fill('#a-tkname', 'home-assistant');
  await page.fill('#a-tkpw', 'ein-neues-langes-passwort');
  await page.click('.win-foot .btn.primary'); await wait(300);
  await shot('a-token-once');
  await page.click('.win-foot .btn.primary'); await wait(300);
  await full('a-security-after');

  // Integrations test, app export, info.
  await tab('integrations');
  await page.click('[data-it="webdav"]'); await wait(300);
  await page.fill('#a-oourl', 'https://office.tower.lan');
  await page.click('#a-ootest'); await wait(1000);
  await full('a-integrations-after');
  await tab('app');
  await page.click('#a-export'); await wait();
  await page.click('.win-foot .btn.primary'); await wait(300);
  await shot('a-export-done');
  await closeWin();
  await page.click('#a-upd'); await wait(1000);
  await shot('a-app-after');
  await tab('info');
  await page.hover('#a-coffee'); await wait(300);
  await shot('a-info-hover');

  // Actions from the menu bar.
  await page.evaluate(() => CC.run('whatsNew')); await wait();
  await shot('a-whatsnew');
  await closeWin();
  await page.evaluate(() => CC.run('docs')); await wait();
  await shot('a-docs');
  await closeWin();
  await page.evaluate(() => CC.run('openActivity')); await wait();
  await shot('a-activity');
  await page.click('[data-af="who"]'); await wait();
  await page.click('#menu button:has-text("Anna")'); await wait();
  await shot('a-activity-filtered');
  await page.evaluate(() => CC.setPref('theme', 'light')); await wait();
  await shot('a-activity-light');
  await page.evaluate(() => CC.setPref('theme', 'dark'));
  for (const id of ['about', 'mounts', 'openShares']) { await page.evaluate((x) => CC.run(x), id); await wait(150); console.log(id, '->', await page.evaluate(() => CC.view + '/' + CC.settingsTab_)); }

  // Phone width.
  await page.setViewportSize({ width: 390, height: 844 });
  await tab('users'); await shot('a-phone-users');
  await tab('look'); await shot('a-phone-look');
  await page.setViewportSize({ width: 1600, height: 950 });
};
