// Round 7 windows: the clash rule in the copy window, permission checkboxes, the compact search, dropdowns instead of selects.
module.exports = async (page, shot) => {
  const wait = (ms = 400) => page.waitForTimeout(ms);
  await wait(800);

  const setup = () => page.evaluate(() => {
    while (winStack.length) CC.closeWin();
    CC.go('files');
    Files.go('/mnt/user/downloads', 0); Files.go('/mnt/user/backups', 1); Files.active = 0;
    const back = FS.nodeAt('/mnt/user/backups');
    back.children = back.children.filter(c => !c.name.startsWith('setup-notes'));
    back.children.push({ name: 'setup-notes.md', size: 99, time: NOW - 400 * DAY, owner: 'nobody', group: 'users', mode: '0664' });
    const t = Files.T(0);
    t.marked = new Set();
    t.cursor = Files.entries(0).findIndex(k => k.name === 'setup-notes.md');
    CC.render();
  });
  const copyWith = async (label) => {
    await setup();
    await page.evaluate(() => CC.run('copy')); await wait(300);
    if (label) await page.evaluate((l) => { $('#clash').dataset.val = CLASH_RULES.find(r => r[1] === l)[0]; }, label);
    await page.click('.scrim .win-foot .btn.primary'); await wait(300);
    const asked = await page.locator('.scrim .notch', { hasText: 'Konflikte' }).count();
    if (asked) return { asked };
    await wait(6500);
    const names = await page.evaluate(() => FS.kids(FS.nodeAt('/mnt/user/backups')).filter(c => c.name.startsWith('setup-notes')).map(c => `${c.name}:${c.size}`));
    return { asked, names };
  };

  await setup();
  await page.evaluate(() => CC.run('copy')); await wait(300);
  console.log('clash rule default:', await page.evaluate(() => CC.admin.ddVal(document.querySelector('#clash'))));
  await shot('r7-copy-window');
  await page.keyboard.press('Escape');

  console.log('Fragen:', JSON.stringify(await copyWith(null)));
  await page.evaluate(() => { while (winStack.length) CC.closeWin(); });
  console.log('Überspringen:', JSON.stringify(await copyWith('Überspringen')));
  console.log('Überschreiben:', JSON.stringify(await copyWith('Überschreiben')));
  console.log('Wenn neuer:', JSON.stringify(await copyWith('Wenn neuer')));
  console.log('Umbenennen:', JSON.stringify(await copyWith('Umbenennen')));

  // Permissions as a checkbox grid that keeps the number and the symbolic form in step.
  await setup();
  await page.evaluate(() => CC.run('props')); await wait(300);
  const perms = () => page.evaluate(() => ({
    oct: document.querySelector('#r-oct').value, sym: document.querySelector('#r-sym').textContent,
    boxes: document.querySelectorAll('.bk-perms .cbox').length, switches: document.querySelectorAll('.bk-perms .sw').length,
  }));
  console.log('perms before:', JSON.stringify(await perms()));
  await page.click('[data-bk-perm="2:2"]'); await wait(150);
  await page.click('[data-bk-perm="0:1"]'); await wait(150);
  console.log('perms after two clicks:', JSON.stringify(await perms()));
  await page.fill('#r-oct', '750'); await wait(150);
  console.log('perms after typing 750:', JSON.stringify(await perms()), 'checked:', await page.locator('.bk-perms [aria-checked="true"]').count());
  await page.click('#r-owner'); await wait(250);
  await shot('r7-props-owner');
  await page.click('#menu button:has-text("anna")'); await wait(200);
  await page.click('#r-group'); await wait(250);
  await page.click('#menu button:has-text("familie")'); await wait(200);
  console.log('owner and group:', await page.evaluate(() => [CC.admin.ddVal($('#r-owner')), $('#r-uid').textContent, CC.admin.ddVal($('#r-group')), $('#r-gid').textContent].join(' ')));
  await shot('r7-props');
  await page.click('.scrim .win-foot .btn.primary'); await wait(300);
  console.log('saved node:', await page.evaluate(() => { const n = FS.kids(FS.nodeAt('/mnt/user/downloads')).find(k => k.name === 'setup-notes.md'); return `${n.owner}:${n.group} ${n.mode}`; }));

  // Search stays as short as its form until something is asked for, then grows downwards.
  await page.evaluate(() => { while (winStack.length) CC.closeWin(); });
  await page.evaluate(() => CC.run('search')); await wait(400);
  const box = () => page.evaluate(() => { const r = document.querySelector('.bk-search').getBoundingClientRect(); return { top: Math.round(r.top), height: Math.round(r.height), hits: !document.querySelector('.bk-hits').hidden }; });
  console.log('search empty:', JSON.stringify(await box()));
  await shot('r7-search-empty');
  await page.fill('#s-name', '*.iso'); await wait(220);
  console.log('search mid-grow:', JSON.stringify(await box()));
  await wait(500);
  console.log('search with hits:', JSON.stringify(await box()), await page.locator('#s-count').textContent());
  await shot('r7-search-hits');
  await page.click('#s-when'); await wait(250);
  await shot('r7-search-when');
  await page.click('#menu button:has-text("Letzte 30 Tage")'); await wait(300);
  console.log('Geändert:', await page.evaluate(() => CC.admin.ddVal($('#s-when'))), await page.locator('#s-count').textContent());
  await page.click('#s-when'); await wait(200);
  await page.mouse.click(5, 5); await wait(200);
  console.log('search still open after a click beside its dropdown:', await page.locator('.bk-search').count(), 'menu:', await page.locator('#menu').count());
  await page.fill('#s-name', ''); await page.click('#s-when'); await wait(200);
  await page.click('#menu button:has-text("Beliebig")'); await wait(150);
  await page.click('#s-type button[data-v="all"]'); await wait(500);
  console.log('search cleared:', JSON.stringify(await box()));

  await page.evaluate(() => { while (winStack.length) CC.closeWin(); });
  console.log('native selects left:', await page.evaluate(async () => {
    let n = 0;
    for (const id of ['search', 'multiRename', 'props', 'copy']) { CC.run(id); await new Promise(r => setTimeout(r, 150)); n += document.querySelectorAll('select').length; while (winStack.length) CC.closeWin(); }
    return n;
  }));
  await page.evaluate(() => { const t = Files.T(0); t.marked = new Set(Files.entries(0).filter(k => !k.dir).slice(0, 3).map(k => k.name)); CC.render(); CC.run('multiRename'); });
  await wait(300);
  await page.click('#m-case'); await wait(200);
  await page.click('#menu button:has-text("alles klein")'); await wait(250);
  console.log('Schreibung:', await page.evaluate(() => CC.admin.ddVal($('#m-case'))), (await page.locator('#m-count').textContent()));
  await shot('r7-rename');
  await page.evaluate(() => { while (winStack.length) CC.closeWin(); });
};
