// Round 13, Unraid: disk share against user share, hard links, cache or array, permission repair, sleeping disks, gathering a folder.
module.exports = async (page, shot) => {
  const wait = (ms = 350) => page.waitForTimeout(ms);
  const closeAll = () => page.evaluate(() => { while (winStack.length) CC.closeWin(); $$('.toast').forEach(t => t.remove()); });
  const pick = (path, i, name) => page.evaluate(([path, i, name]) => {
    Files.go(path, i);
    Files.active = i;
    const t = Files.T();
    t.cursor = name ? Math.max(0, Files.entries().findIndex(k => k.name === name)) : 0;
    CC.render();
  }, [path, i, name]);
  const run = (id) => page.evaluate((id) => CC.run(id), id);
  const primary = () => page.click('.scrim:last-of-type .win-foot .btn.primary');
  const title = () => page.evaluate(() => $$('.scrim .win .notch').at(-1)?.textContent || 'none');
  const toasts = () => page.evaluate(() => $$('.toast').map(t => t.textContent.trim()).join(' | '));
  const jobsDone = () => page.waitForFunction(() => !Jobs.running().length, null, { timeout: 30000 });
  const devOf = (p) => page.evaluate((p) => FS.nodeAt(p)?.dev || 'none', p);
  await wait(800);

  // [9672] Sleeping disks: moons in the tree, listing stays quiet, opening a file wakes its disk.
  await page.evaluate(() => { Files.openTree.add('/mnt'); CC.render(); });
  console.log('9672 moons at start:', await page.evaluate(() => $$('#tree .u-moon').map(m => m.closest('.nav-row').dataset.go).join(', ')));
  await pick('/mnt/disk1/filme', 0);
  console.log('9672 moons after listing disk1:', await page.evaluate(() => $$('#tree .u-moon').length));
  await shot('r13-9672-tree');
  await pick('/mnt/user/filme/Big Buck Bunny (2008)', 0, 'poster.jpg');
  await run('view'); await wait(500);
  console.log('9672 toast on open:', await toasts());
  console.log('9672 moons after open:', await page.evaluate(() => $$('#tree .u-moon').map(m => m.closest('.nav-row').dataset.go).join(', ')));
  await shot('r13-9672-wake');
  await closeAll();
  await run('spinDown'); await wait();
  console.log('9672 moons after spin down:', await page.evaluate(() => $$('#tree .u-moon').length));
  await closeAll();

  // [9668] Moving a folder from a disk share into the user share over the same files.
  await pick('/mnt/user/filme', 1);
  await pick('/mnt/disk2/filme', 0, 'Sintel (2010)');
  await run('move'); await wait();
  await primary(); await wait(500);
  console.log('9668 window:', await title(), '| files listed:', await page.evaluate(() => $$('.scrim .u-tbl tr').length));
  await shot('r13-9668-warning');
  await page.click('#u-way button[data-v="share"]'); await wait();
  console.log('9668 share way:', await page.evaluate(() => $('#u-plan').textContent), '| primary disabled:', await page.evaluate(() => $$('.scrim .win-foot .btn.primary').at(-1).disabled));
  await page.click('#u-way button[data-v="disk"]'); await wait();
  await page.click('#u-disk button[data-v="disk3"]'); await wait();
  console.log('9668 disk way:', await page.evaluate(() => $('#u-plan').textContent));
  await shot('r13-9668-safe');
  await primary(); await wait(300);
  console.log('9668 toast:', await toasts());
  await jobsDone(); await wait(400);
  console.log('9668 after move: on disk3', !!(await page.evaluate(() => FS.nodeAt('/mnt/disk3/filme/Sintel (2010)/Sintel (2010).mkv'))),
    '| left on disk2', !!(await page.evaluate(() => FS.nodeAt('/mnt/disk2/filme/Sintel (2010)'))),
    '| user share dev', await devOf('/mnt/user/filme/Sintel (2010)/Sintel (2010).mkv'),
    '| size', await page.evaluate(() => fmtSize(FS.nodeAt('/mnt/user/filme/Sintel (2010)/Sintel (2010).mkv').size)));
  await closeAll();

  // [9669] Hard links: the paths in Eigenschaften, the extra way in the copy window, the warning before a copy.
  await pick('/mnt/user/downloads', 0, 'Big.Buck.Bunny.2008.1080p.mkv');
  await run('props'); await wait();
  console.log('9669 props links:', await page.evaluate(() => $$('.u-where .u-links .bk-path').map(p => p.textContent).join(', ')), '| on:', await page.evaluate(() => $('.u-where .u-pills').textContent));
  await shot('r13-9669-props');
  await closeAll();
  await pick('/mnt/user/isos', 1);
  await pick('/mnt/user/downloads', 0, 'Big.Buck.Bunny.2008.1080p.mkv');
  await run('copy'); await wait();
  console.log('9669 hard link offered towards isos (other disks):', await page.locator('#how').count());
  await closeAll();
  await pick('/mnt/user/backups', 1);
  await pick('/mnt/user/downloads', 0, 'Big.Buck.Bunny.2008.1080p.mkv');
  await run('copy'); await wait();
  console.log('9669 hard link offered towards backups (disk1 allowed):', await page.locator('#how').count());
  await shot('r13-9669-copywin');
  await primary(); await wait(500);
  console.log('9669 warning:', await title(), '|', await page.evaluate(() => $('.scrim .u-alert b')?.textContent));
  await shot('r13-9669-warning');
  await page.click('.scrim:last-of-type .win-foot .btn:has-text("Als harten Link anlegen")'); await wait(400);
  console.log('9669 link made:', await page.evaluate(() => { const a = FS.nodeAt('/mnt/user/backups/Big.Buck.Bunny.2008.1080p.mkv'), b = FS.nodeAt('/mnt/user/downloads/Big.Buck.Bunny.2008.1080p.mkv'); return !!a && a.ino === b.ino && a.dev === b.dev; }), '|', await toasts());
  await closeAll();
  await pick('/mnt/user/downloads', 0, 'Big.Buck.Bunny.2008.1080p.mkv');
  await run('props'); await wait();
  console.log('9669 props links now:', await page.evaluate(() => $$('.u-where .u-links .bk-path').length));
  await closeAll();
  await pick('/mnt/user/backups', 1);
  await pick('/mnt/user/downloads', 0, 'album.flac');
  await run('move'); await wait();
  await primary(); await wait(500);
  console.log('9669 move across a pool:', await title(), '|', await page.evaluate(() => $('.scrim .u-alert b')?.textContent), '| link button:', await page.locator('.scrim .win-foot .btn:has-text("Als harten Link anlegen")').count());
  await shot('r13-9669-move');
  await closeAll();

  // [9670] Cache or array: the share badge in the foot, cache badges on files, the cache-only view.
  await pick('/mnt/user/downloads', 0);
  console.log('9670 store badge:', await page.evaluate(() => $('.panel.active .u-store').textContent), '| tip:', await page.evaluate(() => $('.panel.active .u-store').dataset.tip));
  console.log('9670 cache badges:', await page.evaluate(() => $$('.panel.active .u-cache').map(b => b.closest('.name').querySelector('.t').textContent).join(', ')));
  await shot('r13-9670-badges');
  await page.click('[data-menu="2"]'); await wait();
  console.log('9670 menu has filter:', await page.locator('#menu button:has-text("Nur auf dem Cache")').count());
  await page.click('#menu button:has-text("Nur auf dem Cache")'); await wait();
  console.log('9670 cache only:', await page.evaluate(() => Files.entries().map(k => k.name).join(', ')));
  await shot('r13-9670-filter');
  await page.click('.panel.active .u-filter'); await wait();
  console.log('9670 filter off, entries:', await page.evaluate(() => Files.entries().length));
  await pick('/mnt/user/filme', 0);
  console.log('9670 filme badge:', await page.evaluate(() => $('.panel.active .u-store').textContent));
  await closeAll();

  // [9671] Permission repair: refused inside appdata, a list of what stays when run on all shares.
  await pick('/mnt/user', 0, 'appdata');
  await page.click('.panel.active .row.cursor', { button: 'right' }); await wait();
  console.log('9671 in context menu:', await page.locator('#menu button:has-text("Rechte wie Unraid setzen")').count());
  await page.click('#menu button:has-text("Rechte wie Unraid setzen")'); await wait();
  console.log('9671 on appdata:', await page.evaluate(() => $('.scrim .u-alert b').textContent));
  await shot('r13-9671-appdata');
  await closeAll();
  await pick('/mnt', 0, 'user');
  await run('fixPerms'); await wait();
  console.log('9671 skipped:', await page.evaluate(() => $$('.scrim .u-tbl tr').map(r => r.textContent.trim()).join(' | ')));
  await shot('r13-9671-window');
  await primary(); await jobsDone(); await wait(400);
  console.log('9671 after:', await page.evaluate(() => ['/mnt/user/downloads/setup-notes.md', '/mnt/user/downloads/unfertig', '/mnt/user/appdata/bombvault/bombvault.db', '/mnt/user/system/docker.img'].map(p => { const n = FS.nodeAt(p); return `${p.split('/').pop()} ${n.owner}:${n.group} ${n.mode}`; }).join(', ')), '|', await toasts());
  await closeAll();

  // [9673] Gather a folder onto one disk, then the plan for emptying a disk.
  await pick('/mnt/user', 0, 'backups');
  await run('gather'); await wait();
  console.log('9673 spread:', await page.evaluate(() => { const s = {}; CC.unraid.filesOf(FS.nodeAt('/mnt/user/backups')).forEach(([k]) => { s[k.dev] = (s[k.dev] || 0) + 1; }); return JSON.stringify(s); }));
  console.log('9673 plan:', await page.evaluate(() => $('#g-plan').textContent), '| button:', await page.evaluate(() => $$('.scrim .win-foot .btn.primary').at(-1).textContent));
  await shot('r13-9673-gather');
  await primary(); await wait(300);
  console.log('9673 toast:', await toasts());
  await jobsDone(); await wait(400);
  console.log('9673 after:', await page.evaluate(() => { const s = {}; CC.unraid.filesOf(FS.nodeAt('/mnt/user/backups')).forEach(([k]) => { s[k.dev] = (s[k.dev] || 0) + 1; }); return JSON.stringify(s); }),
    '| disk dirs:', await page.evaluate(() => CC.unraid.DISKS.filter(d => FS.nodeAt(`/mnt/${d}/backups`)).join(', ')));
  await closeAll();
  await pick('/mnt/user', 0, 'filme');
  await run('gather'); await wait();
  await page.click('#g-mode button[data-v="scatter"]'); await wait();
  console.log('9673 scatter plan:', await page.evaluate(() => $('#g-plan').textContent));
  await shot('r13-9673-scatter');
  await closeAll();
};
